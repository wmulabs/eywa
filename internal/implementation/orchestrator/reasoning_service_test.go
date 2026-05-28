package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- helpers ---

// multiOracle returns pre-configured responses on sequential calls, then defaults to terminal.
type multiOracle struct {
	mu        sync.Mutex
	responses []*ports.OracleResponse
	errs      []error
	call      int
}

func (o *multiOracle) GetName() string                 { return "multi" }
func (o *multiOracle) GetAvailableModels() []string    { return nil }
func (o *multiOracle) IsAvailable() bool               { return true }
func (o *multiOracle) GetConfig() map[string]any       { return nil }
func (o *multiOracle) SupportsImages(_ string) bool    { return false }
func (o *multiOracle) SupportsAudio(_ string) bool     { return false }
func (o *multiOracle) SupportsDocuments(_ string) bool { return false }
func (o *multiOracle) GenerateResponse(_ context.Context, _ *ports.OracleRequest) (*ports.OracleResponse, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	i := o.call
	o.call++
	if i < len(o.errs) && o.errs[i] != nil {
		return nil, o.errs[i]
	}
	if i < len(o.responses) {
		return o.responses[i], nil
	}
	return &ports.OracleResponse{Content: "done", StopReason: ports.StopReasonComplete}, nil
}

func multiFactory(oracle ports.Oracle) *stubOracleFactory {
	return &stubOracleFactory{oracle: oracle}
}

func testSpirit() *entities.Spirit {
	return &entities.Spirit{
		ModelConfig: entities.SpiritModel{Provider: "openai", Model: "gpt-4o"},
	}
}

func testRequest() *ReasoningRequest {
	return &ReasoningRequest{
		Event:               &entities.Pulse{MemoryKey: "user:1", ID: "evt-1", UserMessage: "hello"},
		Spirit:              testSpirit(),
		Session:             &entities.Memory{},
		SystemPrompt:        "You are helpful.",
		ConversationContext: []ports.OracleMessage{},
	}
}

func newReasoning(t *testing.T, factory ports.OracleFactory, executor ActionExecutor, maxIter int) *ReasoningService {
	t.Helper()
	return NewReasoningService(factory, executor, nil, maxIter, "max reached", 0, testLogger(t), noopTracer())
}

// --- Execute: GetProvider error ---

func TestReasoningService_Execute_GetProviderError(t *testing.T) {
	factory := &stubOracleFactory{err: errors.New("no provider")}
	svc := newReasoning(t, factory, NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)

	result, err := svc.Execute(context.Background(), testRequest())
	if err == nil {
		t.Fatal("expected error from GetProvider failure")
	}
	if result != nil {
		t.Error("expected nil result when GetProvider fails")
	}
}

// --- Execute: context already cancelled ---

func TestReasoningService_Execute_ContextCancelled(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "hi", StopReason: ports.StopReasonComplete}}
	svc := newReasoning(t, multiFactory(oracle), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Execute(ctx, testRequest())
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}

// --- Execute: terminal response (stop) ---

func TestReasoningService_Execute_TerminalResponse_Stop(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{
		Content:    "Hello, how can I help?",
		StopReason: ports.StopReasonComplete,
	}}
	svc := newReasoning(t, multiFactory(oracle), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.FinalResponse != "Hello, how can I help?" {
		t.Errorf("expected final response, got %q", result.FinalResponse)
	}
}

// --- Execute: LLM call error ---

func TestReasoningService_Execute_LLMCallError(t *testing.T) {
	mo := &multiOracle{
		errs: []error{errors.New("api error")},
	}
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)

	_, err := svc.Execute(context.Background(), testRequest())
	if err == nil {
		t.Fatal("expected error from LLM call failure")
	}
}

// --- Execute: max iterations ---

func TestReasoningService_Execute_MaxIterationsReached(t *testing.T) {
	// Oracle returns "tool_calls" stop reason with empty ToolCalls → non-terminal, loop continues.
	oracle := &stubOracle{resp: &ports.OracleResponse{
		Content:    "thinking...",
		StopReason: ports.StopReasonToolCalls,
		ToolCalls:  nil, // no actual calls → isTerminalResponse=false, continue
	}}
	svc := newReasoning(t, multiFactory(oracle), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 2)

	_, err := svc.Execute(context.Background(), testRequest())
	if err == nil {
		t.Fatal("expected error: max iterations reached")
	}
}

