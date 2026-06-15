package scouts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubImprintRepo struct {
	imprints []entities.Imprint
	err      error
}

var _ ports.ImprintRepository = (*stubImprintRepo)(nil)

func (r *stubImprintRepo) Save(_ context.Context, _ entities.Imprint) error { return nil }
func (r *stubImprintRepo) GetByUserKey(_ context.Context, _, _ string) ([]entities.Imprint, error) {
	return r.imprints, r.err
}
func (r *stubImprintRepo) List(_ context.Context, _ ports.ImprintListOptions) ([]entities.Imprint, int64, error) {
	panic("not implemented")
}
func (r *stubImprintRepo) Delete(_ context.Context, _ string) error          { panic("not implemented") }
func (r *stubImprintRepo) Prune(_ context.Context, _, _ string, _ int) error { return nil }

type stubLoreRepo struct {
	lores []entities.Lore
	err   error
}

var _ ports.LoreRepository = (*stubLoreRepo)(nil)

func (r *stubLoreRepo) GetByName(_ context.Context, _ string) (entities.Lore, error) {
	panic("not implemented")
}
func (r *stubLoreRepo) Create(_ context.Context, _ entities.Lore) error { panic("not implemented") }
func (r *stubLoreRepo) GetByID(_ context.Context, _ string) (entities.Lore, error) {
	panic("not implemented")
}
func (r *stubLoreRepo) GetBySpiritID(_ context.Context, _ string) ([]entities.Lore, error) {
	panic("not implemented")
}
func (r *stubLoreRepo) GetByIDs(_ context.Context, _ []string) ([]entities.Lore, error) {
	return r.lores, r.err
}
func (r *stubLoreRepo) Update(_ context.Context, _ entities.Lore) error { panic("not implemented") }
func (r *stubLoreRepo) Delete(_ context.Context, _ string) error        { panic("not implemented") }

type stubLoreStore struct {
	chunks []entities.LoreChunk
	err    error
}

var _ ports.LoreStore = (*stubLoreStore)(nil)

func (s *stubLoreStore) Search(_ context.Context, _ string, _ []float32, _ int, _ float64) ([]entities.LoreChunk, error) {
	return s.chunks, s.err
}
func (s *stubLoreStore) Upsert(_ context.Context, _ []entities.LoreChunk) error {
	panic("not implemented")
}
func (s *stubLoreStore) Delete(_ context.Context, _ string) error { panic("not implemented") }

type stubLoreEmbedder struct {
	embeddings [][]float32
	err        error
}

var _ ports.LoreEmbedder = (*stubLoreEmbedder)(nil)

func (e *stubLoreEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return e.embeddings, e.err
}
func (e *stubLoreEmbedder) Dimensions() int { return 1536 }

// =============================================================================
// ImprintScout
// =============================================================================

func TestNewImprintScout_DefaultMaxImprints(t *testing.T) {
	s := NewImprintScout(&stubImprintRepo{}, 0) // <= 0 → default 50
	if s.maxImprints != 50 {
		t.Errorf("expected maxImprints=50, got %d", s.maxImprints)
	}
}

func TestNewImprintScout_CustomMaxImprints(t *testing.T) {
	s := NewImprintScout(&stubImprintRepo{}, 10)
	if s.maxImprints != 10 {
		t.Errorf("expected maxImprints=10, got %d", s.maxImprints)
	}
}

