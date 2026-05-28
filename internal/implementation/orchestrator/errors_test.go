package orchestrator

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrSessionHeld_IsNonRetriable(t *testing.T) {
	if IsRetriable(ErrSessionHeld) {
		t.Error("ErrSessionHeld must be non-retriable")
	}
}

func TestIsSessionHeld_MatchesCode(t *testing.T) {
	if !IsSessionHeld(ErrSessionHeld) {
		t.Error("IsSessionHeld must return true for ErrSessionHeld")
	}
}

func TestIsSessionHeld_RejectsOtherErrors(t *testing.T) {
	if IsSessionHeld(ErrMemoryBusy("user-123")) {
		t.Error("IsSessionHeld must return false for ErrMemoryBusy")
	}
}

func TestIsRetriable_RetriableError_ReturnsTrue(t *testing.T) {
	err := ErrLockAcquisitionFailed("user:123", fmt.Errorf("redis timeout"))
	if !IsRetriable(err) {
		t.Error("ErrLockAcquisitionFailed must be retriable")
	}
}

func TestIsRetriable_NonOrchestrationError_ReturnsFalse(t *testing.T) {
	err := errors.New("plain error")
	if IsRetriable(err) {
		t.Error("plain error must not be retriable")
	}
}

func TestOrchestrationError_Error_WithInnerErr(t *testing.T) {
	inner := errors.New("db down")
	err := ErrLockAcquisitionFailed("user:123", inner)
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
	if !strings.Contains(msg, "LOCK_ACQUISITION_FAILED") {
		t.Errorf("expected code in error message, got: %s", msg)
	}
}

func TestOrchestrationError_Error_WithoutInnerErr(t *testing.T) {
	err := ErrRateLimitExceeded("user:123")
	msg := err.Error()
	if !strings.Contains(msg, "RATE_LIMIT_EXCEEDED") {
		t.Errorf("expected code in error message, got: %s", msg)
	}
}

func TestOrchestrationError_Unwrap_ReturnsInnerErr(t *testing.T) {
	inner := errors.New("cause")
	err := ErrLockAcquisitionFailed("key", inner)
	var oe *OrchestrationError
	if !errors.As(err, &oe) {
		t.Fatal("expected OrchestrationError")
	}
	if !errors.Is(err, inner) {
		t.Error("expected Unwrap to expose inner error")
	}
}

func TestIsMemoryBusy_MatchesCode(t *testing.T) {
	if !IsMemoryBusy(ErrMemoryBusy("user:123")) {
		t.Error("IsMemoryBusy must return true for ErrMemoryBusy")
	}
}

func TestIsMemoryBusy_RejectsOtherErrors(t *testing.T) {
	if IsMemoryBusy(ErrSessionHeld) {
		t.Error("IsMemoryBusy must return false for ErrSessionHeld")
	}
}

func TestIsRateLimited_MatchesCode(t *testing.T) {
	if !IsRateLimited(ErrRateLimitExceeded("user:123")) {
		t.Error("IsRateLimited must return true for ErrRateLimitExceeded")
	}
}

func TestIsRateLimited_RejectsOtherErrors(t *testing.T) {
	if IsRateLimited(ErrSessionHeld) {
		t.Error("IsRateLimited must return false for ErrSessionHeld")
	}
}

func TestErrInvalidConfig_HasCode(t *testing.T) {
	err := ErrInvalidConfig("bad timeout value")
	var oe *OrchestrationError
	if !errors.As(err, &oe) {
		t.Fatal("expected OrchestrationError")
	}
	if oe.Code != "INVALID_CONFIG" {
		t.Errorf("expected code INVALID_CONFIG, got %s", oe.Code)
	}
}

func TestErrScoutFailed_HasCode(t *testing.T) {
	err := ErrScoutFailed(errors.New("timeout"))
	var oe *OrchestrationError
	if !errors.As(err, &oe) {
		t.Fatal("expected OrchestrationError")
	}
	if oe.Code != "SCOUT_FAILED" {
		t.Errorf("expected code SCOUT_FAILED, got %s", oe.Code)
	}
}

func TestErrMemoryOperationFailed_IsRetriable(t *testing.T) {
	err := ErrMemoryOperationFailed("get", errors.New("redis down"))
	if !IsRetriable(err) {
		t.Error("ErrMemoryOperationFailed must be retriable")
	}
}

func TestErrReasoningFailed_IsRetriable(t *testing.T) {
	err := ErrReasoningFailed(errors.New("llm timeout"))
	if !IsRetriable(err) {
		t.Error("ErrReasoningFailed must be retriable")
	}
}

func TestErrPersistenceFailed_HasCode(t *testing.T) {
	err := ErrPersistenceFailed("insert_echo", errors.New("disk full"))
	var oe *OrchestrationError
	if !errors.As(err, &oe) {
		t.Fatal("expected OrchestrationError")
	}
	if oe.Code != "PERSISTENCE_FAILED" {
		t.Errorf("expected code PERSISTENCE_FAILED, got %s", oe.Code)
	}
}

func TestErrTimeoutExceeded_IsRetriable(t *testing.T) {
	err := ErrTimeoutExceeded("scout_harvest")
	if !IsRetriable(err) {
		t.Error("ErrTimeoutExceeded must be retriable")
	}
}

func TestErrToolBudgetExceeded_IsNonRetriable(t *testing.T) {
	err := ErrToolBudgetExceeded(10)
	if IsRetriable(err) {
		t.Error("ErrToolBudgetExceeded must be non-retriable")
	}
}