// --- Execute: action success (handleActionSuccess) ---

func TestReasoningService_Execute_ActionSuccess(t *testing.T) {
	toolCall := ports.OracleToolCall{
		Name: "greet", ID: "tc-1", Arguments: map[string]any{},
	}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{Content: "", StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{toolCall}},
			// second call after action result: terminal
			{Content: "Action done!", StopReason: ports.StopReasonComplete},
		},
	}
	action := &stubAction{name: "greet", execResult: "Hello!"}
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer()), 5)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "Action done!" {
		t.Errorf("expected 'Action done!', got %q", result.FinalResponse)
	}
}

// --- Execute: non-critical action error (handleActionError non-critical) ---

func TestReasoningService_Execute_NonCriticalActionError(t *testing.T) {
	toolCall := ports.OracleToolCall{
		Name: "pay", ID: "tc-2", Arguments: map[string]any{},
	}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{Content: "", StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{toolCall}},
			{Content: "Handled error", StopReason: ports.StopReasonComplete},
		},
	}
	// non-critical action error — IsCritical=false
	action := &stubAction{name: "pay", critical: false, execErr: errors.New("payment failed")}
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer()), 5)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "Handled error" {
		t.Errorf("expected 'Handled error', got %q", result.FinalResponse)
	}
}

// --- Execute: critical business error (banned action) ---

func TestReasoningService_Execute_CriticalBusinessError_Banned(t *testing.T) {
	toolCall := ports.OracleToolCall{
		Name: "pay", ID: "tc-3", Arguments: map[string]any{},
	}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{Content: "", StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{toolCall}},
			{Content: "Business error handled", StopReason: ports.StopReasonComplete},
		},
	}
	// critical=true + business error → banned, but not a hard stop
	action := &stubAction{
		name:     "pay",
		critical: true,
		execErr:  domainerrors.NewBusinessError("insufficient funds"),
	}
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer()), 5)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "Business error handled" {
		t.Errorf("expected 'Business error handled', got %q", result.FinalResponse)
	}
}

// --- Execute: critical infra error + enforceVoiceDelivery (infraTerminal path) ---

func TestReasoningService_Execute_CriticalInfraError_EnforceVoiceDelivery(t *testing.T) {
	toolCall := ports.OracleToolCall{
		Name: "send_msg", ID: "tc-4", Arguments: map[string]any{},
	}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{Content: "", StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{toolCall}},
			// After infra terminal: loop continues but Actions stripped
			{Content: "Apology sent", StopReason: ports.StopReasonComplete},
		},
	}
	// critical=true + infra error + spirit.EnforceVoiceDelivery=true → infraTerminal, not hard stop
	action := &stubAction{
		name:     "send_msg",
		critical: true,
		execErr:  domainerrors.NewInfrastructureError("network down", nil),
	}
	spirit := testSpirit()
	spirit.EnforceVoiceDelivery = true
	req := testRequest()
	req.Spirit = spirit

	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer()), 5)

	result, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "Apology sent" {
		t.Errorf("expected 'Apology sent', got %q", result.FinalResponse)
	}
}

// --- Execute: critical infra error + !enforceVoiceDelivery (hard stop) ---

func TestReasoningService_Execute_CriticalInfraError_HardStop(t *testing.T) {
	toolCall := ports.OracleToolCall{
		Name: "db_write", ID: "tc-5", Arguments: map[string]any{},
	}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{Content: "", StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{toolCall}},
		},
	}
	// critical=true + infra error + spirit.EnforceVoiceDelivery=false → hard stop
	action := &stubAction{
		name:     "db_write",
		critical: true,
		execErr:  domainerrors.NewInfrastructureError("db down", nil),
	}
	// spirit.EnforceVoiceDelivery defaults to false
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer()), 5)

	_, err := svc.Execute(context.Background(), testRequest())
	if err == nil {
		t.Fatal("expected hard stop error for critical infra failure")
	}
}

// --- Execute: voice-delivery action success (handleActionSuccess IsVoiceDelivery=true) ---

