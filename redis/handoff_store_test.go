package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newHandoffClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func TestHandoffStore_SetGetClear(t *testing.T) {
	ctx := context.Background()
	client, _ := newHandoffClient(t)
	store := NewHandoffStore(client, time.Hour)

	if got, err := store.GetActiveSpirit(ctx, "user:1"); err != nil || got != "" {
		t.Fatalf("expected empty/no-error before set, got %q err=%v", got, err)
	}

	if err := store.SetActiveSpirit(ctx, "user:1", "billing"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := store.GetActiveSpirit(ctx, "user:1")
	if err != nil || got != "billing" {
		t.Fatalf("expected billing, got %q err=%v", got, err)
	}

	if err := store.ClearActiveSpirit(ctx, "user:1"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got, _ := store.GetActiveSpirit(ctx, "user:1"); got != "" {
		t.Errorf("expected empty after clear, got %q", got)
	}
}

func TestHandoffStore_Overwrite(t *testing.T) {
	ctx := context.Background()
	client, _ := newHandoffClient(t)
	store := NewHandoffStore(client, 0)

	_ = store.SetActiveSpirit(ctx, "user:1", "billing")
	_ = store.SetActiveSpirit(ctx, "user:1", "sales")
	if got, _ := store.GetActiveSpirit(ctx, "user:1"); got != "sales" {
		t.Errorf("expected sales after overwrite, got %q", got)
	}
}

func TestHandoffStore_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	client, mr := newHandoffClient(t)
	store := NewHandoffStore(client, time.Minute)

	if err := store.SetActiveSpirit(ctx, "user:1", "billing"); err != nil {
		t.Fatalf("set: %v", err)
	}
	mr.FastForward(2 * time.Minute)
	if got, _ := store.GetActiveSpirit(ctx, "user:1"); got != "" {
		t.Errorf("expected pin to expire, got %q", got)
	}
}
