package redis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

var (
	// ErrBondLockNotFound is returned when ReleaseLock or ExtendLock is called
	// for a key that has no active lock (e.g., the lock already expired or was released).
	ErrBondLockNotFound = errors.New("bond: lock not found")

	// ErrBondExtendFailed is returned when extending an active lock fails.
	ErrBondExtendFailed = errors.New("bond: failed to extend lock")
)

type mutexEntry struct {
	mutex     *redsync.Mutex
	expiresAt time.Time
}

// Mutexes are stored by key so ReleaseLock and ExtendLock can target the same instance.
// The global mu protects the entries map only — it is never held during Redis I/O.
type BondManager struct {
	rs      *redsync.Redsync
	mu      sync.Mutex
	entries map[string]*mutexEntry
}

func NewBondManager(client *redis.Client) eywa.Bond {
	pool := goredis.NewPool(client)
	rs := redsync.New(pool)

	return &BondManager{
		rs:      rs,
		entries: make(map[string]*mutexEntry),
	}
}

// reapExpired removes entries whose TTL has elapsed. Must be called with mu held.
func (r *BondManager) reapExpired() {
	now := time.Now()
	for key, entry := range r.entries {
		if now.After(entry.expiresAt) {
			delete(r.entries, key)
		}
	}
}

// The lock expires automatically; call ReleaseLock to release it early.
func (r *BondManager) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	log := newLogger()

	r.mu.Lock()
	r.reapExpired()
	r.mu.Unlock()

	mutex := r.rs.NewMutex(key, redsync.WithExpiry(ttl))

	// Redis I/O happens outside the map lock.
	err := mutex.LockContext(ctx)
	if err != nil {
		if errors.Is(err, redsync.ErrFailed) {
			log.Debugw("lock already held by another instance", "key", key)
			return false, nil
		}
		log.Warnw("infrastructure error acquiring lock", "key", key, "error", err)
		return false, fmt.Errorf("acquire bond lock: %w", err)
	}

	r.mu.Lock()
	r.entries[key] = &mutexEntry{mutex: mutex, expiresAt: time.Now().Add(ttl)}
	r.mu.Unlock()

	log.Infow("lock acquired", "key", key, "ttl", ttl)
	return true, nil
}

// ReleaseLock uses the stored mutex instance for a clean unlock.
// If the lock has already expired or was never acquired, returns nil.
func (r *BondManager) ReleaseLock(ctx context.Context, key string) error {
	log := newLogger()

	r.mu.Lock()
	entry, exists := r.entries[key]
	if exists {
		delete(r.entries, key)
	}
	r.mu.Unlock()

	if !exists {
		log.Warnw("lock was not held or already expired", "key", key)
		return nil
	}

	// Redis I/O happens outside the map lock.
	ok, err := entry.mutex.UnlockContext(ctx)
	if err != nil {
		log.Errorw("failed to release lock", "key", key, "error", err)
		return fmt.Errorf("release bond lock: %w", err)
	}

	if !ok {
		log.Warnw("lock was not held or already expired", "key", key)
	}

	log.Infow("lock released", "key", key)
	return nil
}

func (r *BondManager) ExtendLock(ctx context.Context, key string, ttl time.Duration) error {
	log := newLogger()

	r.mu.Lock()
	entry, exists := r.entries[key]
	if exists {
		entry.expiresAt = time.Now().Add(ttl)
	}
	r.mu.Unlock()

	if !exists {
		log.Warnw("lock not found for extension", "key", key)
		return fmt.Errorf("%w: %s", ErrBondLockNotFound, key)
	}

	// Redis I/O happens outside the map lock.
	ok, err := entry.mutex.ExtendContext(ctx)
	if err != nil {
		log.Errorw("failed to extend lock", "key", key, "error", err)
		return fmt.Errorf("%w: %w", ErrBondExtendFailed, err)
	}

	if !ok {
		log.Warnw("lock extension failed - lock may not exist or is not held by us", "key", key)
		return fmt.Errorf("%w: %s", ErrBondExtendFailed, key)
	}

	log.Infow("lock extended", "key", key, "ttl", ttl)
	return nil
}
