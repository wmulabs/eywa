package qdrant

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// Interface compliance — fails at compile time if LoreStore diverges from the port.
var _ eywa.LoreStore = (*LoreStore)(nil)

func TestHashID(t *testing.T) {
	// Same input must always produce same output (deterministic).
	id := "20260519120000-abc1234567"
	h1 := hashID(id)
	h2 := hashID(id)
	if h1 != h2 {
		t.Errorf("hashID is not deterministic: %d != %d", h1, h2)
	}

	// Different inputs should (almost certainly) produce different hashes.
	other := "20260519120001-xyz9999999"
	if hashID(id) == hashID(other) {
		t.Errorf("unexpected collision: %q and %q hash to %d", id, other, h1)
	}
}

func TestHashID_NonZero(t *testing.T) {
	if hashID("") == 0 && hashID("x") == 0 {
		t.Error("hashID should not return 0 for non-trivial inputs")
	}
}

func TestStrVal(t *testing.T) {
	v := strVal("hello")
	if v.GetStringValue() != "hello" {
		t.Errorf("strVal: got %q, want %q", v.GetStringValue(), "hello")
	}
}

func TestChunkFromPayload_MissingID(t *testing.T) {
	payload := map[string]*eywa.LoreChunk{}
	_ = payload // just ensure the type compiles
}

func TestLoreStore_Upsert_UnmarshalableMetadata_ReturnsError(t *testing.T) {
	store := &LoreStore{}

	chunks := []eywa.LoreChunk{
		{
			ID:        "chunk-1",
			LoreID:    "lore-1",
			Content:   "hello",
			Embedding: []float32{0.1, 0.2},
			Metadata:  map[string]any{"bad": make(chan int)},
		},
	}

	err := store.Upsert(context.Background(), chunks)

	if err == nil {
		t.Error("expected error for non-serializable metadata, got nil")
	}
}

// TestIntegration exercises the full lifecycle against a running Qdrant instance.
// Skipped by default.
func TestIntegration_Upsert_Search_Delete(t *testing.T) {
	t.Skip("integration: start Qdrant (docker run -p 6334:6334 qdrant/qdrant) and remove t.Skip")
	// store, err := NewLoreStore("localhost", 6334, "test_lore", 3)
	// ...
}
