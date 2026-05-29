package weaviate

import (
	"context"
	"testing"

	"github.com/weaviate/weaviate/entities/models"
	eywa "github.com/wmulabs/eywa"
)

// Interface compliance — fails at compile time if LoreStore diverges from the port.
var _ eywa.LoreStore = (*LoreStore)(nil)

func TestStrField(t *testing.T) {
	m := map[string]any{
		"key":   "value",
		"other": 42,
	}
	if got := strField(m, "key"); got != "value" {
		t.Errorf("want %q, got %q", "value", got)
	}
	if got := strField(m, "other"); got != "" {
		t.Errorf("non-string field should return empty, got %q", got)
	}
	if got := strField(m, "missing"); got != "" {
		t.Errorf("missing key should return empty, got %q", got)
	}
}

func TestParseSearchResults_Empty(t *testing.T) {
	chunks, err := parseSearchResults(nil, "LoreChunk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected empty result, got %d chunks", len(chunks))
	}
}

func TestParseSearchResults_MalformedData(t *testing.T) {
	data := map[string]models.JSONObject{
		"Get": map[string]any{
			"LoreChunk": []any{
				nil,
				map[string]any{
					"chunk_id": "c1",
					"lore_id":  "l1",
					"content":  "hello",
				},
			},
		},
	}
	chunks, err := parseSearchResults(data, "LoreChunk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// nil item is skipped; the valid one is included
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].ID != "c1" {
		t.Errorf("expected chunk_id c1, got %q", chunks[0].ID)
	}
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

func TestLoreStore_Upsert_SkipsChunksWithNoEmbedding(t *testing.T) {
	chunks := []eywa.LoreChunk{
		{ID: "no-embed", LoreID: "lore-1", Content: "text", Embedding: nil},
	}

	// LoreStore with nil client — should not panic because chunk is skipped before any client call
	s := &LoreStore{className: "Test"}
	if err := s.Upsert(context.Background(), chunks); err != nil {
		t.Errorf("expected nil error for chunk with no embedding, got %v", err)
	}
}

// TestIntegration exercises the full lifecycle against a running Weaviate instance.
func TestIntegration_Upsert_Search_Delete(t *testing.T) {
	t.Skip("integration: start Weaviate (docker run -p 8080:8080 semitechnologies/weaviate) and remove t.Skip")
}

// Silence unused import warning when integration body is commented out.
var _ eywa.LoreChunk
