package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

const updatePlanActionName = "update_plan"

const planRequiredInstruction = "\n\n--- PLANNING ---\n" +
	"Before acting, lay out your plan using the update_plan action, and keep it current as you work."

// executeActionsWithPlan runs Action calls, handling the built-in update_plan in-process (mutating
// the turn plan) and delegating the rest to the normal executor. plan is nil when planning is off.
func (r *ReasoningService) executeActionsWithPlan(ctx context.Context, req *ReasoningRequest, calls []ports.OracleToolCall, plan *planState) []ActionResult {
	if plan == nil {
		return r.executeAllActions(ctx, req, calls)
	}

	results := make([]ActionResult, len(calls))
	rest := make([]ports.OracleToolCall, 0, len(calls))
	restIdx := make([]int, 0, len(calls))
	for i, c := range calls {
		if c.Name == updatePlanActionName {
			results[i] = applyUpdatePlan(c, plan)
		} else {
			rest = append(rest, c)
			restIdx = append(restIdx, i)
		}
	}

	if len(rest) > 0 {
		sub := r.executeAllActions(ctx, req, rest)
		for j, res := range sub {
			results[restIdx[j]] = res
		}
	}
	return results
}

// applyUpdatePlan mutates the turn plan from a tool call. A parse/validation failure is returned as
// a non-critical business error so the model self-corrects on the next iteration.
func applyUpdatePlan(call ports.OracleToolCall, plan *planState) ActionResult {
	items, err := parsePlanItems(call.Arguments)
	if err != nil {
		return ActionResult{ActionName: call.Name, Error: errors.NewBusinessError(err.Error())}
	}
	plan.apply(items)
	return ActionResult{ActionName: call.Name, Result: "Plan updated." + plan.render()}
}

func planNudgeMessage(incomplete []string) ports.OracleMessage {
	return ports.OracleMessage{
		Role: ports.RoleUser,
		Content: "These plan items are still open: " + strings.Join(incomplete, "; ") +
			". Complete or abandon them via update_plan, or confirm you are done.",
	}
}

// PlanPolicy enables a turn-scoped plan/scratchpad the model maintains via the update_plan action.
// The plan is injected into each iteration's context to keep multi-step tasks coherent. Disabled by default.
type PlanPolicy struct {
	Enabled  bool `json:"enabled"`
	MaxItems int  `json:"max_items"`
	Required bool `json:"required"`
}

const defaultMaxPlanItems = 12

// planState holds the agent's plan for a single turn. Not safe for concurrent use; one per Execute.
type planState struct {
	items    []entities.PlanItem
	maxItems int
}

func newPlanState(maxItems int) *planState {
	if maxItems <= 0 {
		maxItems = defaultMaxPlanItems
	}
	return &planState{maxItems: maxItems}
}

// apply replaces the plan with items, clamped to maxItems.
func (p *planState) apply(items []entities.PlanItem) {
	if len(items) > p.maxItems {
		items = items[:p.maxItems]
	}
	p.items = items
}

// render returns a compact plan block for the system prompt, or "" when the plan is empty.
func (p *planState) render() string {
	if len(p.items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n--- CURRENT PLAN ---\n")
	for _, item := range p.items {
		b.WriteString(planMarker(item.Status))
		b.WriteString(" ")
		b.WriteString(item.Title)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// incomplete returns the titles of items still pending or in progress.
func (p *planState) incomplete() []string {
	var inc []string
	for _, item := range p.items {
		if item.Status == entities.PlanPending || item.Status == entities.PlanInProgress {
			inc = append(inc, item.Title)
		}
	}
	return inc
}

func validPlanStatus(s entities.PlanStatus) bool {
	switch s {
	case entities.PlanPending, entities.PlanInProgress, entities.PlanDone, entities.PlanAbandoned:
		return true
	default:
		return false
	}
}

// parsePlanItems converts the update_plan tool arguments into PlanItems, validating each status.
func parsePlanItems(args map[string]any) ([]entities.PlanItem, error) {
	raw, ok := args["items"].([]any)
	if !ok {
		return nil, fmt.Errorf("update_plan requires an 'items' array")
	}
	items := make([]entities.PlanItem, 0, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each plan item must be an object")
		}
		title, _ := m["title"].(string)
		status := entities.PlanStatus(fmt.Sprintf("%v", m["status"]))
		if !validPlanStatus(status) {
			return nil, fmt.Errorf("invalid plan item status %q", status)
		}
		items = append(items, entities.PlanItem{Title: title, Status: status})
	}
	return items, nil
}

func updatePlanTool() ports.OracleTool {
	return ports.OracleTool{
		Name: updatePlanActionName,
		Description: "Maintain your task plan. Call with the full list of plan items and their current " +
			"status each time the plan changes. Statuses: pending, in_progress, done, abandoned.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title":  map[string]any{"type": "string", "description": "Short description of the step."},
							"status": map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "done", "abandoned"}},
						},
						"required": []string{"title", "status"},
					},
				},
			},
			"required": []string{"items"},
		},
	}
}

func planMarker(status entities.PlanStatus) string {
	switch status {
	case entities.PlanDone:
		return "[x]"
	case entities.PlanInProgress:
		return "[~]"
	case entities.PlanAbandoned:
		return "[-]"
	case entities.PlanPending:
		return "[ ]"
	default:
		return "[ ]"
	}
}
