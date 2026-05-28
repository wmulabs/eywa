package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
)

type ScheduleRitualTool struct {
	manager ports.RitualManager
}

func NewScheduleRitualAction(manager ports.RitualManager) ports.Action {
	return &ScheduleRitualTool{manager: manager}
}

func (t *ScheduleRitualTool) GetName() string { return "schedule_ritual" }

func (t *ScheduleRitualTool) GetDescription() string {
	return "Schedules an event to execute at a specific future time. " +
		"Use when the user requests an action at a later date, or when current conditions are not met but may be in the future. " +
		"For repeating tasks, provide a recurrence config."
}

func (t *ScheduleRitualTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"event_key": map[string]any{
				"type":        "string",
				"description": "Event type to execute (e.g. 'start_route', 'send_reminder').",
			},
			"execute_at": map[string]any{
				"type":        "string",
				"description": "RFC3339 datetime for execution (e.g. '2026-04-25T08:00:00-05:00'). Must be in the future.",
			},
			"user_message": map[string]any{
				"type":        "string",
				"description": "Optional message to include in the scheduled event.",
			},
			"data": map[string]any{
				"type":        "object",
				"description": "Optional key-value data to include in the event payload.",
			},
			"recurrence": map[string]any{
				"type":        "object",
				"description": "Optional recurrence config for repeating tasks.",
				"properties": map[string]any{
					"cron":     map[string]any{"type": "string", "description": "Standard 5-field cron expression (e.g. '0 8 * * 1-5')."},
					"timezone": map[string]any{"type": "string", "description": "IANA timezone (e.g. 'America/Mexico_City')."},
					"ends_at":  map[string]any{"type": "string", "description": "Optional RFC3339 end datetime after which no more recurrences are scheduled."},
					"max_runs": map[string]any{"type": "integer", "description": "Maximum number of executions. 0 means unlimited."},
				},
				"required": []string{"cron"},
			},
		},
		"required": []string{"event_key", "execute_at"},
	}
}

func (t *ScheduleRitualTool) Validate(args map[string]any) error {
	if eventKey, ok := args["event_key"].(string); !ok || eventKey == "" {
		return domainerrors.NewBusinessError("missing or invalid 'event_key'")
	}
	executeAt, ok := args["execute_at"].(string)
	if !ok || executeAt == "" {
		return domainerrors.NewBusinessError("missing or invalid 'execute_at'")
	}
	t2, err := time.Parse(time.RFC3339, executeAt)
	if err != nil {
		return domainerrors.NewBusinessError("'execute_at' must be a valid RFC3339 datetime")
	}
	if !t2.After(time.Now()) {
		return domainerrors.NewBusinessError("'execute_at' must be in the future")
	}

	if recRaw, ok := args["recurrence"].(map[string]any); ok {
		cronExpr, _ := recRaw["cron"].(string)
		if cronExpr == "" {
			return domainerrors.NewBusinessError("recurrence.cron is required")
		}
		if _, err := cron.ParseStandard(cronExpr); err != nil {
			return domainerrors.NewBusinessError("recurrence.cron: invalid cron expression")
		}
		if endsAt, _ := recRaw["ends_at"].(string); endsAt != "" {
			if _, err := time.Parse(time.RFC3339, endsAt); err != nil {
				return domainerrors.NewBusinessError("recurrence.ends_at must be a valid RFC3339 datetime")
			}
		}
	}

	return nil
}

func (t *ScheduleRitualTool) IsCritical() bool                  { return false }
func (t *ScheduleRitualTool) GetCategory() ports.ActionCategory { return ports.ActionGeneral }

func (t *ScheduleRitualTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	session, ok := ctx.Value(ports.SessionContextKey{}).(*entities.Memory)
	if !ok || session == nil {
		return "", domainerrors.NewInfrastructureError("session not available in context")
	}

	executeAt, _ := time.Parse(time.RFC3339, args["execute_at"].(string))

	event := &entities.Pulse{
		ID:          helpers.GenerateRandomID(),
		MemoryKey:   session.MemoryKey,
		UserMessage: stringArg(args, "user_message"),
		Timestamp:   helpers.NowUTC(),
		Payload:     mapArg(args, "data"),
		Metadata:    make(map[string]any),
	}

	req := ports.RitualRequest{
		EventKey:   args["event_key"].(string),
		Event:      event,
		ExecuteAt:  executeAt,
		CreatedBy:  "llm",
		Recurrence: parseRecurrence(args),
	}

	task, err := t.manager.Schedule(ctx, req)
	if err != nil {
		return "", fmt.Errorf("schedule event: %w", err)
	}

	return fmt.Sprintf(`{"task_id":"%s","execute_at":"%s"}`, task.ID, task.ExecuteAt.Format(time.RFC3339)), nil
}

