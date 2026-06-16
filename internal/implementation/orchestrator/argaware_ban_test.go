package orchestrator

import (
	"context"
	"sync"
	"testing"

	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// bizErrAction fails with a critical business error for configured ids and records every execution
// so a test can assert which calls actually ran (vs were skipped by an arg-aware ban).
type bizErrAction struct {
	mu       sync.Mutex
	execArgs []map[string]any
	failIDs  map[string]bool
}

func (a *bizErrAction) GetName() string                   { return "lookup" }
func (a *bizErrAction) GetDescription() string            { return "lookup by id" }
func (a *bizErrAction) GetParameters() map[string]any     { return nil }
func (a *bizErrAction) IsCritical() bool                  { return true }
func (a *bizErrAction) GetCategory() ports.ActionCategory { return ports.ActionRetrieval }
func (a *bizErrAction) Validate(_ map[string]any) error   { return nil }
func (a *bizErrAction) Execute(_ context.Context, args map[string]any) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.execArgs = append(a.execArgs, args)
	id, _ := args["id"].(string)
	if a.failIDs[id] {
		return "", domainerrors.NewBusinessError("record not found for " + id)
	}
	return "found " + id, nil
}

func (a *bizErrAction) execCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.execArgs)
}

func lookupCall(id string) *ports.OracleResponse {
	return &ports.OracleResponse{
		StopReason: ports.StopReasonToolCalls,
		ToolCalls:  []ports.OracleToolCall{{ID: "c", Name: "lookup", Arguments: map[string]any{"id": id}}},
	}
}

func TestExecute_ArgAwareBan_SkipsRepeatedSignature(t *testing.T) {
	action := &bizErrAction{failIDs: map[string]bool{"X": true}}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		lookupCall("X"), // fails (business, critical) -> signature banned
		lookupCall("X"), // identical -> must be skipped, not executed
		{Content: "done", StopReason: ports.StopReasonComplete},
	}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	if _, err := svc.Execute(context.Background(), testRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.execCount() != 1 {
		t.Errorf("a repeated identical call must be skipped after a critical business failure; executed %d times", action.execCount())
	}
}

func TestExecute_ArgAwareBan_DifferentArgsStillExecute(t *testing.T) {
	action := &bizErrAction{failIDs: map[string]bool{"X": true}} // Y succeeds
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		lookupCall("X"), // fails -> bans signature(id=X)
		lookupCall("Y"), // different args -> must still execute
		{Content: "done", StopReason: ports.StopReasonComplete},
	}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	if _, err := svc.Execute(context.Background(), testRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.execCount() != 2 {
		t.Errorf("a call with corrected arguments must still run; executed %d times", action.execCount())
	}
}
