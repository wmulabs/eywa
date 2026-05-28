package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// stubVigilRepo implements ports.VigilRepository for tests.
type stubVigilRepo struct {
	vigil *entities.Vigil
	err   error
}

func (s *stubVigilRepo) Acquire(_ context.Context, _, _ string, _ time.Duration) error {
	return s.err
}
func (s *stubVigilRepo) Release(_ context.Context, _ string) error { return s.err }
func (s *stubVigilRepo) Get(_ context.Context, _ string) (*entities.Vigil, error) {
	return s.vigil, s.err
}
func (s *stubVigilRepo) Refresh(_ context.Context, _ string, _ time.Duration) error {
	return s.err
}
func (s *stubVigilRepo) ListAll(_ context.Context) ([]*entities.Vigil, error) {
	return nil, s.err
}

var _ ports.VigilRepository = (*stubVigilRepo)(nil)

func stateForVigilTest(memoryKey string) *ProcessingState {
	return &ProcessingState{
		Event: &entities.Pulse{MemoryKey: memoryKey, UserMessage: "hello"},
	}
}

func TestVigilCheckStep_NilRepo_Proceeds(t *testing.T) {
	step := NewVigilCheckStep(nil, testLogger(t))
	state := stateForVigilTest("user-1")
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("nil repo must be a no-op, got: %v", err)
	}
}

func TestVigilCheckStep_NoVigil_Proceeds(t *testing.T) {
	repo := &stubVigilRepo{vigil: nil}
	step := NewVigilCheckStep(repo, testLogger(t))
	state := stateForVigilTest("user-1")
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("nil vigil must proceed, got: %v", err)
	}
}

func TestVigilCheckStep_VigilExists_ReturnsSessionHeld(t *testing.T) {
	now := time.Now()
	repo := &stubVigilRepo{vigil: &entities.Vigil{
		MemoryKey: "user-1", OperatorID: "op-1",
		SeatSince: now, ExpiresAt: now.Add(30 * time.Minute),
	}}
	step := NewVigilCheckStep(repo, testLogger(t))
	state := stateForVigilTest("user-1")
	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatal("expected ErrSessionHeld, got nil")
	}
	if !IsSessionHeld(err) {
		t.Errorf("expected ErrSessionHeld, got: %v", err)
	}
	if IsRetriable(err) {
		t.Error("ErrSessionHeld must be non-retriable")
	}
	if state.ProcessingStatus != "session_held" {
		t.Errorf("want ProcessingStatus=session_held, got %s", state.ProcessingStatus)
	}
}

func TestVigilCheckStep_RepoError_Proceeds(t *testing.T) {
	repo := &stubVigilRepo{err: errors.New("redis unavailable")}
	step := NewVigilCheckStep(repo, testLogger(t))
	state := stateForVigilTest("user-1")
	// fail-open: repo error must not block user message
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("repo error must degrade gracefully, got: %v", err)
	}
}
