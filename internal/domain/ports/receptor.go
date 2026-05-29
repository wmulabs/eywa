package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// Receptor transforms a raw channel payload into normalized Pulses.
// Returns a slice because a single webhook may carry messages from multiple senders.
// Receptors are registered on the Engine via RegisterReceptor().
type Receptor interface {
	GetName() string
	Convert(ctx context.Context, eventType string, rawPayload map[string]any) ([]*entities.Pulse, error)
}
