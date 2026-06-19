package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// countingAction records how many times it is executed, so memoization can be asserted.
type countingAction struct {
	name   string
	result string
	calls  int32
}

func (a *countingAction) GetName() string                   { return a.name }
func (a *countingAction) GetDescription() string            { return "counts executions" }
func (a *countingAction) GetParameters() map[string]any     { return nil }
func (a *countingAction) IsCritical() bool                  { return false }
func (a *countingAction) GetCategory() ports.ActionCategory { return "" }
func (a *countingAction) Validate(_ map[string]any) error   { return nil }
func (a *countingAction) Execute(_ context.Context, _ map[string]any) (string, error) {
	atomic.AddInt32(&a.calls, 1)
	return a.result, nil
}

// memCheckpointStore is an in-memory CheckpointStore for tests, with call counters and injectable
// errors to exercise the fail-open paths.
type memCheckpointStore struct {
	mu                          sync.Mutex
	data                        map[string][]byte
	saves, loads, deletes       int
	loadErr, saveErr, deleteErr error
}

var _ ports.CheckpointStore = (*memCheckpointStore)(nil)

func newMemCheckpointStore() *memCheckpointStore {
	return &memCheckpointStore{data: make(map[string][]byte)}
}

func (s *memCheckpointStore) Save(_ context.Context, turnID string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves++
	if s.saveErr != nil {
		return s.saveErr
	}
	s.data[turnID] = data
	return nil
}

func (s *memCheckpointStore) Load(_ context.Context, turnID string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loads++
	if s.loadErr != nil {
		return nil, false, s.loadErr
	}
	data, ok := s.data[turnID]
	return data, ok, nil
}

func (s *memCheckpointStore) Delete(_ context.Context, turnID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.data, turnID)
	return nil
}

func (s *memCheckpointStore) has(turnID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[turnID]
	return ok
}

// nonTerminal advances the loop without executing actions: empty ToolCalls with the tool-calls stop
// reason is non-terminal, so the iteration appends an assistant message and continues.
func nonTerminal(content string) *ports.OracleResponse {
	return &ports.OracleResponse{Content: content, StopReason: ports.StopReasonToolCalls}
}

func terminal(content string) *ports.OracleResponse {
	return &ports.OracleResponse{Content: content, StopReason: ports.StopReasonComplete}
}

func durableRequest(idempotencyKey string) *ReasoningRequest {
	req := testRequest()
	req.Event.IdempotencyKey = idempotencyKey
	return req
}

func TestReasoningService_Checkpoint_SavedPerIterationAndDeletedOnDone(t *testing.T) {
	mo := &multiOracle{responses: []*ports.OracleResponse{
		nonTerminal("step 1"),
		nonTerminal("step 2"),
		terminal("final"),
	}}
	store := newMemCheckpointStore()
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 5)
	svc.SetCheckpointStore(store)

	result, err := svc.Execute(context.Background(), durableRequest("turn-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "final" {
		t.Errorf("FinalResponse = %q, want %q", result.FinalResponse, "final")
	}
	// One save after each of the two non-terminal iterations; the terminal iteration deletes instead.
	if store.saves != 2 {
		t.Errorf("saves = %d, want 2", store.saves)
	}
	if store.loads != 1 {
		t.Errorf("loads = %d, want 1", store.loads)
	}
	if store.deletes != 1 {
		t.Errorf("deletes = %d, want 1", store.deletes)
	}
	if store.has("turn-1") {
		t.Error("checkpoint should be deleted after a completed turn")
	}
}

