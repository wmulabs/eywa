package ports

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// VigilRepository manages ephemeral operator seat state in Redis.
// A vigil is created when an operator takes a seat and expires automatically via TTL.
type VigilRepository interface {
	// Acquire creates the vigil for memoryKey. Returns an error if the seat is already held.
	Acquire(ctx context.Context, memoryKey, operatorID string, ttl time.Duration) error
	// Release removes the vigil immediately (operator explicitly releases seat).
	Release(ctx context.Context, memoryKey string) error
	// Get returns the current vigil, or nil if no vigil exists (no error).
	Get(ctx context.Context, memoryKey string) (*entities.Vigil, error)
	// Refresh resets the TTL (sliding window — call on every operator message).
	Refresh(ctx context.Context, memoryKey string, ttl time.Duration) error
	// ListAll returns all active vigil seats across all sessions.
	ListAll(ctx context.Context) ([]*entities.Vigil, error)
}
