package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newBondClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func TestBondManager_AcquireLock_Success(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	acquired, err := bond.AcquireLock(context.Background(), "lock:user:1", time.Second*5)
	if err != nil {
		t.Fatalf("AcquireLock error: %v", err)
	}
	if !acquired {
		t.Error("expected lock acquired")
	}
}

func TestBondManager_AcquireLock_AlreadyHeld_ReturnsFalse(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	_, _ = bond.AcquireLock(context.Background(), "lock:user:2", time.Second*30)
	acquired, _ := bond.AcquireLock(context.Background(), "lock:user:2", time.Second*30)

	// With miniredis + redsync, second acquire must not succeed regardless of error handling.
	if acquired {
		t.Error("expected acquired=false when lock already held")
	}
}

func TestBondManager_ReleaseLock_AfterAcquire(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	_, _ = bond.AcquireLock(context.Background(), "lock:user:3", time.Second*30)
	if err := bond.ReleaseLock(context.Background(), "lock:user:3"); err != nil {
		t.Errorf("ReleaseLock error: %v", err)
	}
}

func TestBondManager_ReleaseLock_NotHeld_ReturnsNil(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	// Release without acquiring — should not error
	if err := bond.ReleaseLock(context.Background(), "lock:ghost"); err != nil {
		t.Errorf("expected nil releasing unheld lock, got: %v", err)
	}
}

func TestBondManager_AcquireLock_AfterRelease_Succeeds(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	_, _ = bond.AcquireLock(context.Background(), "lock:cycle", time.Second*30)
	_ = bond.ReleaseLock(context.Background(), "lock:cycle")

	acquired, err := bond.AcquireLock(context.Background(), "lock:cycle", time.Second*30)
	if err != nil {
		t.Fatalf("AcquireLock after release error: %v", err)
	}
	if !acquired {
		t.Error("expected re-acquire after release to succeed")
	}
}

func TestBondManager_ExtendLock_NotHeld_ReturnsError(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	err := bond.ExtendLock(context.Background(), "lock:missing", time.Second*30)
	if err == nil {
		t.Error("expected error extending non-existent lock")
	}
}

func TestBondManager_ExtendLock_HeldLock_Succeeds(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	acquired, err := bond.AcquireLock(context.Background(), "lock:extend", time.Second*30)
	if err != nil || !acquired {
		t.Skipf("could not acquire lock to test extension: err=%v acquired=%v", err, acquired)
	}

	if err := bond.ExtendLock(context.Background(), "lock:extend", time.Second*60); err != nil {
		t.Errorf("ExtendLock error: %v", err)
	}
}

// Bug #4: a contended acquire must fail fast (false, nil), not retry until the caller's
// timeout — retrying misclassifies contention as a retriable infrastructure error.
func TestBondManager_AcquireLock_Contended_FailsFast(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	if acquired, err := bond.AcquireLock(context.Background(), "lock:ff", 30*time.Second); err != nil || !acquired {
		t.Fatalf("initial acquire failed: err=%v acquired=%v", err, acquired)
	}

	start := time.Now()
	acquired, err := bond.AcquireLock(context.Background(), "lock:ff", 30*time.Second)
	elapsed := time.Since(start)

	if acquired {
		t.Error("expected contended acquire to fail")
	}
	if err != nil {
		t.Errorf("contended acquire must return (false, nil), got err=%v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected fail-fast (<500ms), took %v — redsync retries not disabled", elapsed)
	}
}

// Bug #3: ExtendLock must renew the Redis key to the new TTL, not re-apply the acquire TTL.
func TestBondManager_ExtendLock_AppliesNewTTL(t *testing.T) {
	client, mr := newBondClient(t)
	bond := NewBondManager(client)

	if acquired, err := bond.AcquireLock(context.Background(), "lock:ttl", 10*time.Second); err != nil || !acquired {
		t.Fatalf("acquire failed: err=%v acquired=%v", err, acquired)
	}

	if err := bond.ExtendLock(context.Background(), "lock:ttl", 60*time.Second); err != nil {
		t.Fatalf("extend failed: %v", err)
	}

	// Before the fix, redsync re-applied the original 10s expiry; now it must be ~60s.
	if ttl := mr.TTL("lock:ttl"); ttl <= 10*time.Second {
		t.Errorf("expected TTL renewed to ~60s, got %v", ttl)
	}
}

// After extending, releasing must still work — recreating the mutex preserves ownership.
func TestBondManager_ExtendThenRelease_Succeeds(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	if acquired, err := bond.AcquireLock(context.Background(), "lock:er", 10*time.Second); err != nil || !acquired {
		t.Fatalf("acquire failed: err=%v acquired=%v", err, acquired)
	}
	if err := bond.ExtendLock(context.Background(), "lock:er", 30*time.Second); err != nil {
		t.Fatalf("extend failed: %v", err)
	}
	if err := bond.ReleaseLock(context.Background(), "lock:er"); err != nil {
		t.Errorf("release after extend failed: %v", err)
	}

	// Lock is free again.
	if acquired, _ := bond.AcquireLock(context.Background(), "lock:er", 10*time.Second); !acquired {
		t.Error("expected re-acquire after release to succeed")
	}
}

func TestBondManager_AcquireLock_DifferentKeys_Independent(t *testing.T) {
	client, _ := newBondClient(t)
	bond := NewBondManager(client)

	acqA, _ := bond.AcquireLock(context.Background(), "lock:A", time.Second*30)
	acqB, _ := bond.AcquireLock(context.Background(), "lock:B", time.Second*30)

	if !acqA || !acqB {
		t.Error("expected both different keys acquired independently")
	}
}

func TestBondManager_AcquireLock_ConnectionError_ReturnsError(t *testing.T) {
	client, mr := newBondClient(t)
	bond := NewBondManager(client)

	// Close the miniredis server to force a connection error
	mr.Close()

	acquired, err := bond.AcquireLock(context.Background(), "lock:fail", time.Second*5)
	// Connection errors != ErrFailed, so should return err and false
	if err == nil {
		t.Error("expected error when Redis unreachable")
	}
	if acquired {
		t.Error("expected acquired=false on connection error")
	}
}

func TestBondManager_ReleaseLock_UnlockError_ReturnsError(t *testing.T) {
	client, mr := newBondClient(t)
	bond := NewBondManager(client)

	// Acquire the lock first
	acquired, err := bond.AcquireLock(context.Background(), "lock:err-release", time.Second*30)
	if err != nil || !acquired {
		t.Skipf("could not acquire lock: err=%v acquired=%v", err, acquired)
	}

	// Close miniredis to force unlock to fail
	mr.Close()

	err = bond.ReleaseLock(context.Background(), "lock:err-release")
	if err == nil {
		t.Error("expected error when Redis unreachable during release")
	}
}

func TestBondManager_ExtendLock_ExtendError_ReturnsError(t *testing.T) {
	client, mr := newBondClient(t)
	bond := NewBondManager(client)

	acquired, err := bond.AcquireLock(context.Background(), "lock:extend-fail", time.Second*30)
	if err != nil || !acquired {
		t.Skipf("could not acquire lock: err=%v acquired=%v", err, acquired)
	}

	// Close miniredis to force extend to fail
	mr.Close()

	err = bond.ExtendLock(context.Background(), "lock:extend-fail", time.Second*60)
	if err == nil {
		t.Error("expected error when Redis unreachable during extend")
	}
}
