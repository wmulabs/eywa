package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubSearchLoreStore struct {
	chunks      []entities.LoreChunk
	err         error
	gotTopK     int
	gotMinScore float64
}

func (s *stubSearchLoreStore) Upsert(_ context.Context, _ []entities.LoreChunk) error { return nil }
func (s *stubSearchLoreStore) Delete(_ context.Context, _ string) error               { return nil }
func (s *stubSearchLoreStore) Search(_ context.Context, _ string, _ []float32, topK int, minScore float64) ([]entities.LoreChunk, error) {
	s.gotTopK = topK
	s.gotMinScore = minScore
	return s.chunks, s.err
}

type stubFilterableLoreStore struct {
	stubSearchLoreStore
	filtered []entities.LoreChunk
	fErr     error
	gotOpts  ports.LoreSearchOptions
}

func (s *stubFilterableLoreStore) SearchFiltered(_ context.Context, _ string, _ []float32, opts ports.LoreSearchOptions) ([]entities.LoreChunk, error) {
	s.gotOpts = opts
	return s.filtered, s.fErr
}

type stubEmptyEmbedder struct{}

func (stubEmptyEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) { return nil, nil }
func (stubEmptyEmbedder) Dimensions() int                                          { return 2 }

func TestSearchLore_NotConfigured(t *testing.T) {
	if _, err := (&Weave{}).SearchLore(context.Background(), "lore", "q", ports.LoreSearchOptions{}); err == nil {
		t.Error("expected error when lore store/embedder are not configured")
	}
}

func TestSearchLore_EmbedError(t *testing.T) {
	w := &Weave{loreStore: &stubSearchLoreStore{}, loreEmbedder: &stubErrLoreEmbedder{err: errors.New("boom")}}
	if _, err := w.SearchLore(context.Background(), "lore", "q", ports.LoreSearchOptions{}); err == nil {
		t.Error("expected error when embedding the query fails")
	}
}

func TestSearchLore_EmptyEmbedding(t *testing.T) {
	w := &Weave{loreStore: &stubSearchLoreStore{}, loreEmbedder: stubEmptyEmbedder{}}
	if _, err := w.SearchLore(context.Background(), "lore", "q", ports.LoreSearchOptions{}); err == nil {
		t.Error("expected error when the embedder returns no vector")
	}
}