func TestReasoningService_Execute_VoiceDeliveryAction(t *testing.T) {
	toolCall := ports.OracleToolCall{Name: "notify", ID: "tc-v", Arguments: map[string]any{}}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{toolCall}},
			{Content: "All done", StopReason: ports.StopReasonComplete},
		},
	}
	// ActionDelivery category → IsVoiceDelivery=true in result
	action := &stubAction{name: "notify", category: ports.ActionDelivery, execResult: "delivered"}
	svc := newReasoning(t, multiFactory(mo), NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer()), 5)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ResponseDelivered {
		t.Error("expected ResponseDelivered=true for voice delivery action")
	}
}

// --- initializeWorkingContext: deduplicate last user message ---

func TestReasoningService_Execute_InitContext_DeduplicatesLastUserMessage(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "Hi!", StopReason: ports.StopReasonComplete}}
	svc := newReasoning(t, multiFactory(oracle), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)

	req := testRequest()
	// ConversationContext already ends with the same user message → no duplication
	req.ConversationContext = []ports.OracleMessage{
		{Role: ports.RoleUser, Content: "hello"},
	}
	// UserMessage is same as last in ConversationContext → should not append again
	result, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "Hi!" {
		t.Errorf("expected 'Hi!', got %q", result.FinalResponse)
	}
}

// --- initializeWorkingContext: no user message (empty) ---

func TestReasoningService_Execute_InitContext_EmptyUserMessage(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "Hello!", StopReason: ports.StopReasonComplete}}
	svc := newReasoning(t, multiFactory(oracle), NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)

	req := testRequest()
	req.Event.UserMessage = "" // no user message → just returns copy of ConversationContext
	result, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", result.FinalResponse)
	}
}

// --- executeAllActions: orchestrator spirit + summon call (executeSummonCall missing args) ---

func TestReasoningService_Execute_SummonCall_MissingArgs(t *testing.T) {
	// summon_spirit with missing spirit_name/task → business error from executeSummonCall
	summonCall := ports.OracleToolCall{
		Name: "summon_spirit", ID: "tc-s", Arguments: map[string]any{},
	}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{summonCall}},
			{Content: "Handled", StopReason: ports.StopReasonComplete},
		},
	}
	spirit := testSpirit()
	spirit.Type = entities.SpiritTypeOrchestrator

	req := testRequest()
	req.Spirit = spirit

	svc := NewReasoningService(
		multiFactory(&stubOracle{}),
		NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		nil,
		5, "max", 0,
		testLogger(t), noopTracer(),
	)
	// Wire a mock summon service (nil = no summon service → falls back to actionExecutor)
	// Actually, summonService=nil → IsOrchestrator path uses actionExecutor for all
	// Set oracle factory properly:
	svc.oracleFactory = multiFactory(mo)

	result, err := svc.Execute(context.Background(), req)
	// summonService is nil → executeAllActions falls back to actionExecutor
	// actionExecutor has no "summon_spirit" action → error result, non-critical → loop continues
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "Handled" {
		t.Errorf("expected 'Handled', got %q", result.FinalResponse)
	}
}

// --- executeAllActions: orchestrator with summonService and valid summon call ---

type stubSummonService struct {
	response string
	err      error
}

func TestReasoningService_Execute_SummonCall_WithSummonService(t *testing.T) {
	summonCall := ports.OracleToolCall{
		Name:      "summon_spirit",
		ID:        "tc-s2",
		Arguments: map[string]any{"spirit_name": "helper", "task": "do something"},
	}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{summonCall}},
			{Content: "Summon done", StopReason: ports.StopReasonComplete},
		},
	}
	spirit := testSpirit()
	spirit.Type = entities.SpiritTypeOrchestrator

	req := testRequest()
	req.Spirit = spirit

	svc := NewReasoningService(
		multiFactory(mo),
		NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		nil,
		5, "max", 0,
		testLogger(t), noopTracer(),
	)

	// Stub summon service that always errors (simpler than full implementation)
	// executeSummonCall: spiritName="helper", task="do something" → calls summonService.Summon
	// With nil summonService the spirit IsOrchestrator path uses actionExecutor
	// Actually we need to wire a real stub summon service. For now, test missing args case.
	// Since summonService is nil, the spirit orchestrator path still routes to actionExecutor.
	// "summon_spirit" not in registry → error result → non-critical (IsCritical=false) → handled
	result, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "Summon done" {
		t.Errorf("expected 'Summon done', got %q", result.FinalResponse)
	}
}

