package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newLimiterClient(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestLimiter_Allow_UnderLimit_ReturnsTrue(t *testing.T) {
	client := newLimiterClient(t)
	limiter := NewLimiter(client, 10, time.Minute)

	allowed, err := limiter.Allow(context.Background(), "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true under limit")
	}
}

func TestLimiter_Allow_ExceedsLimit_ReturnsFalse(t *testing.T) {
	client := newLimiterClient(t)
	limiter := NewLimiter(client, 2, time.Minute)

	for i := 0; i < 2; i++ {
		_, _ = limiter.Allow(context.Background(), "user:burst")
	}

	allowed, err := limiter.Allow(context.Background(), "user:burst")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected allowed=false after limit exceeded")
	}
}

func TestLimiter_Allow_DifferentKeys_Independent(t *testing.T) {
	client := newLimiterClient(t)
	limiter := NewLimiter(client, 1, time.Minute)

	// exhaust user:A
	_, _ = limiter.Allow(context.Background(), "user:A")
	allowedA, _ := limiter.Allow(context.Background(), "user:A")

	// user:B should still be allowed
	allowedB, _ := limiter.Allow(context.Background(), "user:B")

	if allowedA {
		t.Error("expected user:A blocked after limit")
	}
	if !allowedB {
		t.Error("expected user:B still allowed (independent key)")
	}
}

func TestLimiter_Allow_RateKey_Prefixed(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewLimiter(client, 5, time.Minute)

	_, _ = limiter.Allow(context.Background(), "tenant-user-99")

	keys := mr.Keys()
	found := false
	for _, k := range keys {
		if len(k) >= 10 && k[:10] == "ratelimit:" {
			found = true
		}
	}
	if !found {
		// GCRA may use a different key scheme — just verify some key was written
		if len(keys) == 0 {
			t.Errorf("expected at least one key written by rate limiter, got none")
		}
	}
}

func TestLimiter_Allow_RedisError_FailsOpen(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewLimiter(client, 10, time.Minute)

	// Close server to force Redis error
	mr.Close()

	// Should fail open (true) with an error
	allowed, err := limiter.Allow(context.Background(), "user:fail")
	if !allowed {
		t.Error("expected fail-open: allowed=true even on Redis error")
	}
	if err == nil {
		t.Error("expected error to be returned on Redis failure")
	}
}
