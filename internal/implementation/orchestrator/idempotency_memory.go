package orchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

const maxInMemoryIdempotencyKeys = 100_000

// inMemoryIdempotencyStore is an in-process IdempotencyStore backed by a sync.Mutex map.
// Suitable for single-instance deployments, local development, and tests.
// It provides no cross-instance coordination — use redis.NewIdempotencyStore for multi-instance.
type inMemoryIdempotencyStore struct {
	mu   sync.Mutex
	seen map[string]time.Time // key -> expiry
}

// NewInMemoryIdempotencyStore returns an IdempotencyStore that deduplicates within a single process.
// Keys are capped at 100,000 live entries; expired entries are reaped on each call.
// For multi-instance production, use redis.NewIdempotencyStore.
func NewInMemoryIdempotencyStore() ports.IdempotencyStore {
	return &inMemoryIdempotencyStore{seen: make(map[string]time.Time)}
}

func (s *inMemoryIdempotencyStore) Seen(_ context.Context, key string, ttl time.Duration) (bool, error) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.reapExpired(now)

	if expiry, ok := s.seen[key]; ok && now.Before(expiry) {
		return true, nil
	}

	// Cap protects against unbounded growth on adversarial input. When full, fail open
	// (treat as not-seen) rather than block: dropping legitimate events is worse than a rare duplicate.
	if len(s.seen) >= maxInMemoryIdempotencyKeys {
		return false, nil
	}

	s.seen[key] = now.Add(ttl)
	return false, nil
}

func (s *inMemoryIdempotencyStore) reapExpired(now time.Time) {
	for key, expiry := range s.seen {
		if now.After(expiry) {
			delete(s.seen, key)
		}
	}
}
