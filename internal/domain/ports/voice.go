package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// Voice is the channel through which the Spirit speaks back to the user.
type Voice interface {
	GetName() string

	// ShouldAutoRespond controls whether the orchestrator calls SendResponse automatically
	// when no ActionDelivery Action ran during the reasoning cycle.
	// WhatsApp: true. HTTP webhook: false (response lives in the HTTP body). Email: true.
	ShouldAutoRespond() bool

	SendResponse(ctx context.Context, event *entities.Pulse, response string) error
	GetChannelMetadata(event *entities.Pulse) map[string]any
}

type VoiceRegistry interface {
	Register(channel Voice) error
	// RegisterMultiple returns an error on the first duplicate found.
	RegisterMultiple(channels ...Voice) error
	Get(name string) (Voice, error)
	Has(name string) bool
	List() []Voice
}
