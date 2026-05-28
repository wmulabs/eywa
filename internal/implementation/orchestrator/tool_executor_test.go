package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubAction struct {
	name       string
	category   ports.ActionCategory
	critical   bool
	validateErr error
	execResult string
	execErr    error
}

func (a *stubAction) GetName() string                            { return a.name }
func (a *stubAction) GetDescription() string                     { return "stub action" }
func (a *stubAction) GetParameters() map[string]any      { return nil }
func (a *stubAction) IsCritical() bool                           { return a.critical }
func (a *stubAction) GetCategory() ports.ActionCategory          { return a.category }
func (a *stubAction) Validate(_ map[string]any) error    { return a.validateErr }
func (a *stubAction) Execute(_ context.Context, _ map[string]any) (string, error) {
	return a.execResult, a.execErr
}

type stubActionRegistry struct {
	actions map[string]ports.Action
	getErr  error
}

func (r *stubActionRegistry) Register(_ ports.Action) error           { return nil }
func (r *stubActionRegistry) RegisterMultiple(_ ...ports.Action) error { return nil }
func (r *stubActionRegistry) Get(name string) (ports.Action, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if a, ok := r.actions[name]; ok {
		return a, nil
	}
	return nil, errors.New("not found")
}
func (r *stubActionRegistry) GetMultiple(names []string) ([]ports.Action, error) {
	var result []ports.Action
	for _, n := range names {
		if a, ok := r.actions[n]; ok {
			result = append(result, a)
		}
	}
	return result, nil
}
func (r *stubActionRegistry) Execute(_ context.Context, name string, args map[string]any) (string, error) {
	a, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return a.Execute(context.Background(), args)
}
func (r *stubActionRegistry) List() []ports.Action      { return nil }
func (r *stubActionRegistry) IsRegistered(name string) bool { _, ok := r.actions[name]; return ok }
func (r *stubActionRegistry) ListRegistered() []string  { return nil }
func (r *stubActionRegistry) GetActionDefinitions(_ context.Context, _ []string) ([]entities.ActionDefinition, error) {
	return nil, nil
}

func newRegistry(actions ...ports.Action) *stubActionRegistry {
	m := make(map[string]ports.Action)
	for _, a := range actions {
		m[a.GetName()] = a
	}
	return &stubActionRegistry{actions: m}
}

func call(name string) ports.OracleToolCall {
	return ports.OracleToolCall{Name: name, Arguments: map[string]any{}}
}

// --- WithRetry ---

func TestWithRetry_ZeroAttempts_DefaultsToOne(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	exec.WithRetry(ActionRetryConfig{MaxAttempts: 0})
	if exec.retryConfig.MaxAttempts != 1 {
		t.Errorf("expected 1, got %d", exec.retryConfig.MaxAttempts)
	}
}

func TestWithRetry_MultipleAttemptsWithZeroDelay_SetsMinDelay(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	exec.WithRetry(ActionRetryConfig{MaxAttempts: 3, BaseDelay: 0})
	if exec.retryConfig.BaseDelay != minRetryDelay {
		t.Errorf("expected minRetryDelay, got %v", exec.retryConfig.BaseDelay)
	}
}

func TestWithRetry_NegativeDelay_SetsZero(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	exec.WithRetry(ActionRetryConfig{MaxAttempts: 1, BaseDelay: -time.Second})
	if exec.retryConfig.BaseDelay != 0 {
		t.Errorf("expected 0 delay, got %v", exec.retryConfig.BaseDelay)
	}
}

// --- ExecuteAll ---

func TestExecuteAll_EmptyCallList(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	results := exec.ExecuteAll(context.Background(), nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestExecuteAll_SingleCall_Success(t *testing.T) {
	action := &stubAction{name: "greet", execResult: "Hello!"}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{call("greet")})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("unexpected error: %v", results[0].Error)
	}
	if results[0].Result != "Hello!" {
		t.Errorf("expected Hello!, got %q", results[0].Result)
	}
}

func TestExecuteAll_ActionNotFound(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{call("unknown")})
	if results[0].Error == nil {
		t.Error("expected error for unknown action")
	}
}

func TestExecuteAll_ValidationFails(t *testing.T) {
	action := &stubAction{name: "pay", validateErr: errors.New("missing amount")}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{call("pay")})
	if results[0].Error == nil {
		t.Error("expected error for validation failure")
	}
}

func TestExecuteAll_BusinessError_NotRetried(t *testing.T) {
	action := &stubAction{
		name:    "pay",
		execErr: domainerrors.NewBusinessError("insufficient funds"),
	}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	exec.WithRetry(ActionRetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond})
	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{call("pay")})
	if results[0].Error == nil {
		t.Error("expected error for business failure")
	}
}

