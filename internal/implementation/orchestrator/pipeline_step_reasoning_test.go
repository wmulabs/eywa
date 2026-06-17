package orchestrator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type stubReasoningExecutor struct {
	result     *ReasoningResult
	err        error
	streamInit error // synchronous error returned by ExecuteStream
	delay      time.Duration
}

var _ ReasoningExecutor = (*stubReasoningExecutor)(nil)

func (s *stubReasoningExecutor) Execute(_ context.Context, _ *ReasoningRequest) (*ReasoningResult, error) {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return s.result, s.err
}

func (s *stubReasoningExecutor) ExecuteStream(_ context.Context, _ *ReasoningRequest) (<-chan ReasoningEvent, error) {
	if s.streamInit != nil {
		return nil, s.streamInit
	}
	ch := make(chan ReasoningEvent, 4)
	go func() {
		defer close(ch)
		if s.delay > 0 {
			time.Sleep(s.delay)
		}
		if s.err != nil {
			ch <- ReasoningEvent{Type: ReasoningEventError, Err: s.err, Result: s.result}
			return
		}
		if s.result != nil && s.result.FinalResponse != "" {
			ch <- ReasoningEvent{Type: ReasoningEventDelta, Delta: s.result.FinalResponse}
		}
		ch <- ReasoningEvent{Type: ReasoningEventDone, Result: s.result}
	}()
	return ch, nil
}

func reasoningState(spirit *entities.Spirit) *ProcessingState {
	return &ProcessingState{
		Event:       &entities.Pulse{EventType: "user_message", MemoryKey: "user:1"},
		Spirit:      spirit,
		EventConfig: &entities.Link{},
		Session:     &entities.Memory{},
	}
}

func TestReasoningStep_Skipped_DoesNotCallExecutor(t *testing.T) {
	called := false
	exec := &stubReasoningExecutor{
		result: &ReasoningResult{FinalResponse: "hi"},
	}
	step := NewReasoningStep(&stubReasoningExecutorSpy{inner: exec, called: &called}, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})
	state.Skipped = true

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called {
		t.Error("expected executor not to be called when state.Skipped=true")
	}
}

func TestReasoningStep_NotifierSpirit_DoesNotCallExecutor(t *testing.T) {
	called := false
	exec := &stubReasoningExecutor{result: &ReasoningResult{}}
	step := NewReasoningStep(&stubReasoningExecutorSpy{inner: exec, called: &called}, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeNotifier})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called {
		t.Error("expected executor not to be called for notifier spirit")
	}
}