func TestReasoningService_Checkpoint_ResumesAfterInterruption(t *testing.T) {
	script := func() *multiOracle {
		return &multiOracle{responses: []*ports.OracleResponse{
			nonTerminal("step 1"),
			nonTerminal("step 2"),
			terminal("final"),
		}}
	}

	// Uninterrupted baseline.
	baseStore := newMemCheckpointStore()
	baseSvc := newReasoning(t, multiFactory(script()), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 5)
	baseSvc.SetCheckpointStore(baseStore)
	want, err := baseSvc.Execute(context.Background(), durableRequest("turn-base"))
	if err != nil {
		t.Fatalf("baseline run failed: %v", err)
	}

	// Interrupted run: iteration 0 succeeds and checkpoints, iteration 1 fails with an LLM error.
	store := newMemCheckpointStore()
	crashing := &multiOracle{
		responses: []*ports.OracleResponse{nonTerminal("step 1")},
		errs:      []error{nil, errors.New("connection reset")},
	}
	crashSvc := newReasoning(t, multiFactory(crashing), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 5)
	crashSvc.SetCheckpointStore(store)
	if _, err := crashSvc.Execute(context.Background(), durableRequest("turn-2")); err == nil {
		t.Fatal("expected the injected LLM error to surface")
	}
	if !store.has("turn-2") {
		t.Fatal("checkpoint must survive an interrupted turn")
	}

	// Resume: a fresh service replays only the remaining iterations against the same checkpoint.
	resuming := &multiOracle{responses: []*ports.OracleResponse{
		nonTerminal("step 2"),
		terminal("final"),
	}}
	resumeSvc := newReasoning(t, multiFactory(resuming), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 5)
	resumeSvc.SetCheckpointStore(store)
	got, err := resumeSvc.Execute(context.Background(), durableRequest("turn-2"))
	if err != nil {
		t.Fatalf("resume run failed: %v", err)
	}

	if got.FinalResponse != want.FinalResponse {
		t.Errorf("resumed FinalResponse = %q, want %q", got.FinalResponse, want.FinalResponse)
	}
	if got.IterationsUsed != want.IterationsUsed {
		t.Errorf("resumed IterationsUsed = %d, want %d", got.IterationsUsed, want.IterationsUsed)
	}
	if len(got.WorkingContext) != len(want.WorkingContext) {
		t.Errorf("resumed WorkingContext len = %d, want %d", len(got.WorkingContext), len(want.WorkingContext))
	}
	if store.has("turn-2") {
		t.Error("checkpoint should be deleted after the resumed turn completes")
	}
}

func TestReasoningService_Checkpoint_DeletedOnMaxIterations(t *testing.T) {
	// Every response is non-terminal, so the loop exhausts its iteration budget.
	mo := &multiOracle{responses: []*ports.OracleResponse{nonTerminal("a"), nonTerminal("b")}}
	store := newMemCheckpointStore()
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 2)
	svc.SetCheckpointStore(store)

	if _, err := svc.Execute(context.Background(), durableRequest("turn-max")); err == nil {
		t.Fatal("expected max-iterations error")
	}
	if store.has("turn-max") {
		t.Error("checkpoint should be deleted after the iteration budget is exhausted")
	}
}

func TestReasoningService_Checkpoint_NoStableKey_SkipsCheckpoint(t *testing.T) {
	mo := &multiOracle{responses: []*ports.OracleResponse{terminal("ok")}}
	store := newMemCheckpointStore()
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)
	svc.SetCheckpointStore(store)

	req := &ReasoningRequest{
		Event:               &entities.Pulse{MemoryKey: "user:1", UserMessage: "hello"}, // no ID, no IdempotencyKey
		Spirit:              testSpirit(),
		Session:             &entities.Memory{},
		ConversationContext: []ports.OracleMessage{},
	}
	if _, err := svc.Execute(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.saves != 0 || store.loads != 0 || store.deletes != 0 {
		t.Errorf("expected no checkpoint calls without a stable turn key, got saves=%d loads=%d deletes=%d",
			store.saves, store.loads, store.deletes)
	}
}

