package orchestrator

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// SpiritSelector resolves which Spirit should handle a given Pulse.
// Weave implements this interface via its selectSpirit method.
type SpiritSelector interface {
	selectSpirit(ctx context.Context, event *entities.Pulse, config *entities.Link) string
}

// InteractionLogger persists a completed interaction to the audit log.
// Weave implements this interface via its logInteraction method.
type InteractionLogger interface {
	logInteraction(ctx context.Context, state *ProcessingState) error
}

// ReasoningExecutor drives the multi-turn LLM loop for a single Pulse.
// ReasoningService implements this interface.
type ReasoningExecutor interface {
	Execute(ctx context.Context, req *ReasoningRequest) (*ReasoningResult, error)
	ExecuteStream(ctx context.Context, req *ReasoningRequest) (<-chan ReasoningEvent, error)
}

// EventValidatorIface validates an incoming Pulse before pipeline execution.
// EventValidator implements this interface.
type EventValidatorIface interface {
	Validate(ctx context.Context, event *entities.Pulse) error
}
