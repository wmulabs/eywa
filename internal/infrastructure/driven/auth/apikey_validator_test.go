package auth

import (
	"context"
	"testing"
)

func TestAPIKeyValidator_ValidKey_ReturnsCorrectRole(t *testing.T) {
	v := NewAPIKeyValidator(map[string]string{
		"sk-admin-123": "admin",
		"sk-op-456":    "operator",
	})

	claims, err := v.Validate(context.Background(), "sk-admin-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Role != "admin" {
		t.Errorf("want role admin, got %s", claims.Role)
	}
	// Subject is a SHA-256 digest prefix — never the raw key.
	if len(claims.Subject) == 0 {
		t.Error("expected non-empty subject")
	}
	if claims.Subject == "sk-admin-123" {
		t.Error("subject must not be the raw API key")
	}
}

func TestAPIKeyValidator_ValidOperatorKey(t *testing.T) {
	v := NewAPIKeyValidator(map[string]string{"sk-op-456": "operator"})

	claims, err := v.Validate(context.Background(), "sk-op-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Role != "operator" {
		t.Errorf("want role operator, got %s", claims.Role)
	}
}

func TestAPIKeyValidator_InvalidKey_ReturnsError(t *testing.T) {
	v := NewAPIKeyValidator(map[string]string{"sk-admin-123": "admin"})

	if _, err := v.Validate(context.Background(), "not-a-valid-key"); err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestAPIKeyValidator_EmptyKey_ReturnsError(t *testing.T) {
	v := NewAPIKeyValidator(map[string]string{"sk-admin-123": "admin"})

	if _, err := v.Validate(context.Background(), ""); err == nil {
		t.Error("expected error for empty key")
	}
}