type ListRitualsTool struct {
	manager ports.RitualManager
}

func NewListRitualsAction(manager ports.RitualManager) ports.Action {
	return &ListRitualsTool{manager: manager}
}

func (t *ListRitualsTool) GetName() string                   { return "list_rituals" }
func (t *ListRitualsTool) IsCritical() bool                  { return false }
func (t *ListRitualsTool) GetCategory() ports.ActionCategory { return ports.ActionGeneral }

func (t *ListRitualsTool) GetDescription() string {
	return "Lists pending rituals for the current memory. " +
		"Use before scheduling to check for existing rituals and avoid duplicates."
}

func (t *ListRitualsTool) GetParameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (t *ListRitualsTool) Validate(_ map[string]any) error { return nil }

func (t *ListRitualsTool) Execute(ctx context.Context, _ map[string]any) (string, error) {
	session, ok := ctx.Value(ports.SessionContextKey{}).(*entities.Memory)
	if !ok || session == nil {
		return "", domainerrors.NewInfrastructureError("session not available in context")
	}

	tasks, err := t.manager.ListPendingByMemoryKey(ctx, session.MemoryKey, 50, 0)
	if err != nil {
		return "", fmt.Errorf("list scheduled tasks: %w", err)
	}

	type taskSummary struct {
		TaskID     string                     `json:"task_id"`
		EventKey   string                     `json:"event_key"`
		ExecuteAt  string                     `json:"execute_at"`
		Recurrence *entities.RecurrenceConfig `json:"recurrence,omitempty"`
	}

	summaries := make([]taskSummary, 0, len(tasks))
	for _, task := range tasks {
		summaries = append(summaries, taskSummary{
			TaskID:     task.ID,
			EventKey:   task.EventKey,
			ExecuteAt:  task.ExecuteAt.Format(time.RFC3339),
			Recurrence: task.Recurrence,
		})
	}

	b, _ := json.Marshal(summaries)
	return string(b), nil
}

type CancelRitualTool struct {
	manager ports.RitualManager
}

func NewCancelRitualAction(manager ports.RitualManager) ports.Action {
	return &CancelRitualTool{manager: manager}
}

func (t *CancelRitualTool) GetName() string                   { return "cancel_ritual" }
func (t *CancelRitualTool) IsCritical() bool                  { return false }
func (t *CancelRitualTool) GetCategory() ports.ActionCategory { return ports.ActionGeneral }

func (t *CancelRitualTool) GetDescription() string {
	return "Cancels a pending ritual. " +
		"Only meaningful before the execution time. " +
		"Use list_rituals to find the task_id."
}

func (t *CancelRitualTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "Task ID returned by schedule_event or list_scheduled_tasks.",
			},
		},
		"required": []string{"task_id"},
	}
}

func (t *CancelRitualTool) Validate(args map[string]any) error {
	if id, ok := args["task_id"].(string); !ok || id == "" {
		return domainerrors.NewBusinessError("missing or invalid 'task_id'")
	}
	return nil
}

func (t *CancelRitualTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	session, ok := ctx.Value(ports.SessionContextKey{}).(*entities.Memory)
	if !ok || session == nil {
		return "", domainerrors.NewInfrastructureError("session not available in context")
	}

	id := args["task_id"].(string)

	if err := t.manager.Cancel(ctx, id, session.MemoryKey); err != nil {
		return "", err
	}

	return `{"cancelled":true}`, nil
}

func stringArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func mapArg(args map[string]any, key string) map[string]any {
	if v, ok := args[key].(map[string]any); ok {
		return v
	}
	return nil
}

func parseRecurrence(args map[string]any) *entities.RecurrenceConfig {
	raw, ok := args["recurrence"].(map[string]any)
	if !ok {
		return nil
	}

	rec := &entities.RecurrenceConfig{
		Cron:     stringArg(raw, "cron"),
		Timezone: stringArg(raw, "timezone"),
	}

	if endsAtStr := stringArg(raw, "ends_at"); endsAtStr != "" {
		if t, err := time.Parse(time.RFC3339, endsAtStr); err == nil {
			rec.EndsAt = &t
		}
	}

	if maxRuns, ok := raw["max_runs"].(float64); ok {
		rec.MaxRuns = int(maxRuns)
	}

	return rec
}
