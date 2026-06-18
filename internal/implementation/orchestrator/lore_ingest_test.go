package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type capturingLoreStore struct {
	upserted []entities.LoreChunk
}

func (s *capturingLoreStore) Upsert(_ context.Context, chunks []entities.LoreChunk) error {
	s.upserted = chunks
	return nil
}
func (s *capturingLoreStore) Search(_ context.Context, _ string, _ []float32, _ int, _ float64) ([]entities.LoreChunk, error) {
	return nil, nil
}
func (s *capturingLoreStore) Delete(_ context.Context, _ string) error { return nil }

func ingestWeave(store *capturingLoreStore) *Weave {
	return &Weave{
		loreRepository: &stubLoreRepoWithData{lore: entities.Lore{ID: "lore", ChunkSize: 20, Overlap: 0}},
		loreStore:      store,
		loreEmbedder:   &stubSuccessLoreEmbedder{},
	}
}

func TestIngestLore_RecordMode_StableIDAndDocMetadata(t *testing.T) {
	store := &capturingLoreStore{}
	w := ingestWeave(store)

	err := w.IngestLore(context.Background(), entities.LoreIngestion{
		LoreID:     "lore",
		DocumentID: "cand_1",
		NoChunk:    true,
		Text:       strings.Repeat("long profile text ", 20), // would split if chunked
		Metadata:   map[string]any{"location": "SP"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("NoChunk must produce one chunk, got %d", len(store.upserted))
	}
	c := store.upserted[0]
	if c.ID != "cand_1" {
		t.Errorf("expected stable ID 'cand_1', got %q", c.ID)
	}
	if c.Metadata[entities.LoreDocumentIDKey] != "cand_1" || c.Metadata["location"] != "SP" {
		t.Errorf("expected document id + original metadata, got %v", c.Metadata)
	}
}

func TestIngestLore_RecordMode_DoesNotMutateCallerMetadata(t *testing.T) {
	store := &capturingLoreStore{}
	w := ingestWeave(store)
	caller := map[string]any{"location": "SP"}

	if err := w.IngestLore(context.Background(), entities.LoreIngestion{
		LoreID: "lore", DocumentID: "d1", NoChunk: true, Text: "hi", Metadata: caller,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, leaked := caller[entities.LoreDocumentIDKey]; leaked {
		t.Error("IngestLore must not mutate the caller's metadata map")
	}
}

func TestIngestLore_DocumentID_Chunked_SuffixesAndGroups(t *testing.T) {
	store := &capturingLoreStore{}
	w := ingestWeave(store)

	err := w.IngestLore(context.Background(), entities.LoreIngestion{
		LoreID:     "lore",
		DocumentID: "doc_1",
		Text:       strings.Repeat("alpha beta gamma delta. ", 10), // > chunkSize 20 → multiple chunks
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.upserted) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(store.upserted))
	}
	for i, c := range store.upserted {
		if !strings.HasPrefix(c.ID, "doc_1_") {
			t.Errorf("chunk %d expected doc_1_* id, got %q", i, c.ID)
		}
		if c.Metadata[entities.LoreDocumentIDKey] != "doc_1" {
			t.Errorf("chunk %d missing document id in metadata: %v", i, c.Metadata)
		}
	}
}

func TestIngestLore_LegacyPositionalIDs(t *testing.T) {
	store := &capturingLoreStore{}
	w := ingestWeave(store)

	if err := w.IngestLore(context.Background(), entities.LoreIngestion{
		LoreID: "lore", NoChunk: true, Text: "hi", Metadata: map[string]any{"k": "v"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := store.upserted[0]
	if c.ID != "lore_0" {
		t.Errorf("expected legacy positional id 'lore_0', got %q", c.ID)
	}
	if _, ok := c.Metadata[entities.LoreDocumentIDKey]; ok {
		t.Error("no DocumentID → metadata must not carry a document id")
	}
}

func TestChunkID(t *testing.T) {
	cases := []struct {
		ing   entities.LoreIngestion
		i     int
		total int
		want  string
	}{
		{entities.LoreIngestion{LoreID: "l", DocumentID: "d"}, 0, 1, "d"},
		{entities.LoreIngestion{LoreID: "l", DocumentID: "d"}, 2, 5, "d_2"},
		{entities.LoreIngestion{LoreID: "l"}, 3, 5, "l_3"},
	}
	for _, tc := range cases {
		if got := chunkID(tc.ing, tc.i, tc.total); got != tc.want {
			t.Errorf("chunkID(%+v,%d,%d) = %q, want %q", tc.ing, tc.i, tc.total, got, tc.want)
		}
	}
}
