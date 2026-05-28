package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubRitualManager struct {
	scheduled   *entities.Ritual
	scheduleErr error
	listed      []*entities.Ritual
	listErr     error
	cancelErr   error
	cancelledID string
}

var _ ports.RitualManager = (*stubRitualManager)(nil)

func (m *stubRitualManager) Schedule(_ context.Context, _ ports.RitualRequest) (*entities.Ritual, error) {
	return m.scheduled, m.scheduleErr
}
func (m *stubRitualManager) Cancel(_ context.Context, id, _ string) error {
	m.cancelledID = id
	return m.cancelErr
}
func (m *stubRitualManager) ListPendingByMemoryKey(_ context.Context, _ string, _, _ int) ([]*entities.Ritual, error) {
	return m.listed, m.listErr
}
func (m *stubRitualManager) MarkRunning(_ context.Context, _ string) error  { panic("not implemented") }
func (m *stubRitualManager) MarkExecuted(_ context.Context, _ string) error { panic("not implemented") }
func (m *stubRitualManager) MarkFailed(_ context.Context, _ string, _ string) error {
	panic("not implemented")
}
func (m *stubRitualManager) CountByMemoryKey(_ context.Context, _ string) (int64, error) {
	panic("not implemented")
}

func futureRFC3339() string {
	return time.Now().Add(24 * time.Hour).Format(time.RFC3339)
}

func ritualSessionCtx(memoryKey string) context.Context {
	mem := &entities.Memory{MemoryKey: memoryKey}
	return context.WithValue(context.Background(), ports.SessionContextKey{}, mem)
}

func TestScheduleRitualTool_Validate_MissingEventKey(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{"execute_at": futureRFC3339()})
	if err == nil {
		t.Error("expected error for missing event_key")
	}
}

func TestScheduleRitualTool_Validate_MissingExecuteAt(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{"event_key": "start_route"})
	if err == nil {
		t.Error("expected error for missing execute_at")
	}
}

func TestScheduleRitualTool_Validate_InvalidExecuteAtFormat(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{
		"event_key":  "start_route",
		"execute_at": "not-a-date",
	})
	if err == nil {
		t.Error("expected error for invalid execute_at format")
	}
}

func TestScheduleRitualTool_Validate_ExecuteAtInPast(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	err := tool.Validate(map[string]any{
		"event_key":  "start_route",
		"execute_at": past,
	})
	if err == nil {
		t.Error("expected error for execute_at in the past")
	}
}

func TestScheduleRitualTool_Validate_ValidArgs(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{
		"event_key":  "start_route",
		"execute_at": futureRFC3339(),
	})
	if err != nil {
		t.Errorf("expected no error for valid args, got %v", err)
	}
}

func TestScheduleRitualTool_Validate_RecurrenceEmptyCron(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{
		"event_key":  "start_route",
		"execute_at": futureRFC3339(),
		"recurrence": map[string]any{"cron": ""},
	})
	if err == nil {
		t.Error("expected error for empty recurrence.cron")
	}
}

func TestScheduleRitualTool_Validate_RecurrenceInvalidCron(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{
		"event_key":  "start_route",
		"execute_at": futureRFC3339(),
		"recurrence": map[string]any{"cron": "not a valid cron"},
	})
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestScheduleRitualTool_Validate_RecurrenceInvalidEndsAt(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{
		"event_key":  "start_route",
		"execute_at": futureRFC3339(),
		"recurrence": map[string]any{
			"cron":    "0 8 * * 1-5",
			"ends_at": "not-a-date",
		},
	})
	if err == nil {
		t.Error("expected error for invalid recurrence.ends_at")
	}
}

