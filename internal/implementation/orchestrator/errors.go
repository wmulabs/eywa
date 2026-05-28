package orchestrator

import (
	"errors"
	"fmt"
)

type OrchestrationError struct {
	Code      string
	Message   string
	Err       error
	Retriable bool // true = task delivery should retry; false = error is permanent, no retry needed
}

func (e *OrchestrationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *OrchestrationError) Unwrap() error {
	return e.Err
}

// IsRetriable reports whether err is a retriable orchestration error.
// Retriable errors should be retried by the caller (e.g. return non-2xx to trigger Keeper redelivery).
func IsRetriable(err error) bool {
	var oe *OrchestrationError
	if ok := errors.As(err, &oe); ok {
		return oe.Retriable
	}
	return false
}

func ErrInvalidConfig(message string) error {
	return &OrchestrationError{Code: "INVALID_CONFIG", Message: message}
}

func ErrValidationFailed(field string, reason string) error {
	return &OrchestrationError{
		Code:    "VALIDATION_FAILED",
		Message: fmt.Sprintf("validation failed for field '%s': %s", field, reason),
	}
}

func ErrLockAcquisitionFailed(memoryKey string, err error) error {
	return &OrchestrationError{
		Code:      "LOCK_ACQUISITION_FAILED",
		Message:   fmt.Sprintf("failed to acquire lock for memory '%s'", memoryKey),
		Err:       err,
		Retriable: true, // infrastructure failure — transient, should retry
	}
}

// ErrMemoryBusy is returned when the lock is held by another in-flight cycle.
// The message has already been pushed to the inbox and will be coalesced into the
// next processing cycle — no retry needed, acknowledge the delivery.
func ErrMemoryBusy(memoryKey string) error {
	return &OrchestrationError{
		Code:      "MEMORY_BUSY",
		Message:   fmt.Sprintf("memory '%s' is already being processed", memoryKey),
		Retriable: false,
	}
}

func IsMemoryBusy(err error) bool {
	var oe *OrchestrationError
	return errors.As(err, &oe) && oe.Code == "MEMORY_BUSY"
}

func ErrScoutFailed(err error) error {
	return &OrchestrationError{
		Code:    "SCOUT_FAILED",
		Message: "scout harvest failed",
		Err:     err,
	}
}

func ErrSpiritNotFound(spiritName string, err error) error {
	return &OrchestrationError{
		Code:    "SPIRIT_NOT_FOUND",
		Message: fmt.Sprintf("spirit '%s' not found", spiritName),
		Err:     err,
	}
}

func ErrMemoryOperationFailed(operation string, err error) error {
	return &OrchestrationError{
		Code:      "MEMORY_OPERATION_FAILED",
		Message:   fmt.Sprintf("memory operation '%s' failed", operation),
		Err:       err,
		Retriable: true, // Redis outage, nothing sent to user yet
	}
}

func ErrReasoningFailed(err error) error {
	return &OrchestrationError{
		Code:      "REASONING_FAILED",
		Message:   "reasoning loop failed",
		Err:       err,
		Retriable: true, // LLM provider may recover; ProcessEvent checks ResponseDelivered first
	}
}

func ErrPersistenceFailed(operation string, err error) error {
	return &OrchestrationError{
		Code:    "PERSISTENCE_FAILED",
		Message: fmt.Sprintf("persistence operation '%s' failed", operation),
		Err:     err,
	}
}

func ErrTimeoutExceeded(operation string) error {
	return &OrchestrationError{
		Code:      "TIMEOUT_EXCEEDED",
		Message:   fmt.Sprintf("operation '%s' exceeded timeout", operation),
		Retriable: true,
	}
}

// Non-retriable: acknowledge and drop — retrying won't help.
func ErrRateLimitExceeded(memoryKey string) error {
	return &OrchestrationError{
		Code:      "RATE_LIMIT_EXCEEDED",
		Message:   fmt.Sprintf("rate limit exceeded for memory '%s'", memoryKey),
		Retriable: false,
	}
}

func IsRateLimited(err error) bool {
	var oe *OrchestrationError
	return errors.As(err, &oe) && oe.Code == "RATE_LIMIT_EXCEEDED"
}

// Non-retriable — the cycle is terminated intentionally.
func ErrToolBudgetExceeded(limit int) error {
	return &OrchestrationError{
		Code:      "TOOL_BUDGET_EXCEEDED",
		Message:   fmt.Sprintf("tool call budget of %d exceeded in this reasoning cycle", limit),
		Retriable: false,
	}
}

func ErrPromptInjectionDetected() error {
	return &OrchestrationError{
		Code:      "PROMPT_INJECTION_DETECTED",
		Message:   "user message contains disallowed content",
		Retriable: false,
	}
}

// Non-retriable — the value is explicitly denied.
func ErrFilterBlocked(field, value string) error {
	return &OrchestrationError{
		Code:      "FILTER_BLOCKED",
		Message:   fmt.Sprintf("event field '%s' value '%s' is blocked", field, value),
		Retriable: false,
	}
}

func IsFilterBlocked(err error) bool {
	var oe *OrchestrationError
	return errors.As(err, &oe) && oe.Code == "FILTER_BLOCKED"
}

// Non-retriable — the value is not authorized.
func ErrFilterNotAllowed(field, value string) error {
	return &OrchestrationError{
		Code:      "FILTER_NOT_ALLOWED",
		Message:   fmt.Sprintf("event field '%s' value '%s' is not in the allowlist", field, value),
		Retriable: false,
	}
}

func IsFilterNotAllowed(err error) bool {
	var oe *OrchestrationError
	return errors.As(err, &oe) && oe.Code == "FILTER_NOT_ALLOWED"
}

// ErrSessionHeld is the public sentinel for the SESSION_HELD condition.
// Non-retriable — the message is already in the inbox and will drain when the seat is released.
// Use IsSessionHeld for error matching; do not compare by identity (== ErrSessionHeld).
// Internal code returns fresh instances via newErrSessionHeld to avoid shared mutable state.
var ErrSessionHeld = &OrchestrationError{
	Code:      "SESSION_HELD",
	Message:   "session held by operator vigil",
	Retriable: false,
}

// newErrSessionHeld returns a fresh SESSION_HELD error. Used internally to avoid
// returning the shared ErrSessionHeld sentinel, which is mutable.
func newErrSessionHeld() error {
	return &OrchestrationError{
		Code:      "SESSION_HELD",
		Message:   "session held by operator vigil",
		Retriable: false,
	}
}

func IsSessionHeld(err error) bool {
	var oe *OrchestrationError
	return errors.As(err, &oe) && oe.Code == "SESSION_HELD"
}
