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

type stubRitualManager struct {
	markRunningErr  error
	markExecutedErr error
}

var _ ports.RitualManager = (*stubRitualManager)(nil)

func (m *stubRitualManager) Schedule(_ context.Context, _ ports.RitualRequest) (*entities.Ritual, error) {
	panic("not implemented")
}
func (m *stubRitualManager) Cancel(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (m *stubRitualManager) MarkRunning(_ context.Context, _ string) error {
	return m.markRunningErr
}
func (m *stubRitualManager) MarkExecuted(_ context.Context, _ string) error {
	return m.markExecutedErr
}
func (m *stubRitualManager) MarkFailed(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (m *stubRitualManager) ListPendingByMemoryKey(_ context.Context, _ string, _, _ int) ([]*entities.Ritual, error) {
	panic("not implemented")
}
func (m *stubRitualManager) CountByMemoryKey(_ context.Context, _ string) (int64, error) {
	panic("not implemented")
}

func ritualState(ritualID string) *ProcessingState {
	event := &entities.Pulse{EventType: "ritual", MemoryKey: "user:1"}
	if ritualID != "" {
		event.Metadata = map[string]any{entities.MetadataKeyRitualID: ritualID}
	}
	return &ProcessingState{
		Event:       event,
		EventConfig: &entities.Link{},
	}
}

// --- RitualMarkStep ---

func TestRitualMarkStep_NoRitualID_NoOp(t *testing.T) {
	mgr := &stubRitualManager{}
	step := NewRitualMarkStep(mgr, time.Second, testLogger(t))
	state := ritualState("")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRitualMarkStep_MarkRunning_Success(t *testing.T) {
	mgr := &stubRitualManager{}
	step := NewRitualMarkStep(mgr, time.Second, testLogger(t))
	state := ritualState("task-123")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRitualMarkStep_TerminalError_ReturnsNonRetriableError(t *testing.T) {
	mgr := &stubRitualManager{markRunningErr: domainerrors.ErrRitualTerminal}
	step := NewRitualMarkStep(mgr, time.Second, testLogger(t))
	state := ritualState("task-123")

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatal("expected error for terminal task")
	}
	orchErr, ok := err.(*OrchestrationError)
	if !ok {
		t.Fatalf("expected *OrchestrationError, got %T", err)
	}
	if orchErr.Retriable {
		t.Error("expected Retriable=false for terminal error")
	}
	if orchErr.Code != "SCHEDULED_TASK_SKIP" {
		t.Errorf("expected code SCHEDULED_TASK_SKIP, got %s", orchErr.Code)
	}
}

func TestRitualMarkStep_NonTerminalError_LogsAndContinues(t *testing.T) {
	mgr := &stubRitualManager{markRunningErr: errors.New("db timeout")}
	step := NewRitualMarkStep(mgr, time.Second, testLogger(t))
	state := ritualState("task-123")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected non-terminal error swallowed, got: %v", err)
	}
}

// --- MarkExecutedStep ---

func TestMarkExecutedStep_NoRitualID_NoOp(t *testing.T) {
	mgr := &stubRitualManager{}
	step := NewMarkExecutedStep(mgr, time.Second, testLogger(t))
	state := ritualState("")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkExecutedStep_MarkExecuted_Success(t *testing.T) {
	mgr := &stubRitualManager{}
	step := NewMarkExecutedStep(mgr, time.Second, testLogger(t))
	state := ritualState("task-123")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkExecutedStep_MarkExecuted_Error_IsSwallowed(t *testing.T) {
	mgr := &stubRitualManager{markExecutedErr: errors.New("db error")}
	step := NewMarkExecutedStep(mgr, time.Second, testLogger(t))
	state := ritualState("task-123")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected error swallowed, got: %v", err)
	}
}
