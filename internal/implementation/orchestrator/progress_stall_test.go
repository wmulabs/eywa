package orchestrator

import (
	"context"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

func toolCallResp(name string, args map[string]any) *ports.OracleResponse {
	return &ports.OracleResponse{
		StopReason: ports.StopReasonToolCalls,
		ToolCalls:  []ports.OracleToolCall{{ID: "c", Name: name, Arguments: args}},
	}
}

func TestCallSignature_StableAcrossArgOrdering(t *testing.T) {
	a := ports.OracleToolCall{Name: "lookup", Arguments: map[string]any{"id": "X", "region": "mx"}}
	b := ports.OracleToolCall{Name: "lookup", Arguments: map[string]any{"region": "mx", "id": "X"}}

	if callSignature(a) != callSignature(b) {
		t.Error("identical name+args must produce the same signature regardless of map ordering")
	}
}

func TestCallSignature_DiffersOnArgs(t *testing.T) {
	a := ports.OracleToolCall{Name: "lookup", Arguments: map[string]any{"id": "X"}}
	b := ports.OracleToolCall{Name: "lookup", Arguments: map[string]any{"id": "Y"}}

	if callSignature(a) == callSignature(b) {
		t.Error("different args must produce different signatures")
	}
}

func TestCallSignature_DiffersOnName(t *testing.T) {
	a := ports.OracleToolCall{Name: "lookup", Arguments: map[string]any{"id": "X"}}
	b := ports.OracleToolCall{Name: "delete", Arguments: map[string]any{"id": "X"}}

	if callSignature(a) == callSignature(b) {
		t.Error("different action names must produce different signatures")
	}
}

func TestCallSignature_EmptyArgsStable(t *testing.T) {
	a := ports.OracleToolCall{Name: "ping"}
	b := ports.OracleToolCall{Name: "ping", Arguments: map[string]any{}}

	if callSignature(a) != callSignature(b) {
		t.Error("nil and empty args must produce the same signature")
	}
}

func TestStallTracker_DisabledWindow_NeverStalls(t *testing.T) {
	tr := newStallTracker(0)
	repeat := []ports.OracleToolCall{{Name: "lookup", Arguments: map[string]any{"id": "X"}}}
	for range 10 {
		if tr.observe(repeat) {
			t.Fatal("window=0 must never report stalled")
		}
	}
}

func TestStallTracker_NewSignaturesEachIteration_NeverStalls(t *testing.T) {
	tr := newStallTracker(2)
	for i := range 10 {
		calls := []ports.OracleToolCall{{Name: "lookup", Arguments: map[string]any{"id": i}}}
		if tr.observe(calls) {
			t.Fatalf("distinct calls must not stall (iteration %d)", i)
		}
	}
}

func TestStallTracker_RepeatedSignature_StallsAfterWindow(t *testing.T) {
	tr := newStallTracker(2)
	repeat := []ports.OracleToolCall{{Name: "lookup", Arguments: map[string]any{"id": "X"}}}

	if tr.observe(repeat) {
		t.Fatal("first occurrence is progress, must not stall")
	}
	if tr.observe(repeat) {
		t.Fatal("first repeat (streak 1) must not stall yet with window 2")
	}
	if !tr.observe(repeat) {
		t.Fatal("second repeat (streak 2) must report stalled with window 2")
	}
}

func TestExecute_StallDetected_ForcesFinalSynthesis(t *testing.T) {
	action := &stubAction{name: "lookup", execResult: "no change", category: ports.ActionRetrieval}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())

	repeat := map[string]any{"id": "X"}
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		toolCallResp("lookup", repeat), // iter1: progress (new)
		toolCallResp("lookup", repeat), // iter2: streak 1
		toolCallResp("lookup", repeat), // iter3: streak 2 -> stalled, then synthesis
		{Content: "Best answer with what I have.", StopReason: ports.StopReasonComplete}, // synthesis
	}}

	svc := newReasoning(t, multiFactory(oracle), exec, 10)
	svc.SetProgressPolicy(ProgressPolicy{Enabled: true, StallWindow: 2})

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("forced synthesis must succeed with nil error, got %v", err)
	}
	if result.FinalResponse != "Best answer with what I have." {
		t.Errorf("expected synthesized final response, got %q", result.FinalResponse)
	}
	if result.IterationsUsed != 3 {
		t.Errorf("expected stall break at iteration 3, used %d", result.IterationsUsed)
	}
}

func TestExecute_StallDisabled_RunsToIterationCap(t *testing.T) {
	action := &stubAction{name: "lookup", execResult: "no change", category: ports.ActionRetrieval}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())

	repeat := map[string]any{"id": "X"}
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		toolCallResp("lookup", repeat),
		toolCallResp("lookup", repeat),
		toolCallResp("lookup", repeat),
	}}

	svc := newReasoning(t, multiFactory(oracle), exec, 3)
	// No SetProgressPolicy -> disabled: must spin to the cap, not synthesize early.

	result, err := svc.Execute(context.Background(), testRequest())
	if err == nil {
		t.Fatal("disabled policy must hit the iteration cap and return an error")
	}
	if result.IterationsUsed != 3 {
		t.Errorf("expected to run to cap (3 iterations), used %d", result.IterationsUsed)
	}
	if result.FinalResponse != "max reached" {
		t.Errorf("expected canned max-iterations message, got %q", result.FinalResponse)
	}
}

func TestExecute_IterationCapWithPolicy_ForcesFinalSynthesis(t *testing.T) {
	action := &stubAction{name: "lookup", execResult: "ok", category: ports.ActionRetrieval}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())

	// Distinct args each iteration -> never stalls; loop ends via the cap.
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		toolCallResp("lookup", map[string]any{"id": "1"}),
		toolCallResp("lookup", map[string]any{"id": "2"}),
		{Content: "Synthesized at cap.", StopReason: ports.StopReasonComplete},
	}}

	svc := newReasoning(t, multiFactory(oracle), exec, 2)
	svc.SetProgressPolicy(ProgressPolicy{Enabled: true, StallWindow: 5})

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("forced synthesis at cap must return nil error, got %v", err)
	}
	if result.FinalResponse != "Synthesized at cap." {
		t.Errorf("expected synthesized response at cap, got %q", result.FinalResponse)
	}
}

func TestStallTracker_ProgressResetsStreak(t *testing.T) {
	tr := newStallTracker(2)
	repeat := []ports.OracleToolCall{{Name: "lookup", Arguments: map[string]any{"id": "X"}}}
	fresh := []ports.OracleToolCall{{Name: "lookup", Arguments: map[string]any{"id": "NEW"}}}

	tr.observe(repeat) // progress (new)
	tr.observe(repeat) // streak 1
	if tr.observe(fresh) {
		t.Fatal("a new signature must reset the streak")
	}
	if tr.observe(repeat) {
		t.Fatal("streak should have reset; single repeat must not stall")
	}
}