func TestReasoningStep_NilResult_NoOp(t *testing.T) {
	exec := &stubReasoningExecutor{result: nil, err: nil}
	step := NewReasoningStep(exec, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if state.Response != "" {
		t.Error("expected empty response when result is nil")
	}
}

func TestReasoningStep_ConversationalSpirit_PopulatesResponse(t *testing.T) {
	exec := &stubReasoningExecutor{
		result: &ReasoningResult{FinalResponse: "Hello, how can I help?"},
	}
	step := NewReasoningStep(exec, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if state.Response != "Hello, how can I help?" {
		t.Errorf("expected response populated, got '%s'", state.Response)
	}
}

func TestReasoningStep_ExecutorError_ReturnsOrchestrationError(t *testing.T) {
	exec := &stubReasoningExecutor{
		result: &ReasoningResult{},
		err:    errors.New("llm timeout"),
	}
	step := NewReasoningStep(exec, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Error("expected error when executor fails")
	}
	if !IsRetriable(err) {
		t.Error("expected retriable OrchestrationError from reasoning failure")
	}
}

func TestReasoningStep_ResultWithFinalSession_UpdatesStateSession(t *testing.T) {
	newSession := &entities.Memory{MemoryKey: "user:1", SubjectKey: "new-topic"}
	exec := &stubReasoningExecutor{
		result: &ReasoningResult{
			FinalResponse: "done",
			FinalSession:  newSession,
		},
	}
	step := NewReasoningStep(exec, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if state.Session != newSession {
		t.Error("expected state.Session updated from FinalSession")
	}
}

func TestReasoningStep_ResponseDelivered_SetsStateFlag(t *testing.T) {
	exec := &stubReasoningExecutor{
		result: &ReasoningResult{FinalResponse: "sent", ResponseDelivered: true},
	}
	step := NewReasoningStep(exec, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !state.ResponseDelivered {
		t.Error("expected state.ResponseDelivered=true when result.ResponseDelivered=true")
	}
}

// stubReasoningExecutorSpy wraps a stub and tracks whether Execute was called.
type stubReasoningExecutorSpy struct {
	inner  *stubReasoningExecutor
	called *bool
}

var _ ReasoningExecutor = (*stubReasoningExecutorSpy)(nil)

func (s *stubReasoningExecutorSpy) Execute(ctx context.Context, req *ReasoningRequest) (*ReasoningResult, error) {
	*s.called = true
	return s.inner.Execute(ctx, req)
}

func (s *stubReasoningExecutorSpy) ExecuteStream(ctx context.Context, req *ReasoningRequest) (<-chan ReasoningEvent, error) {
	*s.called = true
	return s.inner.ExecuteStream(ctx, req)
}

func TestBuildReasoningRequest_CopiesSpiritResponseFormat(t *testing.T) {
	rf := &entities.ResponseFormat{Name: "person", Schema: map[string]any{"type": "object"}, Strict: true}
	spirit := &entities.Spirit{Type: entities.SpiritTypeConversational, ResponseFormat: rf}
	step := NewReasoningStep(&stubReasoningExecutor{}, time.Second, nil, 0, testLogger(t))

	req := step.buildReasoningRequest(reasoningState(spirit))
	if req.ResponseFormat != rf {
		t.Errorf("expected ResponseFormat propagated from Spirit, got %+v", req.ResponseFormat)
	}
}

func TestReasoningStep_Streaming_ForwardsDeltasAndAssembles(t *testing.T) {
	exec := &stubReasoningExecutor{result: &ReasoningResult{FinalResponse: "hello world"}}
	step := NewReasoningStep(exec, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})

	var got []string
	state.streamSink = func(ev ReasoningEvent) {
		if ev.Type == ReasoningEventDelta {
			got = append(got, ev.Delta)
		}
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 || got[0] != "hello world" {
		t.Errorf("expected forwarded delta 'hello world', got %v", got)
	}
	if state.Response != "hello world" {
		t.Errorf("expected assembled Response 'hello world', got %q", state.Response)
	}
	if state.ReasoningResult == nil {
		t.Error("expected ReasoningResult assembled from the stream")
	}
}

func TestReasoningStep_Streaming_Error_ReturnsReasoningFailed(t *testing.T) {
	exec := &stubReasoningExecutor{result: &ReasoningResult{}, err: errors.New("boom")}
	step := NewReasoningStep(exec, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})
	state.streamSink = func(ReasoningEvent) {}

	if err := step.Execute(context.Background(), state); err == nil {
		t.Fatal("expected an error when the stream reports a terminal failure")
	}
}

func TestReasoningStep_Streaming_InitError_ReturnsReasoningFailed(t *testing.T) {
	exec := &stubReasoningExecutor{streamInit: errors.New("cannot open stream")}
	step := NewReasoningStep(exec, time.Second, nil, 0, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})
	state.streamSink = func(ReasoningEvent) {}

	if err := step.Execute(context.Background(), state); err == nil {
		t.Fatal("expected an error when the stream cannot be opened")
	}
}

func TestReasoningStep_HeartbeatCallsExtendLock(t *testing.T) {
	var extendCalls atomic.Int64
	bond := &extendTrackingBond{extendFn: func() { extendCalls.Add(1) }}

	exec := &stubReasoningExecutor{
		result: &ReasoningResult{FinalResponse: "ok"},
		delay:  250 * time.Millisecond,
	}
	step := NewReasoningStep(exec, time.Second, bond, 150*time.Millisecond, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})
	state.LockAcquired = true
	state.LockKey = "memory:user:1"

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extendCalls.Load() == 0 {
		t.Error("expected ExtendLock to be called at least once during reasoning")
	}
}

func TestReasoningStep_HeartbeatSkipped_WhenLockNotAcquired(t *testing.T) {
	var extendCalls atomic.Int64
	bond := &extendTrackingBond{extendFn: func() { extendCalls.Add(1) }}

	exec := &stubReasoningExecutor{result: &ReasoningResult{FinalResponse: "ok"}}
	step := NewReasoningStep(exec, time.Second, bond, 300*time.Millisecond, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})
	state.LockAcquired = false

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extendCalls.Load() != 0 {
		t.Errorf("expected no ExtendLock calls when lock not acquired, got %d", extendCalls.Load())
	}
}