func TestExecuteAll_InfrastructureError_Retried_ThenSucceeds(t *testing.T) {
	callCount := 0
	action := &actionWithCallCount{name: "send", failTimes: 1, counter: &callCount, result: "sent"}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	exec.WithRetry(ActionRetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond})

	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{call("send")})
	if results[0].Error != nil {
		t.Errorf("expected success after retry, got: %v", results[0].Error)
	}
	if results[0].Result != "sent" {
		t.Errorf("expected 'sent', got %q", results[0].Result)
	}
}

func TestExecuteAll_InfrastructureError_AllRetriesExhausted(t *testing.T) {
	callCount := 0
	action := &actionWithCallCount{name: "send", failTimes: 99, counter: &callCount}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	exec.WithRetry(ActionRetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond})

	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{call("send")})
	if results[0].Error == nil {
		t.Error("expected error after all retries exhausted")
	}
}

func TestExecuteAll_Parallel_MultipleActions(t *testing.T) {
	a1 := &stubAction{name: "a1", execResult: "r1"}
	a2 := &stubAction{name: "a2", execResult: "r2"}
	exec := NewActionExecutor(newRegistry(a1, a2), true, testLogger(t), noopTracer())
	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{call("a1"), call("a2")})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("unexpected error for %s: %v", r.ActionName, r.Error)
		}
	}
}

func TestExecuteAll_CriticalOverride(t *testing.T) {
	action := &stubAction{name: "pay", critical: false, execResult: "ok"}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	override := true
	c := ports.OracleToolCall{Name: "pay", IsCriticalOverride: &override, Arguments: map[string]any{}}
	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{c})
	if !results[0].IsCritical {
		t.Error("expected IsCritical=true from override")
	}
}

func TestExecuteAll_DeliveryCategory_SetsIsVoiceDelivery(t *testing.T) {
	action := &stubAction{name: "send_msg", category: ports.ActionDelivery, execResult: "sent"}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{call("send_msg")})
	if !results[0].IsVoiceDelivery {
		t.Error("expected IsVoiceDelivery=true for delivery action")
	}
}

func TestExecuteAll_ContextCancelledDuringRetry(t *testing.T) {
	callCount := 0
	action := &actionWithCallCount{name: "slow", failTimes: 99, counter: &callCount}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	exec.WithRetry(ActionRetryConfig{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the retry delay gets interrupted.
	cancel()

	results := exec.ExecuteAll(ctx, []ports.OracleToolCall{call("slow")})
	if results[0].Error == nil {
		t.Error("expected error when context is cancelled during retry")
	}
}

// --- GetAction ---

func TestGetAction_Found(t *testing.T) {
	action := &stubAction{name: "greet"}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	a, err := exec.GetAction("greet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.GetName() != "greet" {
		t.Errorf("expected greet, got %s", a.GetName())
	}
}

func TestGetAction_NotFound(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	_, err := exec.GetAction("unknown")
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

// --- helpers ---

type actionWithCallCount struct {
	name      string
	failTimes int
	counter   *int
	result    string
}

func (a *actionWithCallCount) GetName() string                          { return a.name }
func (a *actionWithCallCount) GetDescription() string                   { return "" }
func (a *actionWithCallCount) GetParameters() map[string]any    { return nil }
func (a *actionWithCallCount) IsCritical() bool                         { return false }
func (a *actionWithCallCount) GetCategory() ports.ActionCategory        { return ports.ActionGeneral }
func (a *actionWithCallCount) Validate(_ map[string]any) error  { return nil }
func (a *actionWithCallCount) Execute(_ context.Context, _ map[string]any) (string, error) {
	*a.counter++
	if *a.counter <= a.failTimes {
		return "", domainerrors.NewInfrastructureError("transient", nil)
	}
	return a.result, nil
}

func TestExecuteOne_ZeroMaxAttempts_DefaultsToOne(t *testing.T) {
	// Bypass WithRetry normalization by directly setting retryConfig.MaxAttempts = 0.
	// executeOne must still execute at least once (clamp to 1).
	action := &stubAction{name: "pay", execResult: "ok"}
	reg := newRegistry(action)
	exec := NewActionExecutor(reg, false, testLogger(t), noopTracer())
	exec.retryConfig.MaxAttempts = 0 // bypass normalization

	
	results := exec.ExecuteAll(context.Background(), []ports.OracleToolCall{call("pay")})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("unexpected error: %v", results[0].Error)
	}
}