func TestSearchLore_PlainStore_DefaultsTopKAndIgnoresFilter(t *testing.T) {
	store := &stubSearchLoreStore{chunks: []entities.LoreChunk{{ID: "c1", Content: "hit"}}}
	w := &Weave{loreStore: store, loreEmbedder: &stubSuccessLoreEmbedder{}}

	got, err := w.SearchLore(context.Background(), "lore", "q", ports.LoreSearchOptions{
		Filter: &ports.LoreFilter{Equals: map[string]any{"k": "v"}}, // ignored by a plain store
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c1" {
		t.Errorf("unexpected results: %+v", got)
	}
	if store.gotTopK != 5 {
		t.Errorf("expected default TopK 5, got %d", store.gotTopK)
	}
}

func TestSearchLore_PlainStore_Error(t *testing.T) {
	store := &stubSearchLoreStore{err: errors.New("db down")}
	w := &Weave{loreStore: store, loreEmbedder: &stubSuccessLoreEmbedder{}}
	if _, err := w.SearchLore(context.Background(), "lore", "q", ports.LoreSearchOptions{}); err == nil {
		t.Error("expected the store error to propagate")
	}
}

func TestSearchLore_FilterableStore_AppliesFilter(t *testing.T) {
	store := &stubFilterableLoreStore{filtered: []entities.LoreChunk{{ID: "cand_1", Score: 0.92}}}
	w := &Weave{loreStore: store, loreEmbedder: &stubSuccessLoreEmbedder{}}

	filter := &ports.LoreFilter{
		Equals: map[string]any{"location": "SP"},
		Ranges: map[string]ports.LoreRange{"salary": {Max: ptrFloat(12000)}},
	}
	got, err := w.SearchLore(context.Background(), "candidates", "senior go engineer", ports.LoreSearchOptions{TopK: 10, Filter: filter})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "cand_1" || got[0].Score != 0.92 {
		t.Errorf("expected scored filtered result, got %+v", got)
	}
	if store.gotOpts.Filter != filter || store.gotOpts.TopK != 10 {
		t.Errorf("expected opts forwarded to SearchFiltered, got %+v", store.gotOpts)
	}
}

func TestSearchLore_FilterableStore_Error(t *testing.T) {
	store := &stubFilterableLoreStore{fErr: errors.New("filter unsupported")}
	w := &Weave{loreStore: store, loreEmbedder: &stubSuccessLoreEmbedder{}}
	if _, err := w.SearchLore(context.Background(), "lore", "q", ports.LoreSearchOptions{}); err == nil {
		t.Error("expected the filtered-search error to propagate")
	}
}

func ptrFloat(f float64) *float64 { return &f }

func docChunk(id, docID string, score float64) entities.LoreChunk {
	c := entities.LoreChunk{ID: id, Score: score}
	if docID != "" {
		c.Metadata = map[string]any{entities.LoreDocumentIDKey: docID}
	}
	return c
}

func TestGroupByDocument(t *testing.T) {
	chunks := []entities.LoreChunk{
		docChunk("a1", "A", 0.9),
		docChunk("a2", "A", 0.7), // same doc, lower score → dropped
		docChunk("b1", "B", 0.8),
		docChunk("c1", "", 0.6), // no doc id → its own object (keyed by chunk ID)
	}

	got := groupByDocument(chunks, 2)
	if len(got) != 2 {
		t.Fatalf("expected top-2 distinct documents, got %d: %+v", len(got), got)
	}
	if got[0].ID != "a1" || got[1].ID != "b1" {
		t.Errorf("expected A(0.9) then B(0.8), got %+v", got)
	}
}

func TestGroupByDocument_KeepsHigherScoreRegardlessOfOrder(t *testing.T) {
	// A later chunk of the same document with a higher score must replace the earlier one.
	chunks := []entities.LoreChunk{docChunk("low", "A", 0.4), docChunk("high", "A", 0.95)}
	got := groupByDocument(chunks, 5)
	if len(got) != 1 || got[0].ID != "high" {
		t.Errorf("expected the higher-scoring chunk to represent the document, got %+v", got)
	}
}

func TestGroupByDocument_NoLimit(t *testing.T) {
	chunks := []entities.LoreChunk{docChunk("a1", "A", 0.9), docChunk("b1", "B", 0.8), docChunk("c1", "", 0.6)}
	if got := groupByDocument(chunks, 0); len(got) != 3 {
		t.Errorf("topK 0 must not limit, got %d", len(got))
	}
}

func TestSearchLore_GroupByDocument_OverfetchesAndCollapses(t *testing.T) {
	store := &stubFilterableLoreStore{filtered: []entities.LoreChunk{
		docChunk("a1", "A", 0.9),
		docChunk("a2", "A", 0.7),
		docChunk("b1", "B", 0.8),
	}}
	w := &Weave{loreStore: store, loreEmbedder: &stubSuccessLoreEmbedder{}}

	got, err := w.SearchLore(context.Background(), "lore", "q", ports.LoreSearchOptions{
		TopK: 2, GroupByDocument: true, Overfetch: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.gotOpts.TopK != 6 {
		t.Errorf("expected over-fetch TopK 2*3=6, got %d", store.gotOpts.TopK)
	}
	if len(got) != 2 || got[0].ID != "a1" || got[1].ID != "b1" {
		t.Errorf("expected 2 distinct documents A,B, got %+v", got)
	}
}

func TestSearchLore_GroupByDocument_DefaultOverfetch(t *testing.T) {
	store := &stubFilterableLoreStore{filtered: []entities.LoreChunk{docChunk("a1", "A", 0.9)}}
	w := &Weave{loreStore: store, loreEmbedder: &stubSuccessLoreEmbedder{}}

	if _, err := w.SearchLore(context.Background(), "lore", "q", ports.LoreSearchOptions{TopK: 4, GroupByDocument: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.gotOpts.TopK != 20 { // 4 * defaultOverfetch(5)
		t.Errorf("expected default over-fetch 4*5=20, got %d", store.gotOpts.TopK)
	}
}
