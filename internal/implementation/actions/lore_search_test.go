package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubLoreRepo struct {
	lore entities.Lore
	err  error
}

var _ ports.LoreRepository = (*stubLoreRepo)(nil)

func (r *stubLoreRepo) GetByName(_ context.Context, _ string) (entities.Lore, error) {
	return r.lore, r.err
}

func (r *stubLoreRepo) Create(context.Context, entities.Lore) error {
	panic("not implemented")
}

func (r *stubLoreRepo) GetByID(context.Context, string) (entities.Lore, error) {
	panic("not implemented")
}

func (r *stubLoreRepo) GetBySpiritID(context.Context, string) ([]entities.Lore, error) {
	panic("not implemented")
}

func (r *stubLoreRepo) GetByIDs(context.Context, []string) ([]entities.Lore, error) {
	panic("not implemented")
}

func (r *stubLoreRepo) Update(context.Context, entities.Lore) error {
	panic("not implemented")
}

func (r *stubLoreRepo) Delete(context.Context, string) error {
	panic("not implemented")
}

type stubLoreStore struct {
	chunks []entities.LoreChunk
	err    error
}

var _ ports.LoreStore = (*stubLoreStore)(nil)

func (s *stubLoreStore) Search(_ context.Context, _ string, _ []float32, _ int, _ float64) ([]entities.LoreChunk, error) {
	return s.chunks, s.err
}

func (s *stubLoreStore) Upsert(context.Context, []entities.LoreChunk) error {
	panic("not implemented")
}

func (s *stubLoreStore) Delete(context.Context, string) error {
	panic("not implemented")
}

type stubLoreEmbedder struct {
	embeddings [][]float32
	err        error
}

var _ ports.LoreEmbedder = (*stubLoreEmbedder)(nil)

func (e *stubLoreEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return e.embeddings, e.err
}

func (e *stubLoreEmbedder) Dimensions() int {
	return 1536
}

func TestSearchLoreAction_Validate_MissingLoreName(t *testing.T) {
	action := NewSearchLoreAction(nil, nil, nil)
	err := action.Validate(map[string]any{"query": "test"})
	if err == nil {
		t.Errorf("expected error for missing lore_name, got nil")
	}
}

func TestSearchLoreAction_Validate_MissingQuery(t *testing.T) {
	action := NewSearchLoreAction(nil, nil, nil)
	err := action.Validate(map[string]any{"lore_name": "test_kb"})
	if err == nil {
		t.Errorf("expected error for missing query, got nil")
	}
}

func TestSearchLoreAction_Validate_Valid(t *testing.T) {
	action := NewSearchLoreAction(nil, nil, nil)
	err := action.Validate(map[string]any{
		"lore_name": "test_kb",
		"query":     "test query",
	})
	if err != nil {
		t.Errorf("expected no error for valid args, got %v", err)
	}
}

