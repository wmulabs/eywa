package eywa

import (
	"github.com/wmulabs/eywa/internal/domain/errors"
	orchestrator "github.com/wmulabs/eywa/internal/implementation/orchestrator"
)

type (
	// ActionError is the error type returned by Action.Execute. It carries an
	// ActionErrorType that distinguishes user-facing business errors from infra failures.
	ActionError = errors.ActionError

	// ActionErrorType classifies an ActionError as BusinessError or InfrastructureError.
	ActionErrorType = errors.ActionErrorType

	// NotFoundError indicates a requested resource does not exist.
	NotFoundError = errors.NotFoundError

	// ConflictError indicates a uniqueness constraint violation (duplicate resource).
	ConflictError = errors.ConflictError
)

// Action error type constants for ActionError.Type.
const (
	// BusinessError marks an error that should be surfaced to the user (e.g. "order not found").
	BusinessError = errors.BusinessError

	// InfrastructureError marks a transient failure the user should not see (e.g. DB timeout).
	InfrastructureError = errors.InfrastructureError
)

var (
	// NewBusinessError creates an ActionError with type BusinessError. The Oracle will
	// relay msg to the user as part of its reasoning context.
	NewBusinessError = errors.NewBusinessError

	// NewInfrastructureError creates an ActionError with type InfrastructureError.
	// The Oracle receives a generic failure message; the original err is logged.
	NewInfrastructureError = errors.NewInfrastructureError

	// IsBusinessError reports whether err is an ActionError with type BusinessError.
	IsBusinessError = errors.IsBusinessError

	// IsInfrastructureError reports whether err is an ActionError with type InfrastructureError.
	IsInfrastructureError = errors.IsInfrastructureError

	// MessageFor returns the user-facing message for err if it is a BusinessError,
	// or a generic fallback message for infrastructure errors.
	MessageFor = errors.MessageFor
)

var (
	// ErrNotFound is returned by repositories when a requested resource does not exist.
	ErrNotFound = errors.ErrNotFound

	// ErrConflict is returned by repositories when a resource with the same key already exists.
	ErrConflict = errors.ErrConflict

	// ErrRitualTerminal is returned when an operation is attempted on a Ritual that has
	// already reached a terminal state (executed, cancelled, or failed).
	ErrRitualTerminal = errors.ErrRitualTerminal

	// ErrStructuredOutput is returned when a model response cannot be coerced into the requested
	// schema after the bounded repair attempt. Matchable with errors.Is.
	ErrStructuredOutput = errors.ErrStructuredOutput

	// ErrSessionHeld is returned by Weave.ProcessEventByKey when an active Vigil seat
	// is held for the MemoryKey. The operator is handling the conversation directly.
	ErrSessionHeld = orchestrator.ErrSessionHeld

	// IsSessionHeld reports whether err is ErrSessionHeld.
	IsSessionHeld = orchestrator.IsSessionHeld
)
