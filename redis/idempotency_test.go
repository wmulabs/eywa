package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newIdempotencyClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func TestIdempotencyStore_Seen_FirstCallIsNotDuplicate(t *testing.T) {
	client, _ := newIdempotencyClient(t)
	store := NewIdempotencyStore(client)

	seen, err := store.Seen(context.Background(), "wamid.ABC", time.Minute)
	if err != nil {
		t.Fatalf("Seen error: %v", err)
	}
	if seen {
		t.Error("expected first occurrence to report seen=false")
	}
}

func TestIdempotencyStore_Seen_SecondCallIsDuplicate(t *testing.T) {
	client, _ := newIdempotencyClient(t)
	store := NewIdempotencyStore(client)

	_, _ = store.Seen(context.Background(), "wamid.ABC", time.Minute)
	seen, err := store.Seen(context.Background(), "wamid.ABC", time.Minute)
	if err != nil {
		t.Fatalf("Seen error: %v", err)
	}
	if !seen {
		t.Error("expected second occurrence to report seen=true")
	}
}

func TestIdempotencyStore_Seen_ExpiredKeyIsNotDuplicate(t *testing.T) {
	client, mr := newIdempotencyClient(t)
	store := NewIdempotencyStore(client)

	_, _ = store.Seen(context.Background(), "wamid.ABC", time.Minute)
	mr.FastForward(2 * time.Minute)

	seen, err := store.Seen(context.Background(), "wamid.ABC", time.Minute)
	if err != nil {
		t.Fatalf("Seen error: %v", err)
	}
	if seen {
		t.Error("expected expired key to report seen=false")
	}
}

func TestIdempotencyStore_Seen_DistinctKeysIndependent(t *testing.T) {
	client, _ := newIdempotencyClient(t)
	store := NewIdempotencyStore(client)

	_, _ = store.Seen(context.Background(), "key-a", time.Minute)
	seen, _ := store.Seen(context.Background(), "key-b", time.Minute)
	if seen {
		t.Error("distinct key must not be reported as duplicate")
	}
}