func TestReasoningService_Checkpoint_FailsOpen(t *testing.T) {
	cases := []struct {
		name  string
		store *memCheckpointStore
	}{
		{"load error", &memCheckpointStore{data: map[string][]byte{}, loadErr: errors.New("redis down")}},
		{"save and delete error", &memCheckpointStore{data: map[string][]byte{}, saveErr: errors.New("redis down"), deleteErr: errors.New("redis down")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mo := &multiOracle{responses: []*ports.OracleResponse{nonTerminal("step"), terminal("final")}}
			svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 5)
			svc.SetCheckpointStore(tc.store)

			result, err := svc.Execute(context.Background(), durableRequest("turn-x"))
			if err != nil {
				t.Fatalf("durability must fail open, got error: %v", err)
			}
			if result.FinalResponse != "final" {
				t.Errorf("FinalResponse = %q, want %q", result.FinalResponse, "final")
			}
		})
	}
}

func TestReasoningService_Checkpoint_ResumeDecodeError_StartsFresh(t *testing.T) {
	store := newMemCheckpointStore()
	store.data["turn-bad"] = []byte("{not valid json")

	mo := &multiOracle{responses: []*ports.OracleResponse{terminal("fresh")}}
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)
	svc.SetCheckpointStore(store)

	result, err := svc.Execute(context.Background(), durableRequest("turn-bad"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "fresh" {
		t.Errorf("FinalResponse = %q, want %q (should start fresh on a corrupt checkpoint)", result.FinalResponse, "fresh")
	}
}

func TestReasoningService_SaveCheckpoint_EncodeError_FailsOpen(t *testing.T) {
	store := newMemCheckpointStore()
	svc := newReasoning(t, multiFactory(&multiOracle{}), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)
	svc.SetCheckpointStore(store)

	// A channel in the tool-call arguments is not JSON-serializable, so snapshot marshaling fails.
	ts := &turnState{
		workingContext: []ports.OracleMessage{{
			Role:      ports.RoleAssistant,
			ToolCalls: []ports.OracleToolCall{{Name: "x", Arguments: map[string]any{"bad": make(chan int)}}},
		}},
		bannedSignatures: map[string]bool{},
		result:           &ReasoningResult{},
	}

	svc.saveCheckpoint(context.Background(), "turn-enc", ts)
	if store.saves != 0 {
		t.Errorf("Save should not be reached when encoding fails, got saves=%d", store.saves)
	}
}

func TestReasoningService_Memoization_ReusesAcrossIterations(t *testing.T) {
	tc := ports.OracleToolCall{Name: "ping", ID: "tc-1", Arguments: map[string]any{"x": "1"}}
	mo := &multiOracle{responses: []*ports.OracleResponse{
		{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{tc}},
		{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{tc}}, // identical signature
		terminal("done"),
	}}
	action := &countingAction{name: "ping", result: "pong"}
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer()), 5)
	svc.SetCheckpointStore(newMemCheckpointStore())

	if _, err := svc.Execute(context.Background(), durableRequest("turn-memo")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&action.calls); got != 1 {
		t.Errorf("expected the identical call executed once (memoized), got %d", got)
	}
}

func TestReasoningService_Memoization_OffWithoutCheckpointStore(t *testing.T) {
	tc := ports.OracleToolCall{Name: "ping", ID: "tc-1", Arguments: map[string]any{"x": "1"}}
	mo := &multiOracle{responses: []*ports.OracleResponse{
		{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{tc}},
		{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{tc}},
		terminal("done"),
	}}
	action := &countingAction{name: "ping", result: "pong"}
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer()), 5)

	if _, err := svc.Execute(context.Background(), durableRequest("turn-nomemo")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&action.calls); got != 2 {
		t.Errorf("without durability there is no memoization; expected 2 executions, got %d", got)
	}
}

