package orchestrator

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestInMemoryIdempotencyStore_FirstThenDuplicate(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	ctx := context.Background()

	seen, err := store.Seen(ctx, "k1", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen {
		t.Error("first occurrence must report seen=false")
	}

	seen, _ = store.Seen(ctx, "k1", time.Minute)
	if !seen {
		t.Error("second occurrence must report seen=true")
	}
}

func TestInMemoryIdempotencyStore_ExpiredKey(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	ctx := context.Background()

	if _, err := store.Seen(ctx, "k1", time.Millisecond); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	seen, _ := store.Seen(ctx, "k1", time.Minute)
	if seen {
		t.Error("expired key must report seen=false")
	}
}

func TestInMemoryIdempotencyStore_DistinctKeys(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	ctx := context.Background()

	_, _ = store.Seen(ctx, "a", time.Minute)
	seen, _ := store.Seen(ctx, "b", time.Minute)
	if seen {
		t.Error("distinct keys must be independent")
	}
}

func TestInMemoryIdempotencyStore_AtCap_FailsOpen(t *testing.T) {
	s := &inMemoryIdempotencyStore{seen: make(map[string]time.Time, maxInMemoryIdempotencyKeys)}
	future := time.Now().Add(time.Hour)
	for i := 0; i < maxInMemoryIdempotencyKeys; i++ {
		s.seen[strconv.Itoa(i)] = future
	}

	seen, err := s.Seen(context.Background(), "overflow", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen {
		t.Error("at cap a new key must fail open (seen=false)")
	}
	if _, stored := s.seen["overflow"]; stored {
		t.Error("at cap the new key must not be stored")
	}
}
