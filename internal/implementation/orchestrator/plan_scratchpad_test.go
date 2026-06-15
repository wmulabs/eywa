package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func updatePlanResp(items ...map[string]any) *ports.OracleResponse {
	anyItems := make([]any, len(items))
	for i, it := range items {
		anyItems[i] = it
	}
	return &ports.OracleResponse{
		StopReason: ports.StopReasonToolCalls,
		ToolCalls: []ports.OracleToolCall{
			{ID: "p", Name: updatePlanActionName, Arguments: map[string]any{"items": anyItems}},
		},
	}
}

func TestAvailableActions_IncludesUpdatePlanWhenEnabled(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(&stubOracle{}), exec, 5)
	svc.SetPlanPolicy(PlanPolicy{Enabled: true})

	has := false
	for _, a := range svc.availableActions(testRequest()) {
		if a.Name == updatePlanActionName {
			has = true
		}
	}
	if !has {
		t.Error("update_plan must be offered when the plan policy is enabled")
	}
}

func TestAvailableActions_OmitsUpdatePlanWhenDisabled(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(&stubOracle{}), exec, 5)

	for _, a := range svc.availableActions(testRequest()) {
		if a.Name == updatePlanActionName {
			t.Error("update_plan must not be offered when disabled")
		}
	}
}

func TestExecute_Plan_RecordsFinalPlan(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		updatePlanResp(map[string]any{"title": "greet", "status": "done"}),
		{Content: "done", StopReason: ports.StopReasonComplete},
	}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetPlanPolicy(PlanPolicy{Enabled: true, MaxItems: 12})

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "done" {
		t.Errorf("expected 'done', got %q", result.FinalResponse)
	}
	if len(result.Plan) != 1 || result.Plan[0].Status != entities.PlanDone {
		t.Errorf("expected final plan [greet:done], got %+v", result.Plan)
	}
}

func TestExecute_Plan_NudgesOnIncompletePlan(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		updatePlanResp(map[string]any{"title": "task", "status": "in_progress"}),
		{Content: "premature", StopReason: ports.StopReasonComplete}, // incomplete -> nudge
		{Content: "final answer", StopReason: ports.StopReasonComplete},
	}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetPlanPolicy(PlanPolicy{Enabled: true, MaxItems: 12})

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "final answer" {
		t.Errorf("nudge should have prevented premature stop; got %q", result.FinalResponse)
	}
}

func TestApplyUpdatePlan_InvalidArgs_ReturnsBusinessError(t *testing.T) {
	p := newPlanState(12)
	res := applyUpdatePlan(ports.OracleToolCall{Name: updatePlanActionName, Arguments: map[string]any{}}, p)
	if res.Error == nil {
		t.Error("invalid update_plan args must return an error result")
	}
	if res.IsCritical {
		t.Error("plan parse failure must be non-critical so the model self-corrects")
	}
}

func TestApplyUpdatePlan_Valid_MutatesAndConfirms(t *testing.T) {
	p := newPlanState(12)
	res := applyUpdatePlan(ports.OracleToolCall{
		Name:      updatePlanActionName,
		Arguments: map[string]any{"items": []any{map[string]any{"title": "x", "status": "pending"}}},
	}, p)
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if len(p.items) != 1 || !strings.Contains(res.Result, "Plan updated") {
		t.Errorf("expected plan mutated and confirmation, got %+v / %q", p.items, res.Result)
	}
}

func TestExecuteActionsWithPlan_NilPlan_Delegates(t *testing.T) {
	action := &stubAction{name: "do", execResult: "ok", category: ports.ActionGeneral}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(&stubOracle{}), exec, 5)

	results := svc.executeActionsWithPlan(context.Background(), testRequest(), []ports.OracleToolCall{call("do")}, nil)
	if len(results) != 1 || results[0].Result != "ok" {
		t.Errorf("nil plan must delegate to the normal executor, got %+v", results)
	}
}

func TestExecuteActionsWithPlan_MixesPlanAndRegularActions(t *testing.T) {
	action := &stubAction{name: "do", execResult: "ok", category: ports.ActionGeneral}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(&stubOracle{}), exec, 5)
	p := newPlanState(12)

	calls := []ports.OracleToolCall{
		{Name: updatePlanActionName, Arguments: map[string]any{"items": []any{map[string]any{"title": "t", "status": "done"}}}},
		call("do"),
	}
	results := svc.executeActionsWithPlan(context.Background(), testRequest(), calls, p)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !strings.Contains(results[0].Result, "Plan updated") {
		t.Errorf("first result should be the plan update, got %q", results[0].Result)
	}
	if results[1].Result != "ok" {
		t.Errorf("second result should be the regular action, got %q", results[1].Result)
	}
}

func TestExecute_Plan_Required_InjectsPlanningInstruction(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "done", StopReason: ports.StopReasonComplete}}
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetPlanPolicy(PlanPolicy{Enabled: true, Required: true})

	if _, err := svc.Execute(context.Background(), testRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(oracle.gotReq.SystemPrompt, "PLANNING") {
		t.Errorf("Required plan policy must inject the planning instruction, got %q", oracle.gotReq.SystemPrompt)
	}
}

