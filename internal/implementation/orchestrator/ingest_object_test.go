package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func objectWeave(store *capturingLoreStore, oracle ports.Oracle) *Weave {
	return &Weave{
		oracleFactory:  &stubOracleFactory{oracle: oracle},
		loreRepository: &stubLoreRepoWithData{lore: entities.Lore{ID: "obj"}},
		loreStore:      store,
		loreEmbedder:   &stubSuccessLoreEmbedder{},
	}
}

func TestIngestObject_VerbalizesAndIndexes(t *testing.T) {
	store := &capturingLoreStore{}
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "  Ann is a senior Go engineer in SP.  "}}
	w := objectWeave(store, oracle)

	err := w.IngestObject(context.Background(), "obj", map[string]any{"name": "Ann", "stack": "go", "location": "SP"}, IngestObjectOptions{
		DocumentID: "cand_1", Provider: "openai", Model: "gpt-4o",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.upserted) != 1 {
		t.Fatalf("expected one record vector, got %d", len(store.upserted))
	}
	c := store.upserted[0]
	if c.ID != "cand_1" {
		t.Errorf("expected stable ID 'cand_1', got %q", c.ID)
	}
	if c.Content != "Ann is a senior Go engineer in SP." {
		t.Errorf("expected trimmed verbalized text as content, got %q", c.Content)
	}
	if c.Metadata["location"] != "SP" || c.Metadata[entities.LoreDocumentIDKey] != "cand_1" {
		t.Errorf("expected object fields + document id in metadata, got %v", c.Metadata)
	}
	// verbalization request: deterministic + carries the default prompt and the JSON record
	if oracle.gotReq.Temperature != 0 {
		t.Errorf("expected temperature 0, got %v", oracle.gotReq.Temperature)
	}
	prompt := oracle.gotReq.Messages[0].Content
	if !strings.Contains(prompt, "natural-language description") || !strings.Contains(prompt, `"name":"Ann"`) {
		t.Errorf("expected default prompt + record JSON, got %q", prompt)
	}
}

func TestIngestObject_CustomPrompt(t *testing.T) {
	store := &capturingLoreStore{}
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "desc"}}
	w := objectWeave(store, oracle)

	err := w.IngestObject(context.Background(), "obj", map[string]any{"x": 1}, IngestObjectOptions{
		Prompt: "Summarize for search:", DocumentID: "d1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(oracle.gotReq.Messages[0].Content, "Summarize for search:") {
		t.Errorf("expected custom prompt, got %q", oracle.gotReq.Messages[0].Content)
	}
}

func TestIngestObject_EmptyObject(t *testing.T) {
	w := objectWeave(&capturingLoreStore{}, &stubOracle{resp: &ports.OracleResponse{Content: "x"}})
	if err := w.IngestObject(context.Background(), "obj", nil, IngestObjectOptions{}); err == nil {
		t.Error("expected an error for an empty object")
	}
}

func TestIngestObject_NotConfigured(t *testing.T) {
	w := &Weave{} // no oracle/store/embedder
	if err := w.IngestObject(context.Background(), "obj", map[string]any{"x": 1}, IngestObjectOptions{}); err == nil {
		t.Error("expected an error when object ingestion is not configured")
	}
}

func TestIngestObject_UnmarshalableObject(t *testing.T) {
	// A channel value can't be marshalled — exercises the record-marshal error path.
	w := objectWeave(&capturingLoreStore{}, &stubOracle{resp: &ports.OracleResponse{Content: "x"}})
	if err := w.IngestObject(context.Background(), "obj", map[string]any{"bad": make(chan int)}, IngestObjectOptions{}); err == nil {
		t.Error("expected an error when the object cannot be marshalled")
	}
}

func TestIngestObject_ProviderError(t *testing.T) {
	w := &Weave{
		oracleFactory:  &stubOracleFactory{err: errors.New("no provider")},
		loreRepository: &stubLoreRepoWithData{lore: entities.Lore{ID: "obj"}},
		loreStore:      &capturingLoreStore{},
		loreEmbedder:   &stubSuccessLoreEmbedder{},
	}
	if err := w.IngestObject(context.Background(), "obj", map[string]any{"x": 1}, IngestObjectOptions{}); err == nil {
		t.Error("expected an error when the oracle provider is unavailable")
	}
}

func TestIngestObject_OracleError(t *testing.T) {
	w := objectWeave(&capturingLoreStore{}, &stubOracle{err: errors.New("api down")})
	if err := w.IngestObject(context.Background(), "obj", map[string]any{"x": 1}, IngestObjectOptions{}); err == nil {
		t.Error("expected the oracle error to propagate")
	}
}

func TestIngestObject_EmptyVerbalization(t *testing.T) {
	w := objectWeave(&capturingLoreStore{}, &stubOracle{resp: &ports.OracleResponse{Content: "   "}})
	if err := w.IngestObject(context.Background(), "obj", map[string]any{"x": 1}, IngestObjectOptions{}); err == nil {
		t.Error("expected an error when verbalization is empty")
	}
}