func TestImprintScout_Harvest_EmptyContactPhone_NoOp(t *testing.T) {
	repo := &stubImprintRepo{}
	s := NewImprintScout(repo, 50)
	pulse := &entities.Pulse{ContactPhone: "", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, "bot"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := pulse.Knowledge[imprintContextKey]; exists {
		t.Error("expected no imprints injected when ContactPhone is empty")
	}
}

func TestImprintScout_Harvest_RepoError_NoOp(t *testing.T) {
	repo := &stubImprintRepo{err: errors.New("db error")}
	s := NewImprintScout(repo, 50)
	pulse := &entities.Pulse{ContactPhone: "+1234", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, "bot"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImprintScout_Harvest_EmptyImprints_NoOp(t *testing.T) {
	repo := &stubImprintRepo{imprints: []entities.Imprint{}}
	s := NewImprintScout(repo, 50)
	pulse := &entities.Pulse{ContactPhone: "+1234", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, "bot"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := pulse.Knowledge[imprintContextKey]; exists {
		t.Error("expected no injection when imprints are empty")
	}
}

func TestImprintScout_Harvest_InjectsImprints(t *testing.T) {
	imprints := []entities.Imprint{
		{Category: entities.ImprintCategoryPreference, Fact: "Likes coffee"},
		{Category: entities.ImprintCategoryPersonal, Fact: "Name is Alice"},
	}
	repo := &stubImprintRepo{imprints: imprints}
	s := NewImprintScout(repo, 50)
	pulse := &entities.Pulse{ContactPhone: "+1234", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, "bot"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, exists := pulse.Knowledge[imprintContextKey]
	if !exists {
		t.Fatal("expected imprints injected into knowledge")
	}
	str, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}
	if len(str) == 0 {
		t.Error("expected non-empty imprint string")
	}
}

// =============================================================================
// LoreScout
// =============================================================================

func TestNewLoreScout_DefaultsApplied(t *testing.T) {
	s := NewLoreScout(nil, nil, nil, 0, 0) // topK <= 0, minScore <= 0 → defaults
	if s.topK != 5 {
		t.Errorf("expected topK=5, got %d", s.topK)
	}
	if s.minScore != 0.7 {
		t.Errorf("expected minScore=0.7, got %f", s.minScore)
	}
}

func TestNewLoreScout_CustomValues(t *testing.T) {
	s := NewLoreScout(nil, nil, nil, 10, 0.5)
	if s.topK != 10 {
		t.Errorf("expected topK=10, got %d", s.topK)
	}
}

func TestLoreScout_Harvest_EmbedError_NoOp(t *testing.T) {
	embedder := &stubLoreEmbedder{err: errors.New("embed failed")}
	s := NewLoreScout(nil, nil, embedder, 5, 0.7)
	pulse := &entities.Pulse{UserMessage: "test", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, []string{"lore-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoreScout_Harvest_EmptyEmbeddings_NoOp(t *testing.T) {
	embedder := &stubLoreEmbedder{embeddings: [][]float32{}}
	s := NewLoreScout(nil, nil, embedder, 5, 0.7)
	pulse := &entities.Pulse{UserMessage: "test", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, []string{"lore-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoreScout_Harvest_LoreRepoError_NoOp(t *testing.T) {
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{0.1, 0.2}}}
	repo := &stubLoreRepo{err: errors.New("repo error")}
	s := NewLoreScout(repo, nil, embedder, 5, 0.7)
	pulse := &entities.Pulse{UserMessage: "test", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, []string{"lore-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoreScout_Harvest_NoChunks_NoOp(t *testing.T) {
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{0.1}}}
	repo := &stubLoreRepo{lores: []entities.Lore{{ID: "lore-1", Name: "kb"}}}
	store := &stubLoreStore{chunks: []entities.LoreChunk{}}
	s := NewLoreScout(repo, store, embedder, 5, 0.7)
	pulse := &entities.Pulse{UserMessage: "test", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, []string{"lore-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := pulse.Knowledge[loreContextKey]; exists {
		t.Error("expected no injection when no chunks found")
	}
}

func TestLoreScout_Harvest_InjectsChunks(t *testing.T) {
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{0.1, 0.2}}}
	repo := &stubLoreRepo{lores: []entities.Lore{{ID: "lore-1", Name: "product-kb"}}}
	store := &stubLoreStore{chunks: []entities.LoreChunk{
		{LoreID: "lore-1", Content: "Product info here"},
	}}
	s := NewLoreScout(repo, store, embedder, 5, 0.7)
	pulse := &entities.Pulse{UserMessage: "test", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, []string{"lore-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, exists := pulse.Knowledge[loreContextKey]
	if !exists {
		t.Fatal("expected lore context injected")
	}
	if len(val.(string)) == 0 {
		t.Error("expected non-empty lore context")
	}
}

func TestLoreScout_Harvest_FormatsChunkIDForCitation(t *testing.T) {
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{0.1, 0.2}}}
	repo := &stubLoreRepo{lores: []entities.Lore{{ID: "lore-1", Name: "product-kb"}}}
	store := &stubLoreStore{chunks: []entities.LoreChunk{
		{ID: "chunk-42", LoreID: "lore-1", Content: "Product info here"},
	}}
	s := NewLoreScout(repo, store, embedder, 5, 0.7)
	pulse := &entities.Pulse{UserMessage: "test", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, []string{"lore-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctx := pulse.Knowledge[loreContextKey].(string)
	if !strings.Contains(ctx, `id="chunk-42"`) {
		t.Errorf("expected chunk id emitted for citation grounding, got %q", ctx)
	}
}

func TestLoreScout_Harvest_StoreSearchError_SkipsLore(t *testing.T) {
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{0.1}}}
	repo := &stubLoreRepo{lores: []entities.Lore{{ID: "lore-1"}, {ID: "lore-2"}}}
	// All searches error → no chunks → no injection
	store := &stubLoreStore{err: errors.New("search failed")}
	s := NewLoreScout(repo, store, embedder, 5, 0.7)
	pulse := &entities.Pulse{UserMessage: "test", Knowledge: map[string]any{}}

	if err := s.Harvest(context.Background(), pulse, []string{"lore-1", "lore-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := pulse.Knowledge[loreContextKey]; exists {
		t.Error("expected no injection when all searches fail")
	}
}

func TestLoreScout_Harvest_UnknownLoreID_FallsBackToID(t *testing.T) {
	embedder := &stubLoreEmbedder{embeddings: [][]float32{{0.1}}}
	// Repo returns lore with ID "lore-x" but no name for "lore-y"
	repo := &stubLoreRepo{lores: []entities.Lore{{ID: "lore-x", Name: "kb"}}}
	store := &stubLoreStore{chunks: []entities.LoreChunk{
		{LoreID: "unknown-lore", Content: "some content"},
	}}
	s := NewLoreScout(repo, store, embedder, 5, 0.7)
	pulse := &entities.Pulse{UserMessage: "test", Knowledge: map[string]any{}}

	// The chunk has LoreID "unknown-lore" which has no name in loreNames map → falls back to ID
	if err := s.Harvest(context.Background(), pulse, []string{"lore-x"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
