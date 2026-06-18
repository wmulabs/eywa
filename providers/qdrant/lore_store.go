// Package qdrant provides a LoreStore adapter backed by Qdrant vector database.
//
// Usage:
//
//	store, err := qdrant.NewLoreStore("localhost", 6334, "eywa_lore", 1536)
//	// or for Qdrant Cloud:
//	store, err := qdrant.NewLoreStoreWithConfig(qdrant.Config{
//	    Host:       "xyz.cloud.qdrant.io",
//	    Port:       6334,
//	    Collection: "eywa_lore",
//	    Dim:        1536,
//	    APIKey:     os.Getenv("QDRANT_API_KEY"),
//	    UseTLS:     true,
//	})
//
//	weave, _ := eywa.NewWeaveBuilder(ctx).
//	    WithLoreStore(store).
//	    Build()
//
// The Qdrant collection is created automatically on the first Upsert call.
// Eywa chunk IDs (non-UUID strings) are hashed to uint64 for Qdrant point IDs;
// the original string ID is stored in the point payload for reconstruction.
package qdrant

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"time"

	qclient "github.com/qdrant/go-client/qdrant"
	eywa "github.com/wmulabs/eywa"
)

// compile-time interface checks
var (
	_ eywa.LoreStore           = (*LoreStore)(nil)
	_ eywa.FilterableLoreStore = (*LoreStore)(nil)
)

// qdrantClient is the subset of *qclient.Client the store uses, so the query path can be unit-tested
// against a mock.
type qdrantClient interface {
	Query(ctx context.Context, request *qclient.QueryPoints) ([]*qclient.ScoredPoint, error)
	Upsert(ctx context.Context, request *qclient.UpsertPoints) (*qclient.UpdateResult, error)
	Delete(ctx context.Context, request *qclient.DeletePoints) (*qclient.UpdateResult, error)
	CollectionExists(ctx context.Context, collectionName string) (bool, error)
	CreateCollection(ctx context.Context, request *qclient.CreateCollection) error
}

// Config holds Qdrant connection configuration.
type Config struct {
	// Host is the Qdrant server hostname (default "localhost").
	Host string
	// Port is the Qdrant gRPC port (default 6334).
	Port int
	// Collection is the Qdrant collection name to use.
	Collection string
	// Dim is the embedding vector dimension; must match the configured LoreEmbedder.
	Dim uint64
	// APIKey is optional; required for Qdrant Cloud.
	APIKey string
	// UseTLS enables TLS transport; required for Qdrant Cloud.
	UseTLS bool
}

// LoreStore implements eywa.LoreStore using Qdrant.
// Similarity search uses cosine distance; lore_id is stored as a payload field
// so multiple Lore knowledge bases can share a single collection.
type LoreStore struct {
	client     qdrantClient
	collection string
	dim        uint64
}

// NewLoreStore creates a LoreStore against a local or self-hosted Qdrant instance.
// dim is the embedding vector dimension; collection is created on first Upsert.
func NewLoreStore(host string, port int, collection string, dim uint64) (*LoreStore, error) {
	return NewLoreStoreWithConfig(Config{
		Host:       host,
		Port:       port,
		Collection: collection,
		Dim:        dim,
	})
}

// NewLoreStoreWithConfig creates a LoreStore from a full Config.
func NewLoreStoreWithConfig(cfg Config) (*LoreStore, error) {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 6334
	}

	client, err := qclient.NewClient(&qclient.Config{
		Host:   cfg.Host,
		Port:   cfg.Port,
		APIKey: cfg.APIKey,
		UseTLS: cfg.UseTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant: new client: %w", err)
	}

	return &LoreStore{
		client:     client,
		collection: cfg.Collection,
		dim:        cfg.Dim,
	}, nil
}

