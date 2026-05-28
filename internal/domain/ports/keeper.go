package ports

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// Keeper schedules events for future execution via an external provider (e.g. Cloud Tasks).
// It is a thin provider boundary — all business logic lives in RitualManager.
type Keeper interface {
	// Schedule enqueues an event to be delivered at executeAt.
	// Pass time.Now() for immediate async dispatch.
	// Returns the provider-specific task ID needed for cancellation.
	Schedule(ctx context.Context, eventKey string, event *entities.Pulse, executeAt time.Time) (taskID string, err error)

	// Cancel removes a pending task. Returns nil if the task no longer exists.
	Cancel(ctx context.Context, taskID string) error
}
