package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

const idempotencyKeyPrefix = "eywa:idempotency"

// IdempotencyStore implements eywa.IdempotencyStore using Redis SETNX.
// The first Seen call for a key sets it with the given TTL and reports false (not a duplicate);
// subsequent calls within the TTL window find the key present and report true.
type IdempotencyStore struct {
	client *redis.Client
}

// NewIdempotencyStore creates a Redis-backed IdempotencyStore for cross-instance duplicate suppression.
func NewIdempotencyStore(client *redis.Client) eywa.IdempotencyStore {
	return &IdempotencyStore{client: client}
}

// Seen records key via SETNX with ttl and reports whether it was already present.
// SetNX returns true when the key was set (first occurrence), so a duplicate is the inverse.
func (s *IdempotencyStore) Seen(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	set, err := s.client.SetNX(ctx, s.key(key), 1, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("idempotency setnx: %w", err)
	}
	return !set, nil
}

func (s *IdempotencyStore) key(idempotencyKey string) string {
	return fmt.Sprintf("%s:%s", idempotencyKeyPrefix, idempotencyKey)
}
