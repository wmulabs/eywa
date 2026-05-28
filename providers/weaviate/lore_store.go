// Package weaviate provides a LoreStore adapter backed by Weaviate vector database.
//
// Usage:
//
//	store, err := weaviate.NewLoreStore(ctx, weaviate.Config{
//	    Host:      "localhost:8080",
//	    ClassName: "LoreChunk",
//	})
//	// or with API key (Weaviate Cloud):
//	store, err := weaviate.NewLoreStore(ctx, weaviate.Config{
//	    Host:      "xyz.weaviate.network",
//	    APIKey:    os.Getenv("WEAVIATE_API_KEY"),
//	    ClassName: "LoreChunk",
//	    UseTLS:    true,
//	})
//	weave, _ := eywa.NewWeaveBuilder(ctx).
//	    WithLoreStore(store).
//	    Build()
//
// The Weaviate class is created automatically on NewLoreStore if it does not exist.
// Similarity search uses cosine distance via the near_vector operator.
package weaviate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
	eywa "github.com/wmulabs/eywa"
)

// compile-time interface check
var _ eywa.LoreStore = (*LoreStore)(nil)

// Config holds Weaviate connection configuration.
type Config struct {
	// Host is the Weaviate server address (e.g. "localhost:8080" or "xyz.weaviate.network").
	Host string
	// ClassName is the Weaviate class used to store LoreChunks (e.g. "LoreChunk").
	// The class is created automatically if it does not exist.
	ClassName string
	// APIKey is optional; required for Weaviate Cloud.
	APIKey string
	// UseTLS enables HTTPS transport; set true for Weaviate Cloud.
	UseTLS bool
}

// LoreStore implements eywa.LoreStore using Weaviate.
// All chunks are stored in a single Weaviate class; lore_id is a filterable property.
type LoreStore struct {
	client    *weaviate.Client
	className string
}

// NewLoreStore creates a LoreStore and ensures the Weaviate class exists.
func NewLoreStore(ctx context.Context, cfg Config) (*LoreStore, error) {
	wcfg := weaviate.Config{
		Host:   cfg.Host,
		Scheme: "http",
	}
	if cfg.UseTLS {
		wcfg.Scheme = "https"
	}
	if cfg.APIKey != "" {
		wcfg.AuthConfig = auth.ApiKey{Value: cfg.APIKey}
	}

	client, err := weaviate.NewClient(wcfg)
	if err != nil {
		return nil, fmt.Errorf("weaviate: new client: %w", err)
	}

	s := &LoreStore{client: client, className: cfg.ClassName}
	if err := s.ensureClass(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// ensureClass creates the Weaviate class with the required properties if absent.
func (s *LoreStore) ensureClass(ctx context.Context) error {
	exists, err := s.client.Schema().ClassExistenceChecker().WithClassName(s.className).Do(ctx)
	if err != nil {
		return fmt.Errorf("weaviate: check class: %w", err)
	}
	if exists {
		return nil
	}

	class := &models.Class{
		Class:       s.className,
		Description: "Eywa Lore chunks for RAG retrieval",
		VectorIndexConfig: map[string]any{
			"distance": "cosine",
		},
		Properties: []*models.Property{
			{Name: "chunk_id", DataType: []string{"text"}, Description: "Original Eywa chunk ID"},
			{Name: "lore_id", DataType: []string{"text"}, Description: "Lore knowledge base ID"},
			{Name: "content", DataType: []string{"text"}, Description: "Chunk text content"},
			{Name: "metadata", DataType: []string{"text"}, Description: "JSON-encoded metadata"},
			{Name: "created_at", DataType: []string{"text"}, Description: "ISO 8601 creation timestamp"},
		},
	}

	if err := s.client.Schema().ClassCreator().WithClass(class).Do(ctx); err != nil {
		return fmt.Errorf("weaviate: create class: %w", err)
	}
	return nil
}

// Upsert inserts or replaces LoreChunks. Chunks with no embedding are skipped.
//
// Non-atomic behaviour: Weaviate does not support native upsert by arbitrary string ID.
// Each chunk is deleted by chunk_id then re-inserted. If the delete succeeds but the
// insert fails, one recovery attempt is made. If recovery also fails the chunk may be
// permanently absent from the knowledge base until the next successful Upsert call.
func (s *LoreStore) Upsert(ctx context.Context, chunks []eywa.LoreChunk) error {
	for _, c := range chunks {
		if len(c.Embedding) == 0 {
			continue
		}
		if err := s.upsertOne(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func (s *LoreStore) upsertOne(ctx context.Context, c eywa.LoreChunk) error {
	meta, err := json.Marshal(c.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata for chunk %s: %w", c.ID, err)
	}
	props := map[string]any{
		"chunk_id":   c.ID,
		"lore_id":    c.LoreID,
		"content":    c.Content,
		"metadata":   string(meta),
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}

	_ = s.deleteByChunkID(ctx, c.ID)

	insert := func() error {
		_, err := s.client.Data().Creator().
			WithClassName(s.className).
			WithProperties(props).
			WithVector(c.Embedding).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("create weaviate object: %w", err)
		}
		return nil
	}

	if err := insert(); err != nil {
		// Recovery attempt: the delete already succeeded; try once more before
		// returning an error that would leave the chunk absent.
		if retryErr := insert(); retryErr != nil {
			return fmt.Errorf("weaviate: upsert chunk %q lost after delete (first: %v): %w", c.ID, err, retryErr)
		}
	}
	return nil
}

// Search returns at most topK chunks from loreID with certainty >= minScore.
func (s *LoreStore) Search(ctx context.Context, loreID string, query []float32, topK int, minScore float64) ([]eywa.LoreChunk, error) {
	if topK <= 0 {
		topK = 5
	}

	fields := []graphql.Field{
		{Name: "chunk_id"},
		{Name: "lore_id"},
		{Name: "content"},
		{Name: "metadata"},
		{Name: "created_at"},
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "id"},
			{Name: "certainty"},
		}},
	}

	nearVec := s.client.GraphQL().NearVectorArgBuilder().
		WithVector(query).
		WithCertainty(float32(minScore))

	where := filters.Where().
		WithPath([]string{"lore_id"}).
		WithOperator(filters.Equal).
		WithValueText(loreID)

	res, err := s.client.GraphQL().Get().
		WithClassName(s.className).
		WithFields(fields...).
		WithNearVector(nearVec).
		WithWhere(where).
		WithLimit(topK).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("weaviate: search: %w", err)
	}
	if len(res.Errors) > 0 {
		return nil, fmt.Errorf("weaviate: search: %s", res.Errors[0].Message)
	}

	return parseSearchResults(res.Data, s.className)
}