func TestReasoningStep_HeartbeatExtendError_DoesNotAbortReasoning(t *testing.T) {
	bond := &extendTrackingBond{
		extendFn:  func() {},
		extendErr: errors.New("redis timeout"),
	}
	exec := &stubReasoningExecutor{
		result: &ReasoningResult{FinalResponse: "ok"},
		delay:  120 * time.Millisecond,
	}
	step := NewReasoningStep(exec, time.Second, bond, 300*time.Millisecond, testLogger(t))
	state := reasoningState(&entities.Spirit{Type: entities.SpiritTypeConversational})
	state.LockAcquired = true
	state.LockKey = "memory:user:1"

	err := step.Execute(context.Background(), state)
	if err != nil {
		t.Errorf("ExtendLock error must not abort reasoning, got: %v", err)
	}
}

type extendTrackingBond struct {
	extendFn  func()
	extendErr error
}

func (b *extendTrackingBond) AcquireLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}
func (b *extendTrackingBond) ReleaseLock(_ context.Context, _ string) error { return nil }
func (b *extendTrackingBond) ExtendLock(_ context.Context, _ string, _ time.Duration) error {
	if b.extendFn != nil {
		b.extendFn()
	}
	return b.extendErr
}

type stubSpiritSelectorImpl struct {
	spiritName string
}

var _ SpiritSelector = (*stubSpiritSelectorImpl)(nil)

func (s *stubSpiritSelectorImpl) selectSpirit(_ context.Context, _ *entities.Pulse, _ *entities.Link) string {
	return s.spiritName
}

func TestSpiritSelectionStep_PopulatesSpiritName(t *testing.T) {
	selector := &stubSpiritSelectorImpl{spiritName: "support_spirit"}
	step := NewSpiritSelectionStep(selector, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: "user:1"},
		EventConfig: &entities.Link{AllowedSpirits: []string{"support_spirit", "sales_spirit"}},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if state.SpiritName != "support_spirit" {
		t.Errorf("expected SpiritName 'support_spirit', got '%s'", state.SpiritName)
	}
}

func TestSpiritSelectionStep_SetsPathfinderUsed(t *testing.T) {
	selector := &stubSpiritSelectorImpl{spiritName: "support_spirit"}
	step := NewSpiritSelectionStep(selector, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: "user:1"},
		EventConfig: &entities.Link{PathfinderName: "llm_pathfinder"},
	}

	_ = step.Execute(context.Background(), state)

	if state.PathfinderUsed != "llm_pathfinder" {
		t.Errorf("expected PathfinderUsed 'llm_pathfinder', got '%s'", state.PathfinderUsed)
	}
}

type stubInteractionLoggerImpl struct {
	called bool
	err    error
}

var _ InteractionLogger = (*stubInteractionLoggerImpl)(nil)

func (s *stubInteractionLoggerImpl) logInteraction(_ context.Context, _ *ProcessingState) error {
	s.called = true
	return s.err
}

func TestAuditLogStep_CallsLogInteraction(t *testing.T) {
	logger := &stubInteractionLoggerImpl{}
	step := NewAuditLogStep(logger, time.Second, testLogger(t))
	state := &ProcessingState{Event: &entities.Pulse{}}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !logger.called {
		t.Error("expected logInteraction to be called")
	}
}

func TestAuditLogStep_LogError_IsSwallowed(t *testing.T) {
	logger := &stubInteractionLoggerImpl{err: errors.New("chronicle db down")}
	step := NewAuditLogStep(logger, time.Second, testLogger(t))
	state := &ProcessingState{Event: &entities.Pulse{}}

	err := step.Execute(context.Background(), state)
	if err != nil {
		t.Errorf("expected error to be swallowed, got: %v", err)
	}
}