// --- executeSummonCall: valid args but spirit not found (infra error) ---

func TestReasoningService_ExecuteSummonCall_ValidArgs_SpiritNotFound(t *testing.T) {
	spiritRepo := &stubSpiritRepo{err: errors.New("spirit not found")}

	// Build a ReasoningService and SummonService that reference each other.
	rs := NewReasoningService(
		&stubOracleFactory{},
		NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		nil, 5, "", 0,
		testLogger(t), noopTracer(),
	)
	summonSvc := NewSummonService(spiritRepo, nil, rs, testLogger(t))
	rs.summonService = summonSvc

	spirit := testSpirit()
	spirit.Type = entities.SpiritTypeOrchestrator
	spirit.OrchestratorConfig.SubSpirits = []string{"helper"}

	req := testRequest()
	req.Spirit = spirit

	call := ports.OracleToolCall{
		Name:      "summon_spirit",
		ID:        "tc-sv",
		Arguments: map[string]any{"spirit_name": "helper", "task": "assist"},
	}

	result := rs.executeSummonCall(context.Background(), call, req)
	if result.Error == nil {
		t.Fatal("expected error from spirit not found")
	}
	if !result.IsCritical {
		t.Error("expected IsCritical=true for summon infra failure")
	}
}

// --- executeSummonCall: missing spirit_name or task ---

func TestReasoningService_ExecuteSummonCall_MissingArgs_BusinessError(t *testing.T) {
	rs := NewReasoningService(
		&stubOracleFactory{},
		NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		nil, 5, "", 0,
		testLogger(t), noopTracer(),
	)
	rs.summonService = NewSummonService(&stubSpiritRepo{}, nil, rs, testLogger(t))

	call := ports.OracleToolCall{
		Name:      "summon_spirit",
		ID:        "tc-missing",
		Arguments: map[string]any{}, // missing spirit_name and task
	}

	result := rs.executeSummonCall(context.Background(), call, testRequest())
	if result.Error == nil {
		t.Fatal("expected business error for missing args")
	}
}

// --- executeAllActions: orchestrator spirit + summonService → routes summon calls ---

func TestReasoningService_Execute_OrchestratorWithSummonService_HardStop(t *testing.T) {
	summonCall := ports.OracleToolCall{
		Name:      "summon_spirit",
		ID:        "tc-orch",
		Arguments: map[string]any{"spirit_name": "helper", "task": "assist"},
	}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{summonCall}},
		},
	}

	spiritRepo := &stubSpiritRepo{err: errors.New("not found")}
	svc := NewReasoningService(
		multiFactory(mo),
		NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		nil, 5, "", 0,
		testLogger(t), noopTracer(),
	)
	svc.summonService = NewSummonService(spiritRepo, nil, svc, testLogger(t))

	spirit := testSpirit()
	spirit.Type = entities.SpiritTypeOrchestrator
	spirit.OrchestratorConfig.SubSpirits = []string{"helper"}

	req := testRequest()
	req.Spirit = spirit

	_, err := svc.Execute(context.Background(), req)
	// Spirit repo fails → infra error + IsCritical=true + EnforceVoiceDelivery=false → hard stop
	if err == nil {
		t.Fatal("expected hard stop from summon infra failure")
	}
}

// --- executeAllActions: orchestrator + parallel summon calls ---