// Delete removes all chunks belonging to loreID.
func (s *LoreStore) Delete(ctx context.Context, loreID string) error {
	where := filters.Where().
		WithPath([]string{"lore_id"}).
		WithOperator(filters.Equal).
		WithValueText(loreID)

	res, err := s.client.Batch().ObjectsBatchDeleter().
		WithClassName(s.className).
		WithWhere(where).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("weaviate: delete lore_id %q: %w", loreID, err)
	}
	if res.Results != nil && res.Results.Failed > 0 {
		return fmt.Errorf("weaviate: delete: %d objects failed", res.Results.Failed)
	}
	return nil
}

// deleteByChunkID deletes an object matching chunk_id property (best-effort, no error on miss).
func (s *LoreStore) deleteByChunkID(ctx context.Context, chunkID string) error {
	where := filters.Where().
		WithPath([]string{"chunk_id"}).
		WithOperator(filters.Equal).
		WithValueText(chunkID)

	_, err := s.client.Batch().ObjectsBatchDeleter().
		WithClassName(s.className).
		WithWhere(where).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("delete weaviate objects: %w", err)
	}
	return nil
}

// parseSearchResults extracts LoreChunks from the Weaviate GraphQL Get response.
func parseSearchResults(data map[string]models.JSONObject, className string) ([]eywa.LoreChunk, error) {
	getObj, ok := data["Get"]
	if !ok {
		return nil, nil
	}
	get, _ := getObj.(map[string]any)
	if get == nil {
		return nil, nil
	}
	items, _ := get[className].([]any)

	chunks := make([]eywa.LoreChunk, 0, len(items))
	for _, item := range items {
		obj, _ := item.(map[string]any)
		if obj == nil {
			continue
		}
		c := eywa.LoreChunk{
			ID:      strField(obj, "chunk_id"),
			LoreID:  strField(obj, "lore_id"),
			Content: strField(obj, "content"),
		}
		if raw := strField(obj, "metadata"); raw != "" {
			_ = json.Unmarshal([]byte(raw), &c.Metadata)
		}
		if ts := strField(obj, "created_at"); ts != "" {
			c.CreatedAt, _ = time.Parse(time.RFC3339, ts)
		}
		chunks = append(chunks, c)
	}
	return chunks, nil
}

// strField safely extracts a string from a map[string]any.
func strField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
