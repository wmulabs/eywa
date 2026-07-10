package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

type stubLoreRepository struct {
	lores   []eywa.Lore
	lore    eywa.Lore
	byName  *eywa.Lore
	created *eywa.Lore
	err     error
}

func (s *stubLoreRepository) Create(_ context.Context, lore eywa.Lore) error {
	if s.err != nil {
		return s.err
	}
	s.created = &lore
	return nil
}
func (s *stubLoreRepository) GetByID(_ context.Context, _ string) (eywa.Lore, error) {
	if s.err != nil {
		return eywa.Lore{}, s.err
	}
	if s.lore.ID == "" {
		return eywa.Lore{}, eywa.ErrNotFound
	}
	return s.lore, nil
}
func (s *stubLoreRepository) GetByName(_ context.Context, _ string) (eywa.Lore, error) {
	if s.byName != nil {
		return *s.byName, nil
	}
	return eywa.Lore{}, eywa.ErrNotFound
}
func (s *stubLoreRepository) GetBySpiritID(_ context.Context, _ string) ([]eywa.Lore, error) {
	return s.lores, s.err
}
func (s *stubLoreRepository) GetByIDs(_ context.Context, _ []string) ([]eywa.Lore, error) {
	return s.lores, s.err
}
func (s *stubLoreRepository) List(_ context.Context) ([]eywa.Lore, error) {
	return s.lores, s.err
}
func (s *stubLoreRepository) Update(_ context.Context, _ eywa.Lore) error { return s.err }
func (s *stubLoreRepository) Delete(_ context.Context, _ string) error    { return s.err }

// stubLoreStore implements LoreStore without chunk listing.
type stubLoreStore struct {
	deleted string
	err     error
}

func (s *stubLoreStore) Upsert(_ context.Context, _ []eywa.LoreChunk) error { return s.err }
func (s *stubLoreStore) Search(_ context.Context, _ string, _ []float32, _ int, _ float64) ([]eywa.LoreChunk, error) {
	return nil, s.err
}
func (s *stubLoreStore) Delete(_ context.Context, loreID string) error {
	s.deleted = loreID
	return s.err
}

// stubListingStore adds the optional chunkLister capability.
type stubListingStore struct {
	stubLoreStore
	chunks []eywa.LoreChunk
	total  int64
}

func (s *stubListingStore) ListChunks(_ context.Context, _ string, _, _ int) ([]eywa.LoreChunk, int64, error) {
	return s.chunks, s.total, s.err
}

type stubIngestor struct {
	got *eywa.LoreIngestion
	err error
}

func (s *stubIngestor) IngestLore(_ context.Context, ingestion eywa.LoreIngestion) error {
	s.got = &ingestion
	return s.err
}

type stubSearcher struct {
	chunks []eywa.LoreChunk
	err    error
}

func (s *stubSearcher) SearchLore(_ context.Context, _, _ string, _ eywa.LoreSearchOptions) ([]eywa.LoreChunk, error) {
	return s.chunks, s.err
}

// buildLoreTestApp registers the lore routes directly with stub seams (RegisterRoutes wires the
// concrete Weave as ingestor/searcher; tests need the stubs instead).
func buildLoreTestApp(t *testing.T, h *loreHandler) *fiberlib.App {
	t.Helper()
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	err := RegisterRoutes(app, minimalTestWeave(t), RouteDeps{APIKeys: authedAPIKeys()})
	if err != nil {
		t.Fatalf("RegisterRoutes: %v", err)
	}
	// Mount under the same auth-protected prefix the registrar uses.
	api := app.Group("/api/v1/lores")
	api.Get("", h.list)
	api.Post("", h.create)
	api.Get("/:id", h.getByID)
	api.Put("/:id", h.update)
	api.Delete("/:id", h.delete)
	api.Get("/:id/chunks", h.listChunks)
	api.Post("/:id/documents", h.ingestDocument)
	api.Post("/:id/query", h.query)
	return app
}