// Upsert inserts or replaces LoreChunks in the collection.
// The collection is created automatically if it does not exist.
func (s *LoreStore) Upsert(ctx context.Context, chunks []eywa.LoreChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Marshal all metadata before touching the client so that serialization
	// errors surface early without a network round-trip.
	type pointData struct {
		chunk    eywa.LoreChunk
		metaJSON []byte
	}
	valid := make([]pointData, 0, len(chunks))
	for _, c := range chunks {
		if len(c.Embedding) == 0 {
			continue
		}
		metaJSON, err := json.Marshal(c.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata for chunk %s: %w", c.ID, err)
		}
		valid = append(valid, pointData{chunk: c, metaJSON: metaJSON})
	}

	if len(valid) == 0 {
		return nil
	}

	if err := s.ensureCollection(ctx); err != nil {
		return err
	}

	points := make([]*qclient.PointStruct, 0, len(valid))
	for _, pd := range valid {
		c := pd.chunk
		metaJSON := pd.metaJSON
		points = append(points, &qclient.PointStruct{
			Id:      qclient.NewIDNum(hashID(c.ID)),
			Vectors: qclient.NewVectors(c.Embedding...),
			Payload: map[string]*qclient.Value{
				"chunk_id":   strVal(c.ID),
				"lore_id":    strVal(c.LoreID),
				"content":    strVal(c.Content),
				"metadata":   strVal(string(metaJSON)),
				"created_at": strVal(time.Now().UTC().Format(time.RFC3339)),
			},
		})
	}

	_, err := s.client.Upsert(ctx, &qclient.UpsertPoints{
		CollectionName: s.collection,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("qdrant: upsert: %w", err)
	}
	return nil
}

// Search returns at most topK chunks from loreID with cosine similarity >= minScore.
func (s *LoreStore) Search(ctx context.Context, loreID string, query []float32, topK int, minScore float64) ([]eywa.LoreChunk, error) {
	return s.SearchFiltered(ctx, loreID, query, eywa.LoreSearchOptions{TopK: topK, MinScore: minScore})
}

// SearchFiltered runs a vector search constrained by chunk metadata via Qdrant payload conditions.
func (s *LoreStore) SearchFiltered(ctx context.Context, loreID string, query []float32, opts eywa.LoreSearchOptions) ([]eywa.LoreChunk, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	limit := uint64(topK)
	scoreThreshold := float32(opts.MinScore)

	results, err := s.client.Query(ctx, &qclient.QueryPoints{
		CollectionName: s.collection,
		Query:          qclient.NewQuery(query...),
		Filter:         &qclient.Filter{Must: qdrantConditions(loreID, opts.Filter)},
		Limit:          &limit,
		ScoreThreshold: &scoreThreshold,
		WithPayload:    qclient.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant: search: %w", err)
	}

	chunks := make([]eywa.LoreChunk, 0, len(results))
	for _, r := range results {
		c, err := chunkFromPayload(r.Payload)
		if err != nil {
			continue
		}
		c.Score = float64(r.GetScore())
		chunks = append(chunks, c)
	}
	return chunks, nil
}

// qdrantConditions builds the payload Must-conditions: always the lore_id match, plus metadata
// equality (typed) and numeric ranges from the filter.
func qdrantConditions(loreID string, filter *eywa.LoreFilter) []*qclient.Condition {
	conditions := []*qclient.Condition{qclient.NewMatchKeyword("lore_id", loreID)}
	if filter == nil {
		return conditions
	}

	for _, field := range sortedKeys(filter.Equals) {
		switch v := filter.Equals[field].(type) {
		case string:
			conditions = append(conditions, qclient.NewMatchKeyword(field, v))
		case bool:
			conditions = append(conditions, qclient.NewMatchBool(field, v))
		case int:
			conditions = append(conditions, qclient.NewMatchInt(field, int64(v)))
		case int64:
			conditions = append(conditions, qclient.NewMatchInt(field, v))
		case float64:
			conditions = append(conditions, qclient.NewMatchInt(field, int64(v)))
		}
	}

	for _, field := range sortedRangeKeys(filter.Ranges) {
		r := filter.Ranges[field]
		conditions = append(conditions, qclient.NewRange(field, &qclient.Range{Gte: r.Min, Lte: r.Max}))
	}

	return conditions
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedRangeKeys(m map[string]eywa.LoreRange) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Delete removes all chunks belonging to loreID from the collection.
func (s *LoreStore) Delete(ctx context.Context, loreID string) error {
	_, err := s.client.Delete(ctx, &qclient.DeletePoints{
		CollectionName: s.collection,
		Points: &qclient.PointsSelector{
			PointsSelectorOneOf: &qclient.PointsSelector_Filter{
				Filter: &qclient.Filter{
					Must: []*qclient.Condition{
						qclient.NewMatchKeyword("lore_id", loreID),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("qdrant: delete: %w", err)
	}
	return nil
}

// ensureCollection creates the collection if it does not already exist.
func (s *LoreStore) ensureCollection(ctx context.Context) error {
	exists, err := s.client.CollectionExists(ctx, s.collection)
	if err != nil {
		return fmt.Errorf("qdrant: check collection: %w", err)
	}
	if exists {
		return nil
	}

	err = s.client.CreateCollection(ctx, &qclient.CreateCollection{
		CollectionName: s.collection,
		VectorsConfig: qclient.NewVectorsConfig(&qclient.VectorParams{
			Size:     s.dim,
			Distance: qclient.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("qdrant: create collection: %w", err)
	}
	return nil
}

// hashID maps an arbitrary string ID to a uint64 Qdrant point ID via FNV-1a.
// The original string ID is preserved in the payload under "chunk_id".
//
// Known limitation: FNV-1a has 64-bit output space (~18.4 quintillion values).
// Hash collisions are theoretically possible in very large knowledge bases
// (birthday paradox: ~50% probability at ~4.3 billion chunks). A collision
// causes one chunk to silently overwrite another at the same Qdrant point ID.
// The "chunk_id" payload property can be used to detect collisions post-hoc
// by comparing it to the ID originally passed to Upsert.
func hashID(id string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	return h.Sum64()
}

// strVal creates a Qdrant string payload value.
func strVal(s string) *qclient.Value {
	return &qclient.Value{Kind: &qclient.Value_StringValue{StringValue: s}}
}

// chunkFromPayload reconstructs a LoreChunk from Qdrant point payload.
func chunkFromPayload(payload map[string]*qclient.Value) (eywa.LoreChunk, error) {
	get := func(key string) string {
		if v, ok := payload[key]; ok {
			return v.GetStringValue()
		}
		return ""
	}

	id := get("chunk_id")
	if id == "" {
		return eywa.LoreChunk{}, fmt.Errorf("qdrant: missing chunk_id in payload")
	}

	c := eywa.LoreChunk{
		ID:      id,
		LoreID:  get("lore_id"),
		Content: get("content"),
	}

	if raw := get("metadata"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &c.Metadata)
	}
	if ts := get("created_at"); ts != "" {
		c.CreatedAt, _ = time.Parse(time.RFC3339, ts)
	}
	return c, nil
}
