package actions

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubImprintRepo struct {
	saveErr    error
	imprints   []entities.Imprint
	getErr     error
	deleteErr  error
	deletedIDs []string
}

var _ ports.ImprintRepository = (*stubImprintRepo)(nil)

func (r *stubImprintRepo) Save(_ context.Context, _ entities.Imprint) error {
	return r.saveErr
}

func (r *stubImprintRepo) GetByUserKey(_ context.Context, _ string, _ string) ([]entities.Imprint, error) {
	return r.imprints, r.getErr
}

func (r *stubImprintRepo) Delete(_ context.Context, id string) error {
	r.deletedIDs = append(r.deletedIDs, id)
	return r.deleteErr
}

func (r *stubImprintRepo) List(_ context.Context, _ ports.ImprintListOptions) ([]entities.Imprint, int64, error) {
	panic("not implemented")
}

func (r *stubImprintRepo) Prune(_ context.Context, _ string, _ string, _ int) error {
	panic("not implemented")
}

func ctxWithPulse(phone string) context.Context {
	pulse := &entities.Pulse{}
	pulse.ContactPhone = phone
	return context.WithValue(context.Background(), ports.PulseContextKey{}, pulse)
}

func TestRememberFactAction_Validate_EmptyFact(t *testing.T) {
	action := NewRememberFactAction(&stubImprintRepo{}, "test-spirit")
	err := action.Validate(map[string]any{"fact": ""})
	if err == nil {
		t.Errorf("expected error for empty fact, got nil")
	}
}

func TestRememberFactAction_Validate_ValidFact(t *testing.T) {
	action := NewRememberFactAction(&stubImprintRepo{}, "test-spirit")
	err := action.Validate(map[string]any{"fact": "Likes coffee"})
	if err != nil {
		t.Errorf("expected nil error for valid fact, got %v", err)
	}
}