func TestExecute_Plan_Disabled_NoPlanRecorded(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &multiOracle{responses: []*ports.OracleResponse{{Content: "hi", StopReason: ports.StopReasonComplete}}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Plan != nil {
		t.Errorf("expected no plan when disabled, got %+v", result.Plan)
	}
}

func TestPlanState_Apply_ReplacesItems(t *testing.T) {
	p := newPlanState(12)
	p.apply([]entities.PlanItem{
		{Title: "a", Status: entities.PlanPending},
		{Title: "b", Status: entities.PlanDone},
	})
	if len(p.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(p.items))
	}
	// apply replaces, not appends
	p.apply([]entities.PlanItem{{Title: "c", Status: entities.PlanInProgress}})
	if len(p.items) != 1 || p.items[0].Title != "c" {
		t.Errorf("apply must replace the plan, got %+v", p.items)
	}
}

func TestPlanState_Apply_ClampsToMaxItems(t *testing.T) {
	p := newPlanState(2)
	p.apply([]entities.PlanItem{
		{Title: "a", Status: entities.PlanPending},
		{Title: "b", Status: entities.PlanPending},
		{Title: "c", Status: entities.PlanPending},
	})
	if len(p.items) != 2 {
		t.Errorf("expected clamp to MaxItems=2, got %d", len(p.items))
	}
}

func TestPlanState_Render_ShowsStatusMarkers(t *testing.T) {
	p := newPlanState(12)
	p.apply([]entities.PlanItem{
		{Title: "research order", Status: entities.PlanPending},
		{Title: "check refund", Status: entities.PlanInProgress},
		{Title: "greet user", Status: entities.PlanDone},
	})
	out := p.render()
	if !strings.Contains(out, "CURRENT PLAN") {
		t.Error("render must include a plan header")
	}
	for _, want := range []string{"research order", "check refund", "greet user"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing item %q in %q", want, out)
		}
	}
}

func TestPlanState_Render_EmptyWhenNoItems(t *testing.T) {
	p := newPlanState(12)
	if p.render() != "" {
		t.Error("an empty plan must render to empty string (no injection)")
	}
}

func TestPlanState_Incomplete_ListsPendingAndInProgress(t *testing.T) {
	p := newPlanState(12)
	p.apply([]entities.PlanItem{
		{Title: "done-item", Status: entities.PlanDone},
		{Title: "pending-item", Status: entities.PlanPending},
		{Title: "wip-item", Status: entities.PlanInProgress},
		{Title: "abandoned-item", Status: entities.PlanAbandoned},
	})
	inc := p.incomplete()
	if len(inc) != 2 {
		t.Fatalf("expected 2 incomplete (pending + in_progress), got %v", inc)
	}
}

func TestParsePlanItems_Valid(t *testing.T) {
	args := map[string]any{"items": []any{
		map[string]any{"title": "step one", "status": "pending"},
		map[string]any{"title": "step two", "status": "in_progress"},
	}}
	items, err := parsePlanItems(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 || items[1].Status != entities.PlanInProgress {
		t.Errorf("unexpected parse result: %+v", items)
	}
}

func TestParsePlanItems_InvalidStatus_Errors(t *testing.T) {
	args := map[string]any{"items": []any{
		map[string]any{"title": "x", "status": "bogus"},
	}}
	if _, err := parsePlanItems(args); err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestParsePlanItems_MissingItems_Errors(t *testing.T) {
	if _, err := parsePlanItems(map[string]any{}); err == nil {
		t.Error("expected error when 'items' is absent")
	}
}

func TestParsePlanItems_NonObjectItem_Errors(t *testing.T) {
	args := map[string]any{"items": []any{"not an object"}}
	if _, err := parsePlanItems(args); err == nil {
		t.Error("expected error when an item is not an object")
	}
}

func TestNewPlanState_DefaultsMaxItems(t *testing.T) {
	if got := newPlanState(0); got.maxItems != defaultMaxPlanItems {
		t.Errorf("expected default max items %d, got %d", defaultMaxPlanItems, got.maxItems)
	}
}

func TestPlanMarker_AllStatuses(t *testing.T) {
	cases := map[entities.PlanStatus]string{
		entities.PlanDone:            "[x]",
		entities.PlanInProgress:      "[~]",
		entities.PlanAbandoned:       "[-]",
		entities.PlanPending:         "[ ]",
		entities.PlanStatus("weird"): "[ ]",
	}
	for status, want := range cases {
		if got := planMarker(status); got != want {
			t.Errorf("planMarker(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestUpdatePlanTool_HasNameAndItemsSchema(t *testing.T) {
	tool := updatePlanTool()
	if tool.Name != updatePlanActionName {
		t.Errorf("expected tool name %q, got %q", updatePlanActionName, tool.Name)
	}
	props, _ := tool.Parameters["properties"].(map[string]any)
	if _, ok := props["items"]; !ok {
		t.Error("update_plan schema must expose an 'items' property")
	}
}

func TestPlanState_Incomplete_NoneWhenAllResolved(t *testing.T) {
	p := newPlanState(12)
	p.apply([]entities.PlanItem{
		{Title: "x", Status: entities.PlanDone},
		{Title: "y", Status: entities.PlanAbandoned},
	})
	if len(p.incomplete()) != 0 {
		t.Error("expected no incomplete items when all done/abandoned")
	}
}
