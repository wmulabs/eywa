package openai

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	eywa "github.com/wmulabs/eywa"
)

const defaultEmbeddingModel = "text-embedding-3-small"

// embeddingDimensions maps known OpenAI embedding models to their default vector size, used by
// Dimensions() when no explicit override is configured.
var embeddingDimensions = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,
}

const fallbackEmbeddingDimensions = 1536

// OpenAIEmbedder implements eywa.LoreEmbedder using OpenAI's embeddings API, so RAG works without the
// application supplying its own embedder. It also serves OpenAI-compatible endpoints via Config.BaseURL.
type OpenAIEmbedder struct {
	client     openai.Client
	model      string
	dimensions int // 0 = use the model's default
}

// NewEmbedder builds an embedder. model defaults to text-embedding-3-small; dimensions of 0 keeps the
// model's native size (text-embedding-3 models also accept a reduced size via the dimensions param).
func NewEmbedder(config Config, model string, dimensions int) *OpenAIEmbedder {
	if model == "" {
		model = defaultEmbeddingModel
	}
	return &OpenAIEmbedder{
		client:     createOpenAIClient(config),
		model:      model,
		dimensions: dimensions,
	}
}

var _ eywa.LoreEmbedder = (*OpenAIEmbedder)(nil)

func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(e.model),
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
	}
	if e.dimensions > 0 {
		params.Dimensions = openai.Int(int64(e.dimensions))
	}

	resp, err := e.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("openai embed: expected %d embeddings, got %d", len(texts), len(resp.Data))
	}

	out := make([][]float32, len(texts))
	for _, d := range resp.Data {
		idx := int(d.Index)
		if idx < 0 || idx >= len(out) {
			return nil, fmt.Errorf("openai embed: embedding index %d out of range", idx)
		}
		vec := make([]float32, len(d.Embedding))
		for i, f := range d.Embedding {
			vec[i] = float32(f)
		}
		out[idx] = vec
	}
	return out, nil
}

func (e *OpenAIEmbedder) Dimensions() int {
	if e.dimensions > 0 {
		return e.dimensions
	}
	if dim, ok := embeddingDimensions[e.model]; ok {
		return dim
	}
	return fallbackEmbeddingDimensions
}