func TestSearchLoreAction_Execute_LoreNotFound(t *testing.T) {
	repo := &stubLoreRepo{err: errors.New("not found")}
	store := &stubLoreStore{}
	embedder := &stubLoreEmbedder{}

	action := NewSearchLoreAction(repo, store, embedder)
	result, err := action.Execute(context.Background(), map[string]any{
		"lore_name": "missing_kb",
		"query":     "test query",
	})

	if err == nil {
		t.Errorf("expected error when lore not found, got nil")
	}
	if !domainerrors.IsBusinessError(err) {
		t.Errorf("expected business error, got %T", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestSearchLoreAction_Execute_EmbedError(t *testing.T) {
	repo := &stubLoreRepo{
		lore: entities.Lore{ID: "lore-1", Name: "test_kb"},
		err:  nil,
	}
	store := &stubLoreStore{}
	embedder := &stubLoreEmbedder{err: errors.New("embed failed")}

	action := NewSearchLoreAction(repo, store, embedder)
	result, err := action.Execute(context.Background(), map[string]any{
		"lore_name": "test_kb",
		"query":     "test query",
	})

	if err == nil {
		t.Errorf("expected error when embed fails, got nil")
	}
	if !domainerrors.IsInfrastructureError(err) {
		t.Errorf("expected infrastructure error, got %T", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestSearchLoreAction_Execute_NoChunksFound(t *testing.T) {
	repo := &stubLoreRepo{
		lore: entities.Lore{ID: "lore-1", Name: "test_kb"},
		err:  nil,
	}
	store := &stubLoreStore{chunks: []entities.LoreChunk{}}
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{1, 2, 3}}}

	action := NewSearchLoreAction(repo, store, embedder)
	result, err := action.Execute(context.Background(), map[string]any{
		"lore_name": "test_kb",
		"query":     "test query",
	})

	if err != nil {
		t.Errorf("expected no error for empty chunks, got %v", err)
	}
	if result == "" {
		t.Errorf("expected non-empty result for empty chunks case, got empty string")
	}
	if !contains(result, "No relevant content") {
		t.Errorf("expected 'No relevant content' in result, got %q", result)
	}
}

func TestSearchLoreAction_Execute_ReturnsFormattedChunks(t *testing.T) {
	repo := &stubLoreRepo{
		lore: entities.Lore{ID: "lore-1", Name: "test_kb"},
		err:  nil,
	}
	store := &stubLoreStore{
		chunks: []entities.LoreChunk{
			{ID: "chunk-1", LoreID: "lore-1", Content: "first"},
			{ID: "chunk-2", LoreID: "lore-1", Content: "second"},
		},
	}
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{1, 2, 3}}}

	action := NewSearchLoreAction(repo, store, embedder)
	result, err := action.Execute(context.Background(), map[string]any{
		"lore_name": "test_kb",
		"query":     "test query",
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	expected := "[1] first\n[2] second"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSearchLoreAction_Metadata(t *testing.T) {
	a := &SearchLoreAction{}
	if a.GetName() != "search_lore" {
		t.Errorf("GetName = %q, want %q", a.GetName(), "search_lore")
	}
	if a.GetDescription() == "" {
		t.Error("GetDescription returned empty")
	}
	if a.GetParameters() == nil {
		t.Error("GetParameters returned nil")
	}
	if a.GetCategory() != ports.ActionRetrieval {
		t.Errorf("GetCategory = %v, want ActionRetrieval", a.GetCategory())
	}
	if a.IsCritical() {
		t.Error("IsCritical should return false")
	}
}

func TestSearchLoreAction_Execute_WithTopK(t *testing.T) {
	repo := &stubLoreRepo{lore: entities.Lore{ID: "lore-1", Name: "kb"}}
	store := &stubLoreStore{
		chunks: []entities.LoreChunk{{ID: "c1", Content: "result"}},
	}
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{0.1, 0.2}}}

	action := NewSearchLoreAction(repo, store, embedder)
	result, err := action.Execute(context.Background(), map[string]any{
		"lore_name": "kb",
		"query":     "q",
		"top_k":     float64(3),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestSearchLoreAction_Execute_TopK_NonPositive(t *testing.T) {
	repo := &stubLoreRepo{lore: entities.Lore{ID: "lore-1", Name: "kb"}}
	store := &stubLoreStore{
		chunks: []entities.LoreChunk{{Content: "result"}},
	}
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{0.1}}}

	action := NewSearchLoreAction(repo, store, embedder)
	// top_k=0 → clamped to default 5
	_, err := action.Execute(context.Background(), map[string]any{
		"lore_name": "kb",
		"query":     "q",
		"top_k":     float64(0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchLoreAction_Execute_EmptyEmbeddings(t *testing.T) {
	repo := &stubLoreRepo{lore: entities.Lore{ID: "lore-1", Name: "kb"}}
	store := &stubLoreStore{}
	embedder := &stubLoreEmbedder{embeddings: [][]float32{}} // empty, no error

	action := NewSearchLoreAction(repo, store, embedder)
	_, err := action.Execute(context.Background(), map[string]any{
		"lore_name": "kb",
		"query":     "q",
	})
	if err == nil {
		t.Error("expected error when embeddings are empty")
	}
}

func TestSearchLoreAction_Execute_SearchError(t *testing.T) {
	repo := &stubLoreRepo{lore: entities.Lore{ID: "lore-1", Name: "kb"}}
	store := &stubLoreStore{err: errors.New("search failed")}
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{0.1}}}

	action := NewSearchLoreAction(repo, store, embedder)
	_, err := action.Execute(context.Background(), map[string]any{
		"lore_name": "kb",
		"query":     "q",
	})
	if err == nil {
		t.Error("expected error from loreStore.Search")
	}
}