func TestReasoningService_Memoization_SurvivesResume(t *testing.T) {
	tc := ports.OracleToolCall{Name: "ping", ID: "tc-1", Arguments: map[string]any{"x": "1"}}
	store := newMemCheckpointStore()

	// Crash: the tool runs in iteration 0 (and is memoized), then iteration 1 fails.
	crashAction := &countingAction{name: "ping", result: "pong"}
	crashOracle := &multiOracle{
		responses: []*ports.OracleResponse{{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{tc}}},
		errs:      []error{nil, errors.New("connection reset")},
	}
	crashSvc := newReasoning(t, multiFactory(crashOracle), NewActionExecutor(newRegistry(crashAction), false, testLogger(t), noopTracer()), 5)
	crashSvc.SetCheckpointStore(store)
	if _, err := crashSvc.Execute(context.Background(), durableRequest("turn-r")); err == nil {
		t.Fatal("expected the injected error to surface")
	}
	if got := atomic.LoadInt32(&crashAction.calls); got != 1 {
		t.Fatalf("expected the tool executed once pre-crash, got %d", got)
	}

	// Resume: the model re-requests the same call; it must be reused from the memo, not re-executed.
	resumeAction := &countingAction{name: "ping", result: "pong"}
	resumeOracle := &multiOracle{responses: []*ports.OracleResponse{
		{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{tc}},
		terminal("done"),
	}}
	resumeSvc := newReasoning(t, multiFactory(resumeOracle), NewActionExecutor(newRegistry(resumeAction), false, testLogger(t), noopTracer()), 5)
	resumeSvc.SetCheckpointStore(store)
	got, err := resumeSvc.Execute(context.Background(), durableRequest("turn-r"))
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if calls := atomic.LoadInt32(&resumeAction.calls); calls != 0 {
		t.Errorf("expected the pre-crash tool reused from memo on resume, executed %d times", calls)
	}
	if got.FinalResponse != "done" {
		t.Errorf("FinalResponse = %q, want %q", got.FinalResponse, "done")
	}
}

func TestTurnSnapshot_RoundTrip(t *testing.T) {
	src := &turnState{
		workingContext:     []ports.OracleMessage{{Role: ports.RoleUser, Content: "hi"}},
		activeTopic:        "billing",
		conversationOffset: 1,
		bannedSignatures:   map[string]bool{"sig-a": true, "sig-b": true},
		infraTerminal:      true,
		totalActionCalls:   4,
		iterBoundaries:     []int{2, 5},
		topicSwitched:      true,
		reflectionRounds:   1,
		groundingRevised:   true,
		planNudged:         true,
		criticalErrors:     2,
		plan:               &planState{maxItems: 10, items: []entities.PlanItem{{Title: "do x", Status: entities.PlanDone}}},
		result: &ReasoningResult{
			IterationsUsed:    3,
			TokensUsed:        ports.OracleUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			TokensByModel:     map[string]ports.OracleUsage{"gpt-4o": {TotalTokens: 15}},
			ResponseDelivered: true,
		},
	}

	data, err := json.Marshal(src.snapshot())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var snap turnSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	dst := &turnState{plan: newPlanState(10), result: &ReasoningResult{}}
	dst.restore(snap)

	if dst.activeTopic != "billing" || dst.conversationOffset != 1 || !dst.infraTerminal {
		t.Errorf("scalar fields not restored: %+v", dst)
	}
	if dst.totalActionCalls != 4 || dst.criticalErrors != 2 || dst.reflectionRounds != 1 {
		t.Errorf("counter fields not restored: %+v", dst)
	}
	if !dst.topicSwitched || !dst.groundingRevised || !dst.planNudged {
		t.Errorf("flag fields not restored: %+v", dst)
	}
	if !dst.bannedSignatures["sig-a"] || !dst.bannedSignatures["sig-b"] {
		t.Errorf("banned signatures not restored: %+v", dst.bannedSignatures)
	}
	if len(dst.plan.items) != 1 || dst.plan.items[0].Title != "do x" {
		t.Errorf("plan items not restored: %+v", dst.plan.items)
	}
	if dst.result.IterationsUsed != 3 || dst.result.TokensUsed.TotalTokens != 15 || !dst.result.ResponseDelivered {
		t.Errorf("result fields not restored: %+v", dst.result)
	}
	if len(dst.result.WorkingContext) != 1 {
		t.Errorf("result working context not restored: %+v", dst.result.WorkingContext)
	}
}
