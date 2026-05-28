package ports

import "context"

// Limiter controls request rate per key (e.g., session key or contact phone).
// Implementations should fail open on infrastructure errors to avoid blocking traffic.
type Limiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}
