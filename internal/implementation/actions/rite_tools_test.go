package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubPubSubRite struct {
	published int
}

var _ ports.PubSub = (*stubPubSubRite)(nil)

func (p *stubPubSubRite) Publish(_ context.Context, _, _ string) error {
	p.published++
	return nil
}
func (p *stubPubSubRite) Subscribe(_ context.Context, _ string, _ func(string)) error {
	panic("not implemented")
}

type stubRiteRepo struct {
	created []*entities.Rite
	err     error
}

func (s *stubRiteRepo) Create(_ context.Context, rite *entities.Rite) error {
	s.created = append(s.created, rite)
	return s.err
}
func (s *stubRiteRepo) FindByID(_ context.Context, _ string) (*entities.Rite, error) {
	return nil, nil
}
func (s *stubRiteRepo) List(_ context.Context, _ ports.RiteListOptions) ([]*entities.Rite, int64, error) {
	return nil, 0, nil
}
func (s *stubRiteRepo) Decide(_ context.Context, _, _ string, _ entities.RiteStatus) error {
	return nil
}

var _ ports.RiteRepository = (*stubRiteRepo)(nil)

func riteCtx(memoryKey, subjectKey string) context.Context {
	mem := &entities.Memory{MemoryKey: memoryKey, SubjectKey: subjectKey}
	return context.WithValue(context.Background(), ports.SessionContextKey{}, mem)
}

func TestRequestRiteAction_Execute_CreatesPendingRite(t *testing.T) {
	repo := &stubRiteRepo{}
	action := NewRequestRiteAction(repo, nil)

	ctx := riteCtx("whatsapp:+5511999999999", "")
	result, err := action.Execute(ctx, map[string]any{
		"event_key": "resume_order",
		"reason":    "Large order requires approval",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 rite created, got %d", len(repo.created))
	}
	rite := repo.created[0]
	if rite.EventKey != "resume_order" {
		t.Errorf("want event_key=resume_order, got %s", rite.EventKey)
	}
	if rite.Status != entities.RitePending {
		t.Errorf("want status=pending, got %s", rite.Status)
	}
	if rite.MemoryKey != "whatsapp:+5511999999999" {
		t.Errorf("want memory_key=whatsapp:+5511999999999, got %s", rite.MemoryKey)
	}
	if rite.ID == "" {
		t.Error("expected non-empty rite ID")
	}
	if result == "" {
		t.Error("expected non-empty result JSON")
	}
}

func TestRequestRiteAction_Validate_MissingEventKey_ReturnsBizError(t *testing.T) {
	action := NewRequestRiteAction(&stubRiteRepo{}, nil)
	err := action.Validate(map[string]any{"reason": "some reason"})
	if err == nil {
		t.Error("expected error for missing event_key")
	}
}

func TestRequestRiteAction_Validate_MissingReason_ReturnsBizError(t *testing.T) {
	action := NewRequestRiteAction(&stubRiteRepo{}, nil)
	err := action.Validate(map[string]any{"event_key": "some_event"})
	if err == nil {
		t.Error("expected error for missing reason")
	}
}

func TestRequestRiteAction_Execute_NoSession_ReturnsInfraError(t *testing.T) {
	action := NewRequestRiteAction(&stubRiteRepo{}, nil)
	_, err := action.Execute(context.Background(), map[string]any{
		"event_key": "resume",
		"reason":    "needs approval",
	})
	if err == nil {
		t.Error("expected error when session missing from context")
	}
}

func TestRequestRiteAction_Validate_ValidArgs(t *testing.T) {
	action := NewRequestRiteAction(&stubRiteRepo{}, nil)
	err := action.Validate(map[string]any{
		"event_key": "resume_order",
		"reason":    "large order requires approval",
	})
	if err != nil {
		t.Errorf("expected no error for valid args, got %v", err)
	}
}

func TestRequestRiteAction_Execute_RepoCreateError(t *testing.T) {
	repo := &stubRiteRepo{err: errors.New("db failure")}
	action := NewRequestRiteAction(repo, nil)
	_, err := action.Execute(riteCtx("user:123", ""), map[string]any{
		"event_key": "resume_order",
		"reason":    "needs approval",
	})
	if err == nil {
		t.Error("expected error when riteRepo.Create fails")
	}
}

func TestRequestRiteAction_Execute_WithPubSub_PublishesEvent(t *testing.T) {
	repo := &stubRiteRepo{}
	pubSub := &stubPubSubRite{}
	action := NewRequestRiteAction(repo, pubSub)
	_, err := action.Execute(riteCtx("user:123", "topic-a"), map[string]any{
		"event_key": "resume_order",
		"reason":    "approval needed",
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if pubSub.published == 0 {
		t.Error("expected pubSub.Publish to be called once")
	}
}

func TestRequestRiteAction_Metadata(t *testing.T) {
	a := &RequestRiteAction{}
	if a.GetName() != "request_rite" {
		t.Errorf("GetName = %q, want %q", a.GetName(), "request_rite")
	}
	if a.GetDescription() == "" {
		t.Error("GetDescription returned empty")
	}
	if a.GetParameters() == nil {
		t.Error("GetParameters returned nil")
	}
	if !a.IsCritical() {
		t.Error("IsCritical should return true")
	}
	if a.GetCategory() != ports.ActionGeneral {
		t.Errorf("GetCategory = %v, want ActionGeneral", a.GetCategory())
	}
}
