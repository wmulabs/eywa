package errors

import (
	stdErrors "errors"
	"testing"
)

func TestNotFoundError_Error(t *testing.T) {
	err := &NotFoundError{Entity: "spirit", ID: "mystic"}
	expected := `spirit "mystic" not found`
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestNotFoundError_IsMatchesErrNotFound(t *testing.T) {
	err := &NotFoundError{Entity: "spirit", ID: "mystic"}
	if !stdErrors.Is(err, ErrNotFound) {
		t.Error("errors.Is(err, ErrNotFound) must return true")
	}
}

func TestNotFoundError_IsDoesNotMatchErrConflict(t *testing.T) {
	err := &NotFoundError{Entity: "spirit", ID: "mystic"}
	if stdErrors.Is(err, ErrConflict) {
		t.Error("errors.Is(err, ErrConflict) must return false")
	}
}

func TestConflictError_Error(t *testing.T) {
	err := &ConflictError{Entity: "spirit", ID: "mystic"}
	expected := `spirit "mystic" already exists`
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestConflictError_IsMatchesErrConflict(t *testing.T) {
	err := &ConflictError{Entity: "spirit", ID: "mystic"}
	if !stdErrors.Is(err, ErrConflict) {
		t.Error("errors.Is(err, ErrConflict) must return true")
	}
}

func TestActionError_ErrorWithCause(t *testing.T) {
	cause := stdErrors.New("db timeout")
	err := &ActionError{
		Type:    BusinessError,
		Message: "order not found",
		Err:     cause,
	}
	expected := "business error: order not found (cause: db timeout)"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestActionError_ErrorWithoutCause(t *testing.T) {
	err := &ActionError{
		Type:    InfrastructureError,
		Message: "redis down",
	}
	expected := "infrastructure error: redis down"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestActionError_UnwrapExposesCauseToIs(t *testing.T) {
	cause := stdErrors.New("network timeout")
	err := &ActionError{
		Type:    InfrastructureError,
		Message: "api call failed",
		Err:     cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() must return the cause")
	}

	if !stdErrors.Is(err, cause) {
		t.Error("errors.Is(err, cause) must return true after Unwrap")
	}
}

func TestIsBusinessError_ReturnsTrueForBusinessError(t *testing.T) {
	err := NewBusinessError("order not found")
	if !IsBusinessError(err) {
		t.Error("IsBusinessError must return true for BusinessError")
	}
}

func TestIsBusinessError_ReturnsFalseForInfrastructureError(t *testing.T) {
	err := NewInfrastructureError("redis down")
	if IsBusinessError(err) {
		t.Error("IsBusinessError must return false for InfrastructureError")
	}
}

func TestIsBusinessError_ReturnsFalseForNonActionError(t *testing.T) {
	err := stdErrors.New("plain error")
	if IsBusinessError(err) {
		t.Error("IsBusinessError must return false for non-ActionError")
	}
}

func TestIsInfrastructureError_ReturnsTrueForInfrastructureError(t *testing.T) {
	err := NewInfrastructureError("redis down")
	if !IsInfrastructureError(err) {
		t.Error("IsInfrastructureError must return true for InfrastructureError")
	}
}

func TestIsInfrastructureError_ReturnsFalseForBusinessError(t *testing.T) {
	err := NewBusinessError("order not found")
	if IsInfrastructureError(err) {
		t.Error("IsInfrastructureError must return false for BusinessError")
	}
}

func TestIsInfrastructureError_ReturnsFalseForNonActionError(t *testing.T) {
	err := stdErrors.New("plain error")
	if IsInfrastructureError(err) {
		t.Error("IsInfrastructureError must return false for non-ActionError")
	}
}

func TestMessageFor_ReturnsMessageForActionError(t *testing.T) {
	err := NewBusinessError("order not found")
	if msg := MessageFor(err); msg != "order not found" {
		t.Errorf("got %q, want %q", msg, "order not found")
	}
}

func TestMessageFor_ReturnsFallbackForPlainError(t *testing.T) {
	err := stdErrors.New("plain error text")
	if msg := MessageFor(err); msg != "plain error text" {
		t.Errorf("got %q, want %q", msg, "plain error text")
	}
}

func TestNewBusinessError_WithoutCause(t *testing.T) {
	err := NewBusinessError("order not found")
	if err.Type != BusinessError {
		t.Errorf("Type must be BusinessError")
	}
	if err.Message != "order not found" {
		t.Errorf("Message must be set")
	}
	if err.Err != nil {
		t.Errorf("Err must be nil")
	}
}

func TestNewBusinessError_WithCause(t *testing.T) {
	cause := stdErrors.New("db timeout")
	err := NewBusinessError("order not found", cause)
	if err.Type != BusinessError {
		t.Errorf("Type must be BusinessError")
	}
	if err.Message != "order not found" {
		t.Errorf("Message must be set")
	}
	if err.Err != cause {
		t.Errorf("Err must be the cause")
	}
	if !stdErrors.Is(err, cause) {
		t.Error("errors.Is(err, cause) must return true")
	}
}

func TestNewBusinessError_WithMultipleCauses_UseFirst(t *testing.T) {
	cause1 := stdErrors.New("first")
	cause2 := stdErrors.New("second")
	err := NewBusinessError("msg", cause1, cause2)
	if err.Err != cause1 {
		t.Errorf("must use first cause")
	}
}

func TestNewInfrastructureError_WithoutCause(t *testing.T) {
	err := NewInfrastructureError("redis down")
	if err.Type != InfrastructureError {
		t.Errorf("Type must be InfrastructureError")
	}
	if err.Message != "redis down" {
		t.Errorf("Message must be set")
	}
	if err.Err != nil {
		t.Errorf("Err must be nil")
	}
}

func TestNewInfrastructureError_WithCause(t *testing.T) {
	cause := stdErrors.New("connection refused")
	err := NewInfrastructureError("redis down", cause)
	if err.Type != InfrastructureError {
		t.Errorf("Type must be InfrastructureError")
	}
	if err.Message != "redis down" {
		t.Errorf("Message must be set")
	}
	if err.Err != cause {
		t.Errorf("Err must be the cause")
	}
	if !stdErrors.Is(err, cause) {
		t.Error("errors.Is(err, cause) must return true")
	}
}
