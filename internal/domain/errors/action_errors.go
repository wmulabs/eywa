package errors

import (
	"errors"
	"fmt"
)

type ActionErrorType string

const (
	// BusinessError is surfaced to the Oracle so the Spirit can reason about it.
	// Examples: invalid phone number, order not found, insufficient balance.
	BusinessError ActionErrorType = "business"

	// InfrastructureError is suppressed from the Oracle — it signals a technical failure
	// the model cannot resolve. Examples: API token expired, network timeout, DB unreachable.
	InfrastructureError ActionErrorType = "infrastructure"
)

type ActionError struct {
	Type    ActionErrorType
	Message string
	Err     error
}

func (e *ActionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s error: %s (cause: %v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

func (e *ActionError) Unwrap() error {
	return e.Err
}

func IsBusinessError(err error) bool {
	var toolErr *ActionError
	return errors.As(err, &toolErr) && toolErr.Type == BusinessError
}

func IsInfrastructureError(err error) bool {
	var toolErr *ActionError
	return errors.As(err, &toolErr) && toolErr.Type == InfrastructureError
}

// MessageFor returns the Message field of the innermost ActionError in the chain,
// falling back to err.Error() when no ActionError is present.
func MessageFor(err error) string {
	var toolErr *ActionError
	if errors.As(err, &toolErr) {
		return toolErr.Message
	}
	return err.Error()
}

func NewBusinessError(message string, cause ...error) *ActionError {
	return &ActionError{Type: BusinessError, Message: message, Err: first(cause)}
}

func NewInfrastructureError(message string, cause ...error) *ActionError {
	return &ActionError{Type: InfrastructureError, Message: message, Err: first(cause)}
}

func first(errs []error) error {
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
