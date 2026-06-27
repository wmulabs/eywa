package orchestrator

import (
	"context"
	"testing"
)

func TestInMemoryHandoffStore_SetGetClear(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryHandoffStore()

	if got, _ := store.GetActiveSpirit(ctx, "user:1"); got != "" {
		t.Errorf("expected empty before set, got %q", got)
	}

	if err := store.SetActiveSpirit(ctx, "user:1", "billing"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got, _ := store.GetActiveSpirit(ctx, "user:1"); got != "billing" {
		t.Errorf("expected billing, got %q", got)
	}

	if err := store.ClearActiveSpirit(ctx, "user:1"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got, _ := store.GetActiveSpirit(ctx, "user:1"); got != "" {
		t.Errorf("expected empty after clear, got %q", got)
	}
}

func TestInMemoryHandoffStore_Overwrite(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryHandoffStore()
	_ = store.SetActiveSpirit(ctx, "user:1", "billing")
	_ = store.SetActiveSpirit(ctx, "user:1", "sales")
	if got, _ := store.GetActiveSpirit(ctx, "user:1"); got != "sales" {
		t.Errorf("expected overwrite to sales, got %q", got)
	}
}

func TestInMemoryHandoffStore_KeyCap(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryHandoffStore()
	for i := 0; i < maxInMemoryHandoffKeys; i++ {
		if err := store.SetActiveSpirit(ctx, string(rune(i))+"-k", "s"); err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
	}
	if err := store.SetActiveSpirit(ctx, "overflow", "s"); err == nil {
		t.Error("expected error when exceeding key cap")
	}
	// Updating an existing key must still work at the cap.
	if err := store.SetActiveSpirit(ctx, string(rune(0))+"-k", "s2"); err != nil {
		t.Errorf("updating existing key at cap should succeed: %v", err)
	}
}