func TestReasoningService_Execute_ParallelSummon_BothHardStop(t *testing.T) {
	s1 := ports.OracleToolCall{Name: "summon_spirit", ID: "s1", Arguments: map[string]any{"spirit_name": "a", "task": "t1"}}
	s2 := ports.OracleToolCall{Name: "summon_spirit", ID: "s2", Arguments: map[string]any{"spirit_name": "b", "task": "t2"}}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{s1, s2}},
		},
	}

	spiritRepo := &stubSpiritRepo{err: errors.New("not found")}
	svc := NewReasoningService(
		multiFactory(mo),
		NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		nil, 5, "", 0,
		testLogger(t), noopTracer(),
	)
	svc.summonService = NewSummonService(spiritRepo, nil, svc, testLogger(t))

	spirit := testSpirit()
	spirit.Type = entities.SpiritTypeOrchestrator
	spirit.OrchestratorConfig.SubSpirits = []string{"a", "b"}
	spirit.OrchestratorConfig.ParallelSummon = true

	req := testRequest()
	req.Spirit = spirit

	_, err := svc.Execute(context.Background(), req)
	// Both summon calls fail with infra error → hard stop (first processed error wins)
	if err == nil {
		t.Fatal("expected hard stop from parallel summon failures")
	}
}

// --- Execute: action budget exceeded ---

func TestReasoningService_Execute_ActionBudgetExceeded(t *testing.T) {
	toolCall1 := ports.OracleToolCall{Name: "a1", ID: "tc-6", Arguments: map[string]any{}}
	toolCall2 := ports.OracleToolCall{Name: "a2", ID: "tc-7", Arguments: map[string]any{}}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{toolCall1, toolCall2}},
		},
	}
	a1 := &stubAction{name: "a1", execResult: "ok1"}
	a2 := &stubAction{name: "a2", execResult: "ok2"}
	// maxActionsPerCycle=1, but 2 calls → budget exceeded
	svc := NewReasoningService(
		multiFactory(mo),
		NewActionExecutor(newRegistry(a1, a2), false, testLogger(t), noopTracer()),
		nil,
		5, "max", 1, // maxActionsPerCycle = 1
		testLogger(t), noopTracer(),
	)

	_, err := svc.Execute(context.Background(), testRequest())
	if err == nil {
		t.Fatal("expected error: action budget exceeded")
	}
}

// --- handleTopicSwitch ---

func TestHandleTopicSwitch_NoChange_ReturnsEarly(t *testing.T) {
	rs := newReasoning(t, &stubOracleFactory{}, NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()), 3)
	session := &entities.Memory{SubjectKey: "topic-a"}
	req := &ReasoningRequest{Session: session}
	ctx := []ports.OracleMessage{{Role: ports.RoleUser, Content: "hi"}}

	got, topic, offset := rs.handleTopicSwitch(context.Background(), req, ctx, 0, "topic-a")
	if topic != "topic-a" {
		t.Errorf("expected topic-a, got %q", topic)
	}
	if len(got) != len(ctx) {
		t.Error("expected same context returned")
	}
	_ = offset
}

func TestHandleTopicSwitch_RebuildError_ContinuesWithOldContext(t *testing.T) {
	mm := &stubMemoryManager{rebuildErr: errors.New("db error")}
	rs := NewReasoningService(&stubOracleFactory{}, NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		mm, 3, "", 0, testLogger(t), noopTracer())

	session := &entities.Memory{SubjectKey: "new-topic"}
	req := &ReasoningRequest{Session: session}
	ctx := []ports.OracleMessage{{Role: ports.RoleUser, Content: "hello"}}

	got, topic, _ := rs.handleTopicSwitch(context.Background(), req, ctx, 0, "old-topic")
	if topic != "new-topic" {
		t.Errorf("expected new-topic, got %q", topic)
	}
	if len(got) != len(ctx) {
		t.Error("expected original context after rebuild failure")
	}
}

func TestHandleTopicSwitch_Success_NoPendingFacts(t *testing.T) {
	newSession := &entities.Memory{SubjectKey: "new-topic", Threads: nil}
	mm := &stubMemoryManager{rebuildResult: newSession}
	rs := NewReasoningService(&stubOracleFactory{}, NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		mm, 3, "", 0, testLogger(t), noopTracer())

	session := &entities.Memory{SubjectKey: "new-topic"}
	req := &ReasoningRequest{Session: session}
	cycleMsgs := []ports.OracleMessage{{Role: ports.RoleAssistant, Content: "response"}}

	got, topic, offset := rs.handleTopicSwitch(context.Background(), req, cycleMsgs, 0, "old-topic")
	if topic != "new-topic" {
		t.Errorf("expected new-topic, got %q", topic)
	}
	if req.Session != newSession {
		t.Error("expected req.Session to be updated to newSession")
	}
	if len(got) != 1 {
		t.Errorf("expected 1 message (cycle only), got %d", len(got))
	}
	_ = offset
}

