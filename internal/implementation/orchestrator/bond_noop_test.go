package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

var _ ports.Bond = (*noOpBond)(nil)

func TestNoOpBond_AcquireLock_ReturnsTrue(t *testing.T) {
	b := newNoOpBond()
	ctx := context.Background()

	acquired, err := b.AcquireLock(ctx, "key1", time.Second)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected acquired=true for uncontested lock")
	}
}

func TestNoOpBond_AcquireLock_AlreadyHeld_ReturnsFalse(t *testing.T) {
	b := newNoOpBond()
	ctx := context.Background()

	b.AcquireLock(ctx, "key1", time.Second) //nolint:errcheck

	acquired, err := b.AcquireLock(ctx, "key1", time.Second)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if acquired {
		t.Error("expected acquired=false when lock already held")
	}
}

func TestNoOpBond_AcquireLock_DifferentKeys_BothAcquired(t *testing.T) {
	b := newNoOpBond()
	ctx := context.Background()

	ok1, _ := b.AcquireLock(ctx, "key1", time.Second)
	ok2, _ := b.AcquireLock(ctx, "key2", time.Second)

	if !ok1 {
		t.Error("expected key1 acquired")
	}
	if !ok2 {
		t.Error("expected key2 acquired independently from key1")
	}
}

func TestNoOpBond_ReleaseLock_AllowsReacquire(t *testing.T) {
	b := newNoOpBond()
	ctx := context.Background()

	b.AcquireLock(ctx, "key1", time.Second) //nolint:errcheck
	b.ReleaseLock(ctx, "key1")              //nolint:errcheck

	acquired, err := b.AcquireLock(ctx, "key1", time.Second)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected lock re-acquirable after release")
	}
}

func TestNoOpBond_ReleaseLock_NonExistentKey_ReturnsNil(t *testing.T) {
	b := newNoOpBond()
	ctx := context.Background()

	err := b.ReleaseLock(ctx, "nonexistent")

	if err != nil {
		t.Errorf("expected nil error for non-existent key, got: %v", err)
	}
}

func TestNoOpBond_ExtendLock_ReturnsNil(t *testing.T) {
	b := newNoOpBond()
	ctx := context.Background()

	err := b.ExtendLock(ctx, "key1", time.Second)

	if err != nil {
		t.Errorf("expected nil error from no-op extend, got: %v", err)
	}
}