func TestLoreHandler_List_Returns200(t *testing.T) {
	h := newLoreHandler(&stubLoreRepository{lores: []eywa.Lore{{ID: "l1", Name: "faq"}}}, nil, nil, nil)
	app := buildLoreTestApp(t, h)

	resp, _ := app.Test(authedRequest("GET", "/api/v1/lores", nil))
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_Create_Returns201(t *testing.T) {
	repo := &stubLoreRepository{}
	app := buildLoreTestApp(t, newLoreHandler(repo, nil, nil, nil))

	b, _ := json.Marshal(map[string]any{"name": "faq", "chunk_size": 500})
	req := authedRequest("POST", "/api/v1/lores", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	if repo.created == nil || repo.created.ID == "" {
		t.Error("expected created lore with generated id")
	}
}

func TestLoreHandler_Create_MissingName_Returns400(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, nil, nil))

	b, _ := json.Marshal(map[string]any{"description": "no name"})
	req := authedRequest("POST", "/api/v1/lores", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_Create_DuplicateName_Returns409(t *testing.T) {
	existing := eywa.Lore{ID: "l1", Name: "faq"}
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{byName: &existing}, nil, nil, nil))

	b, _ := json.Marshal(map[string]any{"name": "faq"})
	req := authedRequest("POST", "/api/v1/lores", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 409 {
		t.Errorf("want 409, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_GetByID_NotFound_Returns404(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, nil, nil))

	resp, _ := app.Test(authedRequest("GET", "/api/v1/lores/ghost", nil))
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_Delete_CleansVectors(t *testing.T) {
	store := &stubLoreStore{}
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, store, nil, nil))

	resp, _ := app.Test(authedRequest("DELETE", "/api/v1/lores/l1", nil))
	if resp.StatusCode != 204 {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}
	if store.deleted != "l1" {
		t.Errorf("expected vector delete for l1, got %q", store.deleted)
	}
}

func TestLoreHandler_Delete_VectorFailure_Still204(t *testing.T) {
	store := &stubLoreStore{err: errors.New("store down")}
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, store, nil, nil))

	resp, _ := app.Test(authedRequest("DELETE", "/api/v1/lores/l1", nil))
	if resp.StatusCode != 204 {
		t.Errorf("vector cleanup is best-effort; want 204, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_IngestDocument_DelegatesToIngestor(t *testing.T) {
	ing := &stubIngestor{}
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, ing, nil))

	b, _ := json.Marshal(map[string]any{"text": "conteúdo", "document_id": "doc-1", "no_chunk": true})
	req := authedRequest("POST", "/api/v1/lores/l1/documents", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	if ing.got == nil || ing.got.LoreID != "l1" || ing.got.DocumentID != "doc-1" || !ing.got.NoChunk {
		t.Errorf("ingestion payload wrong: %+v", ing.got)
	}
}

func TestLoreHandler_IngestDocument_MissingText_Returns400(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, &stubIngestor{}, nil))

	b, _ := json.Marshal(map[string]any{"document_id": "doc-1"})
	req := authedRequest("POST", "/api/v1/lores/l1/documents", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_IngestDocument_EngineFailure_Returns502(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, &stubIngestor{err: errors.New("embedder down")}, nil))

	b, _ := json.Marshal(map[string]any{"text": "conteúdo"})
	req := authedRequest("POST", "/api/v1/lores/l1/documents", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 502 {
		t.Errorf("want 502, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_ListChunks_StoreWithoutListing_Returns501(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, &stubLoreStore{}, nil, nil))

	resp, _ := app.Test(authedRequest("GET", "/api/v1/lores/l1/chunks", nil))
	if resp.StatusCode != 501 {
		t.Errorf("want 501, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_ListChunks_Returns200(t *testing.T) {
	store := &stubListingStore{chunks: []eywa.LoreChunk{{ID: "c1", Content: "alpha"}}, total: 1}
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, store, nil, nil))

	resp, _ := app.Test(authedRequest("GET", "/api/v1/lores/l1/chunks?limit=10&offset=0", nil))
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []eywa.LoreChunk `json:"items"`
		Total int64            `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body.Total != 1 || len(body.Items) != 1 {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestLoreHandler_Query_Returns200(t *testing.T) {
	searcher := &stubSearcher{chunks: []eywa.LoreChunk{{ID: "c1", Content: "alpha", Score: 0.92}}}
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, nil, searcher))

	b, _ := json.Marshal(map[string]any{"query": "como funciona?", "top_k": 3})
	req := authedRequest("POST", "/api/v1/lores/l1/query", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_Query_MissingQuery_Returns400(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, nil, &stubSearcher{}))

	b, _ := json.Marshal(map[string]any{"top_k": 3})
	req := authedRequest("POST", "/api/v1/lores/l1/query", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestLoreRoutes_RequireAuth(t *testing.T) {
	// Routes mounted via the registrar (not the test-local group) must 401 without a token.
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	err := RegisterRoutes(app, minimalTestWeave(t), RouteDeps{
		APIKeys:  authedAPIKeys(),
		LoreRepo: &stubLoreRepository{},
	})
	if err != nil {
		t.Fatalf("RegisterRoutes: %v", err)
	}

	resp, _ := app.Test(plainRequest("GET", "/api/v1/lores", nil))
	if resp.StatusCode != 401 {
		t.Errorf("want 401 without token, got %d", resp.StatusCode)
	}
	resp, _ = app.Test(authedRequest("GET", "/api/v1/lores", nil))
	if resp.StatusCode != 200 {
		t.Errorf("want 200 with token, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_Update_Returns200(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, nil, nil))

	b, _ := json.Marshal(map[string]any{"name": "faq", "chunk_size": 800})
	req := authedRequest("PUT", "/api/v1/lores/l1", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_Update_NotFound_Returns404(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{err: eywa.ErrNotFound}, nil, nil, nil))

	b, _ := json.Marshal(map[string]any{"name": "faq"})
	req := authedRequest("PUT", "/api/v1/lores/ghost", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_InvalidJSON_Returns400(t *testing.T) {
	h := newLoreHandler(&stubLoreRepository{}, nil, &stubIngestor{}, &stubSearcher{})
	app := buildLoreTestApp(t, h)

	for _, tc := range []struct{ method, target string }{
		{"POST", "/api/v1/lores"},
		{"PUT", "/api/v1/lores/l1"},
		{"POST", "/api/v1/lores/l1/documents"},
		{"POST", "/api/v1/lores/l1/query"},
	} {
		req := authedRequest(tc.method, tc.target, bytes.NewReader([]byte("not-json")))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		if resp.StatusCode != 400 {
			t.Errorf("%s %s: want 400, got %d", tc.method, tc.target, resp.StatusCode)
		}
	}
}

func TestLoreHandler_RepoErrors_Return500(t *testing.T) {
	broken := &stubLoreRepository{err: errors.New("db down")}
	store := &stubListingStore{}
	store.err = errors.New("store down")
	h := newLoreHandler(broken, store, nil, nil)
	app := buildLoreTestApp(t, h)

	for _, tc := range []struct{ method, target string }{
		{"GET", "/api/v1/lores"},
		{"GET", "/api/v1/lores/l1"},
		{"DELETE", "/api/v1/lores/l1"},
		{"GET", "/api/v1/lores/l1/chunks"},
	} {
		resp, _ := app.Test(authedRequest(tc.method, tc.target, nil))
		if resp.StatusCode != 500 {
			t.Errorf("%s %s: want 500, got %d", tc.method, tc.target, resp.StatusCode)
		}
	}
}

func TestLoreHandler_Create_RepoError_Returns500(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{err: errors.New("db down")}, nil, nil, nil))

	b, _ := json.Marshal(map[string]any{"name": "faq"})
	req := authedRequest("POST", "/api/v1/lores", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_Query_EngineFailure_Returns502(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, nil, &stubSearcher{err: errors.New("embedder down")}))

	b, _ := json.Marshal(map[string]any{"query": "algo"})
	req := authedRequest("POST", "/api/v1/lores/l1/query", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 502 {
		t.Errorf("want 502, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_GetByID_Found_Returns200(t *testing.T) {
	repo := &stubLoreRepository{lore: eywa.Lore{ID: "l1", Name: "faq"}}
	app := buildLoreTestApp(t, newLoreHandler(repo, nil, nil, nil))

	resp, _ := app.Test(authedRequest("GET", "/api/v1/lores/l1", nil))
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_Update_RepoError_Returns500(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{err: errors.New("db down")}, nil, nil, nil))

	b, _ := json.Marshal(map[string]any{"name": "faq"})
	req := authedRequest("PUT", "/api/v1/lores/l1", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestLoreHandler_EmptyCollections_ReturnEmptyArrays(t *testing.T) {
	store := &stubListingStore{}                                            // nil chunks, zero total
	h := newLoreHandler(&stubLoreRepository{}, store, nil, &stubSearcher{}) // nil lores/chunks
	app := buildLoreTestApp(t, h)

	resp, _ := app.Test(authedRequest("GET", "/api/v1/lores", nil))
	if resp.StatusCode != 200 {
		t.Errorf("list: want 200, got %d", resp.StatusCode)
	}
	resp, _ = app.Test(authedRequest("GET", "/api/v1/lores/l1/chunks", nil))
	if resp.StatusCode != 200 {
		t.Errorf("chunks: want 200, got %d", resp.StatusCode)
	}
	b, _ := json.Marshal(map[string]any{"query": "algo"})
	req := authedRequest("POST", "/api/v1/lores/l1/query", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("query: want 200, got %d", resp.StatusCode)
	}
}

// Regression: the create response must carry real timestamps (the repo stamps only its own copy).
func TestLoreHandler_Create_ResponseCarriesTimestamps(t *testing.T) {
	app := buildLoreTestApp(t, newLoreHandler(&stubLoreRepository{}, nil, nil, nil))

	b, _ := json.Marshal(map[string]any{"name": "faq"})
	req := authedRequest("POST", "/api/v1/lores", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var lore eywa.Lore
	json.NewDecoder(resp.Body).Decode(&lore) //nolint:errcheck
	if lore.CreatedAt.IsZero() || lore.UpdatedAt.IsZero() {
		t.Errorf("timestamps missing: created=%v updated=%v", lore.CreatedAt, lore.UpdatedAt)
	}
}
