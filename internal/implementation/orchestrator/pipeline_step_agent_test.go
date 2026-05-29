package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubSpiritRepo struct {
	spirit *entities.Spirit
	err    error
}

var _ ports.SpiritRepository = (*stubSpiritRepo)(nil)

func (r *stubSpiritRepo) FindActiveByName(_ context.Context, _ string) (*entities.Spirit, error) {
	return r.spirit, r.err
}

func (r *stubSpiritRepo) Create(context.Context, *entities.Spirit) error {
	panic("not implemented")
}

func (r *stubSpiritRepo) Update(context.Context, string, *entities.Spirit, string) (*entities.Spirit, error) {
	panic("not implemented")
}

func (r *stubSpiritRepo) FindByID(context.Context, string) (*entities.Spirit, error) {
	panic("not implemented")
}

func (r *stubSpiritRepo) FindActiveByNames(context.Context, []string) (map[string]*entities.Spirit, error) {
	panic("not implemented")
}

func (r *stubSpiritRepo) GetVersion(context.Context, string, int) (*entities.Spirit, error) {
	panic("not implemented")
}

func (r *stubSpiritRepo) FindVersionHistory(context.Context, string) ([]*entities.Spirit, error) {
	panic("not implemented")
}

func (r *stubSpiritRepo) Activate(context.Context, string, int) error {
	panic("not implemented")
}

func (r *stubSpiritRepo) Deactivate(context.Context, string) error {
	panic("not implemented")
}

func (r *stubSpiritRepo) RestoreVersion(context.Context, string) error {
	panic("not implemented")
}

func (r *stubSpiritRepo) ListActive(context.Context, int, int) ([]*entities.Spirit, error) {
	panic("not implemented")
}

func (r *stubSpiritRepo) CountActive(context.Context) (int64, error) {
	panic("not implemented")
}

func (r *stubSpiritRepo) ListAll(context.Context) ([]*entities.Spirit, error) {
	panic("not implemented")
}

func TestSpiritLoadStep_Execute_HappyPath(t *testing.T) {
	spirit := &entities.Spirit{Name: "my_spirit"}
	repo := &stubSpiritRepo{spirit: spirit}
	step := NewSpiritLoadStep(repo, 0, testLogger(t))
	state := &ProcessingState{SpiritName: "my_spirit"}

	err := step.Execute(context.Background(), state)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if state.Spirit == nil {
		t.Error("state.Spirit must not be nil")
	}
	if state.Spirit.Name != "my_spirit" {
		t.Errorf("expected spirit name 'my_spirit', got %q", state.Spirit.Name)
	}
}

func TestSpiritLoadStep_Execute_RepoError(t *testing.T) {
	repo := &stubSpiritRepo{spirit: nil, err: errors.New("db down")}
	step := NewSpiritLoadStep(repo, 0, testLogger(t))
	state := &ProcessingState{SpiritName: "missing"}

	err := step.Execute(context.Background(), state)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if state.Spirit != nil {
		t.Error("state.Spirit must be nil")
	}
	var oe *OrchestrationError
	if !errors.As(err, &oe) || oe.Code != "SPIRIT_NOT_FOUND" {
		t.Errorf("expected SPIRIT_NOT_FOUND OrchestrationError, got: %v", err)
	}
}
