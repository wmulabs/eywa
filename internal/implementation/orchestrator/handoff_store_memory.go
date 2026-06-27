package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

const maxInMemoryHandoffKeys = 10_000

// inMemoryHandoffStore is an in-process HandoffStore backed by a sync.Mutex map.
// Suitable for single-instance deployments and tests. It provides no cross-instance coordination —
// use redis.NewHandoffStore or mongo.NewHandoffStore for multi-instance deployments.
type inMemoryHandoffStore struct {
	mu     sync.RWMutex
	active map[string]string
}

// NewInMemoryHandoffStore returns a HandoffStore that keeps active-Spirit pins in process memory.
// Keys are capped at 10,000; exceeding this returns an error. For multi-instance production use the
// redis or mongo HandoffStore adapters.
func NewInMemoryHandoffStore() ports.HandoffStore {
	return &inMemoryHandoffStore{active: make(map[string]string)}
}

func (s *inMemoryHandoffStore) GetActiveSpirit(_ context.Context, sessionKey string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active[sessionKey], nil
}

func (s *inMemoryHandoffStore) SetActiveSpirit(_ context.Context, sessionKey, spiritName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.active[sessionKey]; !exists && len(s.active) >= maxInMemoryHandoffKeys {
		return fmt.Errorf("inMemoryHandoffStore: key limit of %d reached; use redis.NewHandoffStore in production", maxInMemoryHandoffKeys)
	}

	s.active[sessionKey] = spiritName
	return nil
}

func (s *inMemoryHandoffStore) ClearActiveSpirit(_ context.Context, sessionKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, sessionKey)
	return nil
}