func TestScheduleRitualTool_Validate_ValidRecurrence(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{
		"event_key":  "send_reminder",
		"execute_at": futureRFC3339(),
		"recurrence": map[string]any{
			"cron":    "0 8 * * 1-5",
			"ends_at": time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Errorf("expected no error for valid recurrence, got %v", err)
	}
}

func TestScheduleRitualTool_Execute_NoSession(t *testing.T) {
	tool := NewScheduleRitualAction(&stubRitualManager{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"event_key":  "start_route",
		"execute_at": futureRFC3339(),
	})
	if err == nil {
		t.Error("expected error when session missing from context")
	}
}

func TestScheduleRitualTool_Execute_SchedulerError(t *testing.T) {
	manager := &stubRitualManager{scheduleErr: errors.New("keeper unavailable")}
	tool := NewScheduleRitualAction(manager)
	_, err := tool.Execute(ritualSessionCtx("user:123"), map[string]any{
		"event_key":  "start_route",
		"execute_at": futureRFC3339(),
	})
	if err == nil {
		t.Error("expected error when scheduler fails")
	}
}

func TestScheduleRitualTool_Execute_Success(t *testing.T) {
	executeAt := time.Now().Add(24 * time.Hour)
	manager := &stubRitualManager{
		scheduled: &entities.Ritual{ID: "task-abc", ExecuteAt: executeAt},
	}
	tool := NewScheduleRitualAction(manager)
	result, err := tool.Execute(ritualSessionCtx("user:123"), map[string]any{
		"event_key":  "start_route",
		"execute_at": futureRFC3339(),
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result JSON")
	}
}

func TestListRitualsTool_Execute_NoSession(t *testing.T) {
	tool := NewListRitualsAction(&stubRitualManager{})
	_, err := tool.Execute(context.Background(), nil)
	if err == nil {
		t.Error("expected error when session missing from context")
	}
}

func TestListRitualsTool_Execute_ManagerError(t *testing.T) {
	manager := &stubRitualManager{listErr: errors.New("db down")}
	tool := NewListRitualsAction(manager)
	_, err := tool.Execute(ritualSessionCtx("user:123"), nil)
	if err == nil {
		t.Error("expected error when manager.ListPendingByMemoryKey fails")
	}
}

func TestListRitualsTool_Execute_EmptyList(t *testing.T) {
	tool := NewListRitualsAction(&stubRitualManager{listed: []*entities.Ritual{}})
	result, err := tool.Execute(ritualSessionCtx("user:123"), nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "[]" {
		t.Errorf("expected '[]', got '%s'", result)
	}
}

func TestListRitualsTool_Execute_ReturnsTasks(t *testing.T) {
	manager := &stubRitualManager{
		listed: []*entities.Ritual{
			{ID: "t1", EventKey: "start_route", ExecuteAt: time.Now().Add(time.Hour)},
		},
	}
	tool := NewListRitualsAction(manager)
	result, err := tool.Execute(ritualSessionCtx("user:123"), nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == "" || result == "[]" {
		t.Errorf("expected non-empty task list, got '%s'", result)
	}
}

func TestCancelRitualTool_Validate_MissingTaskID(t *testing.T) {
	tool := NewCancelRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{"task_id": ""})
	if err == nil {
		t.Error("expected error for missing task_id")
	}
}

func TestCancelRitualTool_Validate_ValidTaskID(t *testing.T) {
	tool := NewCancelRitualAction(&stubRitualManager{})
	err := tool.Validate(map[string]any{"task_id": "abc-123"})
	if err != nil {
		t.Errorf("expected no error for valid task_id, got %v", err)
	}
}

func TestCancelRitualTool_Execute_NoSession(t *testing.T) {
	tool := NewCancelRitualAction(&stubRitualManager{})
	_, err := tool.Execute(context.Background(), map[string]any{"task_id": "abc-123"})
	if err == nil {
		t.Error("expected error when session missing from context")
	}
}

func TestCancelRitualTool_Execute_CancelError(t *testing.T) {
	manager := &stubRitualManager{cancelErr: errors.New("task not found")}
	tool := NewCancelRitualAction(manager)
	_, err := tool.Execute(ritualSessionCtx("user:123"), map[string]any{"task_id": "abc-123"})
	if err == nil {
		t.Error("expected error when manager.Cancel fails")
	}
}

func TestCancelRitualTool_Execute_Success(t *testing.T) {
	manager := &stubRitualManager{}
	tool := NewCancelRitualAction(manager)
	result, err := tool.Execute(ritualSessionCtx("user:123"), map[string]any{"task_id": "abc-123"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != `{"cancelled":true}` {
		t.Errorf("expected cancelled JSON, got '%s'", result)
	}
	if manager.cancelledID != "abc-123" {
		t.Errorf("expected cancelled ID 'abc-123', got '%s'", manager.cancelledID)
	}
}

func TestStringArg_Present_ReturnsValue(t *testing.T) {
	val := stringArg(map[string]any{"key": "hello"}, "key")
	if val != "hello" {
		t.Errorf("expected 'hello', got '%s'", val)
	}
}

func TestStringArg_Missing_ReturnsEmpty(t *testing.T) {
	val := stringArg(map[string]any{}, "key")
	if val != "" {
		t.Errorf("expected empty string, got '%s'", val)
	}
}

func TestMapArg_Present_ReturnsMap(t *testing.T) {
	m := map[string]any{"x": 1}
	result := mapArg(map[string]any{"data": m}, "data")
	if result == nil || result["x"] != 1 {
		t.Errorf("expected map with x=1, got %v", result)
	}
}

func TestMapArg_Missing_ReturnsNil(t *testing.T) {
	result := mapArg(map[string]any{}, "data")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseRecurrence_NoRecurrenceKey_ReturnsNil(t *testing.T) {
	result := parseRecurrence(map[string]any{})
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseRecurrence_WithCronAndTimezone(t *testing.T) {
	result := parseRecurrence(map[string]any{
		"recurrence": map[string]any{
			"cron":     "0 8 * * 1-5",
			"timezone": "America/Mexico_City",
		},
	})
	if result == nil {
		t.Fatal("expected non-nil RecurrenceConfig")
	}
	if result.Cron != "0 8 * * 1-5" {
		t.Errorf("expected cron '0 8 * * 1-5', got '%s'", result.Cron)
	}
	if result.Timezone != "America/Mexico_City" {
		t.Errorf("expected timezone 'America/Mexico_City', got '%s'", result.Timezone)
	}
}

func TestParseRecurrence_WithEndsAt_ParsesTime(t *testing.T) {
	endsAt := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	result := parseRecurrence(map[string]any{
		"recurrence": map[string]any{
			"cron":    "0 8 * * *",
			"ends_at": endsAt,
		},
	})
	if result == nil || result.EndsAt == nil {
		t.Fatal("expected non-nil EndsAt")
	}
}

func TestParseRecurrence_WithMaxRuns_SetsMaxRuns(t *testing.T) {
	result := parseRecurrence(map[string]any{
		"recurrence": map[string]any{
			"cron":     "0 8 * * *",
			"max_runs": float64(5),
		},
	})
	if result == nil {
		t.Fatal("expected non-nil RecurrenceConfig")
	}
	if result.MaxRuns != 5 {
		t.Errorf("expected MaxRuns=5, got %d", result.MaxRuns)
	}
}

func TestScheduleRitualTool_Metadata(t *testing.T) {
	a := &ScheduleRitualTool{}
	if a.GetName() != "schedule_ritual" {
		t.Errorf("GetName = %q, want %q", a.GetName(), "schedule_ritual")
	}
	if a.GetDescription() == "" {
		t.Error("GetDescription returned empty")
	}
	if a.GetParameters() == nil {
		t.Error("GetParameters returned nil")
	}
	if a.IsCritical() {
		t.Error("IsCritical should return false")
	}
	if a.GetCategory() != ports.ActionGeneral {
		t.Errorf("GetCategory = %v, want ActionGeneral", a.GetCategory())
	}
}

func TestListRitualsTool_Metadata(t *testing.T) {
	a := &ListRitualsTool{}
	if a.GetName() != "list_rituals" {
		t.Errorf("GetName = %q, want %q", a.GetName(), "list_rituals")
	}
	if a.GetDescription() == "" {
		t.Error("GetDescription returned empty")
	}
	if a.GetParameters() == nil {
		t.Error("GetParameters returned nil")
	}
	if a.IsCritical() {
		t.Error("IsCritical should return false")
	}
	if a.GetCategory() != ports.ActionGeneral {
		t.Errorf("GetCategory = %v, want ActionGeneral", a.GetCategory())
	}
}

func TestCancelRitualTool_Metadata(t *testing.T) {
	a := &CancelRitualTool{}
	if a.GetName() != "cancel_ritual" {
		t.Errorf("GetName = %q, want %q", a.GetName(), "cancel_ritual")
	}
	if a.GetDescription() == "" {
		t.Error("GetDescription returned empty")
	}
	if a.GetParameters() == nil {
		t.Error("GetParameters returned nil")
	}
	if a.IsCritical() {
		t.Error("IsCritical should return false")
	}
	if a.GetCategory() != ports.ActionGeneral {
		t.Errorf("GetCategory = %v, want ActionGeneral", a.GetCategory())
	}
}

func TestListRitualsTool_Validate(t *testing.T) {
	a := &ListRitualsTool{}
	if err := a.Validate(map[string]any{}); err != nil {
		t.Errorf("Validate should return nil, got %v", err)
	}
}
