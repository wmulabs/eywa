// Package pinecone provides a LoreStore adapter backed by Pinecone managed vector database.
//
// Usage:
//
//	store, err := pinecone.NewLoreStore(ctx, pinecone.Config{
//	    APIKey:    os.Getenv("PINECONE_API_KEY"),
//	    IndexName: "eywa-lore",
//	})
//	weave, _ := eywa.NewWeaveBuilder(ctx).
//	    WithLoreStore(store).
//	    Build()
//
// Each Lore knowledge base is isolated by using lore_id as the Pinecone namespace.
// The Pinecone index must be created externally with the correct dimension before use.
package pinecone

import (
	"context"
	"fmt"

	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
	eywa "github.com/wmulabs/eywa"
	"google.golang.org/protobuf/types/known/structpb"
)

// compile-time interface check
var _ eywa.LoreStore = (*LoreStore)(nil)

// Config holds Pinecone connection configuration.
type Config struct {
	// APIKey is the Pinecone API key.
	APIKey string
	// IndexName is the name of the existing Pinecone index.
	// The index must be created externally with the correct embedding dimension.
	IndexName string
}

// LoreStore implements eywa.LoreStore using Pinecone.
// Each Lore knowledge base maps to a distinct Pinecone namespace (lore_id),
// so a single Pinecone index serves all Lore documents in the Weave.
type LoreStore struct {
	client    *pc.Client
	indexName string
	indexHost string
}

// NewLoreStore creates a LoreStore and resolves the Pinecone index host.
// The Pinecone index must already exist with the correct embedding dimension.
func NewLoreStore(ctx context.Context, cfg Config) (*LoreStore, error) {
	client, err := pc.NewClient(pc.NewClientParams{ApiKey: cfg.APIKey})
	if err != nil {
		return nil, fmt.Errorf("pinecone: new client: %w", err)
	}

	idx, err := client.DescribeIndex(ctx, cfg.IndexName)
	if err != nil {
		return nil, fmt.Errorf("pinecone: describe index %q: %w", cfg.IndexName, err)
	}

	return &LoreStore{
		client:    client,
		indexName: cfg.IndexName,
		indexHost: idx.Host,
	}, nil
}

// Upsert inserts or replaces LoreChunks. Each chunk is upserted into a namespace
// equal to its lore_id, providing per-knowledge-base isolation.
// Chunks with no embedding are skipped.
func (s *LoreStore) Upsert(ctx context.Context, chunks []eywa.LoreChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Group chunks by lore_id for namespace-aware batch upsert.
	byLore := make(map[string][]eywa.LoreChunk)
	for _, c := range chunks {
		if len(c.Embedding) > 0 {
			byLore[c.LoreID] = append(byLore[c.LoreID], c)
		}
	}

	for loreID, group := range byLore {
		conn, err := s.indexConn(loreID)
		if err != nil {
			return err
		}

		vectors := make([]*pc.Vector, len(group))
		for i, c := range group {
			meta, err := structpb.NewStruct(map[string]any{
				"content": c.Content,
				"lore_id": c.LoreID,
			})
			if err != nil {
				return fmt.Errorf("pinecone: build metadata: %w", err)
			}
			for k, v := range c.Metadata {
				meta.Fields[k] = structpb.NewStringValue(fmt.Sprintf("%v", v))
			}
			emb := c.Embedding
			vectors[i] = &pc.Vector{
				Id:       c.ID,
				Values:   &emb,
				Metadata: meta,
			}
		}

		if _, err := conn.UpsertVectors(ctx, vectors); err != nil {
			return fmt.Errorf("pinecone: upsert namespace %q: %w", loreID, err)
		}
	}
	return nil
}

// Search returns at most topK chunks from loreID with score >= minScore.
// The Pinecone namespace for the query is set to loreID.
func (s *LoreStore) Search(ctx context.Context, loreID string, query []float32, topK int, minScore float64) ([]eywa.LoreChunk, error) {
	if topK <= 0 {
		topK = 5
	}

	conn, err := s.indexConn(loreID)
	if err != nil {
		return nil, err
	}

	res, err := conn.QueryByVectorValues(ctx, &pc.QueryByVectorValuesRequest{
		Vector:          query,
		TopK:            uint32(topK),
		IncludeValues:   false,
		IncludeMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("pinecone: search: %w", err)
	}

	chunks := make([]eywa.LoreChunk, 0, len(res.Matches))
	for _, m := range res.Matches {
		if m.Vector == nil || float64(m.Score) < minScore {
			continue
		}
		c := eywa.LoreChunk{
			ID:     m.Vector.Id,
			LoreID: loreID,
		}
		if m.Vector.Metadata != nil {
			if v, ok := m.Vector.Metadata.Fields["content"]; ok {
				c.Content = v.GetStringValue()
			}
			c.Metadata = make(map[string]any)
			for k, v := range m.Vector.Metadata.Fields {
				if k != "content" && k != "lore_id" {
					c.Metadata[k] = v.GetStringValue()
				}
			}
		}
		chunks = append(chunks, c)
	}
	return chunks, nil
}

// Delete removes all vectors in the Pinecone namespace equal to loreID,
// effectively deleting all chunks for that Lore knowledge base.
func (s *LoreStore) Delete(ctx context.Context, loreID string) error {
	conn, err := s.indexConn(loreID)
	if err != nil {
		return err
	}
	if err := conn.DeleteAllVectorsInNamespace(ctx); err != nil {
		return fmt.Errorf("pinecone: delete namespace %q: %w", loreID, err)
	}
	return nil
}

// indexConn returns an IndexConnection scoped to the given namespace.
func (s *LoreStore) indexConn(namespace string) (*pc.IndexConnection, error) {
	conn, err := s.client.Index(pc.NewIndexConnParams{
		Host:      s.indexHost,
		Namespace: namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("pinecone: index connection: %w", err)
	}
	return conn, nil
}
