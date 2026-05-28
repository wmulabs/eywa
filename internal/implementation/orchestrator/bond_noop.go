package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

const maxNoOpBondKeys = 10_000

// noOpBond is an in-process Bond implementation backed by a plain sync.Mutex map.
// Suitable for single-instance deployments and local development.
// It provides no distributed coordination — use redis.NewBondManager for multi-instance.
type noOpBond struct {
	mu    sync.Mutex
	locks map[string]bool
}

// NewNoOpBond returns a Bond that uses in-process mutexes.
// Suitable for single-instance deployments and tests.
// Locks are capped at 10,000 concurrent keys; exceeding this returns an error.
// For multi-instance production, use redis.NewBondManager.
func NewNoOpBond() ports.Bond {
	return newNoOpBond()
}

func newNoOpBond() *noOpBond {
	return &noOpBond{
		locks: make(map[string]bool),
	}
}

func (b *noOpBond) AcquireLock(_ context.Context, key string, _ time.Duration) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.locks[key] {
		return false, nil
	}

	if len(b.locks) >= maxNoOpBondKeys {
		return false, fmt.Errorf("noOpBond: key limit of %d reached — possible lock leak; use redis.NewBondManager in production", maxNoOpBondKeys)
	}

	b.locks[key] = true
	return true, nil
}

func (b *noOpBond) ReleaseLock(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.locks, key)
	return nil
}

// ExtendLock is a no-op for the in-process Bond. Locks do not expire.
func (b *noOpBond) ExtendLock(_ context.Context, _ string, _ time.Duration) error {
	return nil
}