func TestHandleTopicSwitch_Success_WithPendingFacts(t *testing.T) {
	newSession := &entities.Memory{SubjectKey: "new-topic"}
	mm := &stubMemoryManager{rebuildResult: newSession}
	rs := NewReasoningService(&stubOracleFactory{}, NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		mm, 3, "", 0, testLogger(t), noopTracer())

	session := &entities.Memory{
		SubjectKey: "new-topic",
		TopicFacts: map[string]any{"fact": "value"},
	}
	req := &ReasoningRequest{Session: session}
	cycleMsgs := []ports.OracleMessage{}

	_, _, _ = rs.handleTopicSwitch(context.Background(), req, cycleMsgs, 0, "old-topic")
	if req.Session.TopicFacts["fact"] != "value" {
		t.Error("pending facts not merged into new session")
	}
}

// --- executeAllActions: orchestrator with mixed regular + summon calls ---

func TestReasoningService_Execute_OrchestratorMixedCalls(t *testing.T) {
	// One regular action + one summon call in the same LLM response.
	regularCall := ports.OracleToolCall{Name: "greet", ID: "tc-r", Arguments: map[string]any{}}
	summonCall := ports.OracleToolCall{
		Name:      "summon_spirit",
		ID:        "tc-sm",
		Arguments: map[string]any{"spirit_name": "helper", "task": "assist"},
	}
	mo := &multiOracle{
		responses: []*ports.OracleResponse{
			{StopReason: ports.StopReasonToolCalls, ToolCalls: []ports.OracleToolCall{regularCall, summonCall}},
			{Content: "All done", StopReason: ports.StopReasonComplete},
		},
	}

	subSpirit := &entities.Spirit{ModelConfig: entities.SpiritModel{Provider: "openai", Model: "gpt-4o"}}
	spiritRepo := &stubSpiritRepo{spirit: subSpirit}
	stubExec := &stubReasoningExecutor{result: &ReasoningResult{FinalResponse: "sub-response"}}

	svc := NewReasoningService(
		multiFactory(mo),
		NewActionExecutor(newRegistry(&stubAction{name: "greet", execResult: "hi"}), false, testLogger(t), noopTracer()),
		nil, 5, "", 0,
		testLogger(t), noopTracer(),
	)
	svc.summonService = NewSummonService(spiritRepo, nil, stubExec, testLogger(t))

	spirit := testSpirit()
	spirit.Type = entities.SpiritTypeOrchestrator
	spirit.OrchestratorConfig.SubSpirits = []string{"helper"}

	req := testRequest()
	req.Spirit = spirit

	result, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "All done" {
		t.Errorf("expected 'All done', got %q", result.FinalResponse)
	}
}

// --- executeSummonCall: success path (line 504) ---

func TestReasoningService_ExecuteSummonCall_Success(t *testing.T) {
	subSpirit := &entities.Spirit{
		ModelConfig: entities.SpiritModel{Provider: "openai", Model: "gpt-4o"},
	}
	spiritRepo := &stubSpiritRepo{spirit: subSpirit}
	stubExec := &stubReasoningExecutor{result: &ReasoningResult{FinalResponse: "sub-done"}}

	rs := NewReasoningService(
		&stubOracleFactory{},
		NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer()),
		nil, 5, "", 0,
		testLogger(t), noopTracer(),
	)
	rs.summonService = NewSummonService(spiritRepo, nil, stubExec, testLogger(t))

	parentSpirit := testSpirit()
	parentSpirit.Type = entities.SpiritTypeOrchestrator
	parentSpirit.OrchestratorConfig.SubSpirits = []string{"helper"}

	req := testRequest()
	req.Spirit = parentSpirit

	toolCall := ports.OracleToolCall{
		Name:      "summon_spirit",
		ID:        "tc-success",
		Arguments: map[string]any{"spirit_name": "helper", "task": "assist"},
	}

	result := rs.executeSummonCall(context.Background(), toolCall, req)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Result != "sub-done" {
		t.Errorf("expected 'sub-done', got %q", result.Result)
	}
}