func TestRememberFactAction_Execute_NoPulseInContext(t *testing.T) {
	action := NewRememberFactAction(&stubImprintRepo{}, "test-spirit")
	result, err := action.Execute(context.Background(), map[string]any{"fact": "Likes coffee"})
	if err == nil {
		t.Errorf("expected error when no pulse in context, got nil")
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestRememberFactAction_Execute_EmptyContactPhone(t *testing.T) {
	action := NewRememberFactAction(&stubImprintRepo{}, "test-spirit")
	result, err := action.Execute(ctxWithPulse(""), map[string]any{"fact": "Likes coffee"})
	if err == nil {
		t.Errorf("expected error when contact phone is empty, got nil")
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestRememberFactAction_Execute_SavesImprint(t *testing.T) {
	action := NewRememberFactAction(&stubImprintRepo{}, "test-spirit")
	result, err := action.Execute(ctxWithPulse("+5511999999999"), map[string]any{"fact": "Likes coffee"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result != "Remembered: Likes coffee" {
		t.Errorf("expected 'Remembered: Likes coffee', got %q", result)
	}
}

func TestRememberFactAction_Execute_RepoSaveError(t *testing.T) {
	repo := &stubImprintRepo{saveErr: errors.New("db failure")}
	action := NewRememberFactAction(repo, "test-spirit")
	_, err := action.Execute(ctxWithPulse("+5511999999999"), map[string]any{"fact": "Likes coffee"})
	if err == nil {
		t.Errorf("expected error when repo.Save fails, got nil")
	}
}

func TestForgetFactAction_Validate_EmptySubstring(t *testing.T) {
	action := NewForgetFactAction(&stubImprintRepo{}, "test-spirit")
	err := action.Validate(map[string]any{"fact_substring": ""})
	if err == nil {
		t.Errorf("expected error for empty fact_substring, got nil")
	}
}

func TestForgetFactAction_Execute_NoPulseInContext(t *testing.T) {
	action := NewForgetFactAction(&stubImprintRepo{}, "test-spirit")
	result, err := action.Execute(context.Background(), map[string]any{"fact_substring": "coffee"})
	if err == nil {
		t.Errorf("expected error when no pulse in context, got nil")
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestForgetFactAction_Validate_ValidSubstring(t *testing.T) {
	action := NewForgetFactAction(&stubImprintRepo{}, "test-spirit")
	err := action.Validate(map[string]any{"fact_substring": "coffee"})
	if err != nil {
		t.Errorf("expected nil error for valid fact_substring, got %v", err)
	}
}

func TestForgetFactAction_Execute_DeletesMatchingFacts(t *testing.T) {
	repo := &stubImprintRepo{
		imprints: []entities.Imprint{
			{ID: "1", Fact: "Likes coffee"},
			{ID: "2", Fact: "Prefers mornings"},
		},
	}
	action := NewForgetFactAction(repo, "test-spirit")
	result, err := action.Execute(ctxWithPulse("+5511999999999"), map[string]any{"fact_substring": "coffee"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !strings.Contains(result, "Forgot 1") {
		t.Errorf("expected result to contain 'Forgot 1', got %q", result)
	}
}

func TestForgetFactAction_Execute_GetByUserKeyError(t *testing.T) {
	repo := &stubImprintRepo{getErr: errors.New("db failure")}
	action := NewForgetFactAction(repo, "test-spirit")
	_, err := action.Execute(ctxWithPulse("+5511999999999"), map[string]any{"fact_substring": "coffee"})
	if err == nil {
		t.Error("expected error when GetByUserKey fails, got nil")
	}
}

func TestForgetFactAction_Execute_NoMatchingFacts(t *testing.T) {
	repo := &stubImprintRepo{
		imprints: []entities.Imprint{
			{ID: "1", Fact: "Prefers mornings"},
		},
	}
	action := NewForgetFactAction(repo, "test-spirit")
	result, err := action.Execute(ctxWithPulse("+5511999999999"), map[string]any{"fact_substring": "coffee"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !strings.Contains(result, "Forgot 0") {
		t.Errorf("expected result to contain 'Forgot 0', got %q", result)
	}
}

func TestForgetFactAction_Execute_DeleteErrorIsIgnored(t *testing.T) {
	repo := &stubImprintRepo{
		imprints:  []entities.Imprint{{ID: "1", Fact: "Likes coffee"}},
		deleteErr: errors.New("delete failed"),
	}
	action := NewForgetFactAction(repo, "test-spirit")
	result, err := action.Execute(ctxWithPulse("+5511999999999"), map[string]any{"fact_substring": "coffee"})
	if err != nil {
		t.Errorf("expected no error when delete fails silently, got %v", err)
	}
	if !strings.Contains(result, "Forgot 0") {
		t.Errorf("expected result to contain 'Forgot 0' when delete failed, got %q", result)
	}
}

func TestRememberFactAction_Execute_DefaultCategoryContext(t *testing.T) {
	action := NewRememberFactAction(&stubImprintRepo{}, "test-spirit")
	result, err := action.Execute(ctxWithPulse("+5511999999999"), map[string]any{"fact": "Loves pasta"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !strings.Contains(result, "Loves pasta") {
		t.Errorf("expected result to contain fact, got %q", result)
	}
}

func TestRememberFactAction_Metadata(t *testing.T) {
	a := &RememberFactAction{}
	if a.GetName() != "remember_fact" {
		t.Errorf("GetName = %q, want %q", a.GetName(), "remember_fact")
	}
	if a.GetDescription() == "" {
		t.Error("GetDescription returned empty")
	}
	if a.GetParameters() == nil {
		t.Error("GetParameters returned nil")
	}
	if a.GetCategory() != ports.ActionModification {
		t.Errorf("GetCategory = %v, want ActionModification", a.GetCategory())
	}
	if a.IsCritical() {
		t.Error("IsCritical should return false")
	}
}

func TestForgetFactAction_Metadata(t *testing.T) {
	a := &ForgetFactAction{}
	if a.GetName() != "forget_fact" {
		t.Errorf("GetName = %q, want %q", a.GetName(), "forget_fact")
	}
	if a.GetDescription() == "" {
		t.Error("GetDescription returned empty")
	}
	if a.GetParameters() == nil {
		t.Error("GetParameters returned nil")
	}
	if a.GetCategory() != ports.ActionModification {
		t.Errorf("GetCategory = %v, want ActionModification", a.GetCategory())
	}
	if a.IsCritical() {
		t.Error("IsCritical should return false")
	}
}
