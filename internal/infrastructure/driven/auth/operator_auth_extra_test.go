package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// mockOperatorRepoFull extends mockOperatorRepo with controllable errors.
type mockOperatorRepoFull struct {
	mockOperatorRepo
	createErr     error
	listRes       []*entities.Operator
	listCount     int64
	listErr       error
	updateErr     error
	deactivateErr error
}

func (r *mockOperatorRepoFull) Create(_ context.Context, _ *entities.Operator) error {
	return r.createErr
}
func (r *mockOperatorRepoFull) List(_ context.Context, _, _ int) ([]*entities.Operator, int64, error) {
	return r.listRes, r.listCount, r.listErr
}
func (r *mockOperatorRepoFull) Update(_ context.Context, _ *entities.Operator) error {
	return r.updateErr
}
func (r *mockOperatorRepoFull) Deactivate(_ context.Context, _ string) error {
	return r.deactivateErr
}

func TestOperatorAuth_WithTokenTTL(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	a := NewOperatorAuth(repo, []byte("secret")).WithTokenTTL(24 * time.Hour)

	_, expiresAt, err := a.Login(context.Background(), "admin@eywa.io", "pass")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if time.Until(expiresAt) < 23*time.Hour {
		t.Errorf("expected ~24h TTL, got %v until expiry", time.Until(expiresAt))
	}
}

func TestOperatorAuth_Login_InactiveAccount(t *testing.T) {
	hash, _ := HashPassword("pass")
	op := &entities.Operator{
		ID:           "op-inactive",
		Email:        "inactive@eywa.io",
		Role:         ports.RoleAdmin,
		PasswordHash: hash,
		IsActive:     false,
	}
	repo := &mockOperatorRepo{
		byEmail: map[string]*entities.Operator{op.Email: op},
		byID:    map[string]*entities.Operator{op.ID: op},
	}
	a := NewOperatorAuth(repo, []byte("secret"))
	_, _, err := a.Login(context.Background(), "inactive@eywa.io", "pass")
	if err == nil {
		t.Fatal("expected error for inactive account")
	}
}

func TestOperatorAuth_CreateOperator(t *testing.T) {
	repo := &mockOperatorRepoFull{}
	a := NewOperatorAuth(repo, []byte("secret"))

	if err := a.CreateOperator(context.Background(), &entities.Operator{ID: "op-new", Email: "new@eywa.io"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOperatorAuth_CreateOperator_Error(t *testing.T) {
	repo := &mockOperatorRepoFull{createErr: errors.New("db error")}
	a := NewOperatorAuth(repo, []byte("secret"))

	if err := a.CreateOperator(context.Background(), &entities.Operator{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestOperatorAuth_ListOperators(t *testing.T) {
	ops := []*entities.Operator{{ID: "op-1"}, {ID: "op-2"}}
	repo := &mockOperatorRepoFull{listRes: ops, listCount: 2}
	a := NewOperatorAuth(repo, []byte("secret"))

	result, count, err := a.ListOperators(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 operators, got %d", len(result))
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
}

func TestOperatorAuth_FindOperatorByID(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	a := NewOperatorAuth(repo, []byte("secret"))

	op, err := a.FindOperatorByID(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.ID != "op-1" {
		t.Errorf("expected op-1, got %s", op.ID)
	}
}

func TestOperatorAuth_FindOperatorByID_NotFound(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	a := NewOperatorAuth(repo, []byte("secret"))

	_, err := a.FindOperatorByID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
}

func TestOperatorAuth_UpdateOperator(t *testing.T) {
	repo := &mockOperatorRepoFull{}
	a := NewOperatorAuth(repo, []byte("secret"))

	if err := a.UpdateOperator(context.Background(), &entities.Operator{ID: "op-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOperatorAuth_DeactivateOperator(t *testing.T) {
	repo := &mockOperatorRepoFull{}
	a := NewOperatorAuth(repo, []byte("secret"))

	if err := a.DeactivateOperator(context.Background(), "op-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOperatorAuth_Validate_MalformedToken(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	a := NewOperatorAuth(repo, []byte("secret"))

	if _, err := a.Validate(context.Background(), "not-a-real-jwt-token"); err == nil {
		t.Fatal("expected error for malformed token")
	}
}
