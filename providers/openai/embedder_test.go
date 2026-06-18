package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Returns index 1 before index 0 to verify the embedder re-orders by Index.
const embeddingReply = `{"object":"list","model":"text-embedding-3-small","data":[{"object":"embedding","index":1,"embedding":[0.3,0.4]},{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":4,"total_tokens":4}}`

func TestEmbedder_Embed(t *testing.T) {
	var body string
	server := captureServer(t, embeddingReply, &body)
	defer server.Close()

	emb := NewEmbedder(Config{APIKey: "k", BaseURL: server.URL}, "", 0)
	vecs, err := emb.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	// index 0 → [0.1,0.2], index 1 → [0.3,0.4] despite reversed response order
	if vecs[0][0] != 0.1 || vecs[0][1] != 0.2 || vecs[1][0] != 0.3 || vecs[1][1] != 0.4 {
		t.Errorf("vectors not ordered by index: %v", vecs)
	}
	if !strings.Contains(body, `"text-embedding-3-small"`) || !strings.Contains(body, `"hello"`) {
		t.Errorf("expected model and input in request, got %s", body)
	}
}

func TestEmbedder_Embed_WithDimensions(t *testing.T) {
	var body string
	server := captureServer(t, embeddingReply, &body)
	defer server.Close()

	emb := NewEmbedder(Config{APIKey: "k", BaseURL: server.URL}, "text-embedding-3-large", 256)
	if _, err := emb.Embed(context.Background(), []string{"hi", "yo"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, `"dimensions":256`) {
		t.Errorf("expected dimensions in request, got %s", body)
	}
}

func TestEmbedder_Embed_Empty(t *testing.T) {
	emb := NewEmbedder(Config{APIKey: "k"}, "", 0)
	vecs, err := emb.Embed(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vecs != nil {
		t.Errorf("expected nil for empty input, got %v", vecs)
	}
}

func TestEmbedder_Embed_CountMismatch(t *testing.T) {
	var body string
	// one embedding returned for two inputs
	reply := `{"object":"list","model":"m","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`
	server := captureServer(t, reply, &body)
	defer server.Close()

	emb := NewEmbedder(Config{APIKey: "k", BaseURL: server.URL}, "", 0)
	if _, err := emb.Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Error("expected an error when the embedding count does not match the input count")
	}
}

func TestEmbedder_Embed_IndexOutOfRange(t *testing.T) {
	var body string
	// two embeddings (count matches two inputs) but a bogus index 5
	reply := `{"object":"list","model":"m","data":[{"object":"embedding","index":0,"embedding":[0.1]},{"object":"embedding","index":5,"embedding":[0.2]}],"usage":{"prompt_tokens":2,"total_tokens":2}}`
	server := captureServer(t, reply, &body)
	defer server.Close()

	emb := NewEmbedder(Config{APIKey: "k", BaseURL: server.URL}, "", 0)
	if _, err := emb.Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Error("expected an error when an embedding index is out of range")
	}
}

func TestEmbedder_Embed_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	emb := NewEmbedder(Config{APIKey: "k", BaseURL: server.URL, MaxRetries: 1}, "", 0)
	if _, err := emb.Embed(context.Background(), []string{"a"}); err == nil {
		t.Error("expected an error on HTTP failure")
	}
}

func TestEmbedder_Dimensions(t *testing.T) {
	cases := []struct {
		model string
		dims  int
		want  int
	}{
		{"", 0, 1536},                        // default model → small
		{"text-embedding-3-large", 0, 3072},  // large default
		{"text-embedding-3-large", 256, 256}, // explicit override
		{"some-custom-model", 0, 1536},       // unknown → fallback
	}
	for _, tc := range cases {
		emb := NewEmbedder(Config{APIKey: "k"}, tc.model, tc.dims)
		if got := emb.Dimensions(); got != tc.want {
			t.Errorf("Dimensions(model=%q,dims=%d) = %d, want %d", tc.model, tc.dims, got, tc.want)
		}
	}
}
