package ports

import (
	"context"
	"time"
)

// Bond is the distributed lock provider (Redlock).
// AcquireLock returns (false, nil) — not (false, error) — when the lock is already held.
// Only return a non-nil error for infrastructure failures (Redis unreachable, etc.).
type Bond interface {
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
	ExtendLock(ctx context.Context, key string, ttl time.Duration) error
}
