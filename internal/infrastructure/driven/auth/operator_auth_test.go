package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type mockOperatorRepo struct {
	byEmail map[string]*entities.Operator
	byID    map[string]*entities.Operator
}

func (r *mockOperatorRepo) FindByEmail(_ context.Context, email string) (*entities.Operator, error) {
	op, ok := r.byEmail[email]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return op, nil
}
func (r *mockOperatorRepo) FindByID(_ context.Context, id string) (*entities.Operator, error) {
	op, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return op, nil
}
func (r *mockOperatorRepo) Create(_ context.Context, _ *entities.Operator) error { return nil }
func (r *mockOperatorRepo) List(_ context.Context, _, _ int) ([]*entities.Operator, int64, error) {
	return nil, 0, nil
}
func (r *mockOperatorRepo) Update(_ context.Context, _ *entities.Operator) error { return nil }
func (r *mockOperatorRepo) Deactivate(_ context.Context, _ string) error         { return nil }

func testOperatorRepo(t *testing.T, password string) *mockOperatorRepo {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	op := &entities.Operator{
		ID:           "op-1",
		Email:        "admin@eywa.io",
		Role:         ports.RoleAdmin,
		PasswordHash: hash,
		IsActive:     true,
	}
	return &mockOperatorRepo{
		byEmail: map[string]*entities.Operator{op.Email: op},
		byID:    map[string]*entities.Operator{op.ID: op},
	}
}

func TestOperatorAuth_Login_Success(t *testing.T) {
	repo := testOperatorRepo(t, "correct-pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	token, expiresAt, err := auth.Login(context.Background(), "admin@eywa.io", "correct-pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if expiresAt.Before(time.Now()) {
		t.Error("token already expired on creation")
	}
}

func TestOperatorAuth_Login_WrongPassword(t *testing.T) {
	repo := testOperatorRepo(t, "correct-pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	if _, _, err := auth.Login(context.Background(), "admin@eywa.io", "wrong-pass"); err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestOperatorAuth_Login_UnknownEmail(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	if _, _, err := auth.Login(context.Background(), "unknown@eywa.io", "pass"); err == nil {
		t.Error("expected error for unknown email")
	}
}

func TestOperatorAuth_Validate_RoundTrip(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	token, _, err := auth.Login(context.Background(), "admin@eywa.io", "pass")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	claims, err := auth.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Role != ports.RoleAdmin {
		t.Errorf("want role %s, got %s", ports.RoleAdmin, claims.Role)
	}
	if claims.Subject != "op-1" {
		t.Errorf("want subject op-1, got %s", claims.Subject)
	}
}

func TestOperatorAuth_Validate_WrongSecret(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	auth1 := NewOperatorAuth(repo, []byte("secret-1"))
	auth2 := NewOperatorAuth(repo, []byte("secret-2"))

	token, _, _ := auth1.Login(context.Background(), "admin@eywa.io", "pass")

	if _, err := auth2.Validate(context.Background(), token); err == nil {
		t.Error("expected validation failure with different secret")
	}
}

func TestOperatorAuth_Validate_InvalidToken(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	if _, err := auth.Validate(context.Background(), "not.a.jwt"); err == nil {
		t.Error("expected error for invalid token format")
	}
}

func TestOperatorAuth_Login_InactiveAccount_ReturnsError(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	repo.byEmail["admin@eywa.io"].IsActive = false
	auth := NewOperatorAuth(repo, []byte("secret"))

	if _, _, err := auth.Login(context.Background(), "admin@eywa.io", "pass"); err == nil {
		t.Error("expected error for inactive account")
	}
}

func TestHashPassword_Roundtrip(t *testing.T) {
	hash, err := HashPassword("my-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "my-password" {
		t.Error("hash must differ from plaintext")
	}
}

func TestHashPassword_TooLong_ReturnsError(t *testing.T) {
	// bcrypt rejects passwords > 72 bytes
	tooLong := string(make([]byte, 73))
	_, err := HashPassword(tooLong)
	if err == nil {
		t.Error("expected error for password > 72 bytes")
	}
}
