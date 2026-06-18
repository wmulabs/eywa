// Package pgvector provides a LoreStore adapter backed by PostgreSQL + pgvector extension.
//
// Usage:
//
//	pool, _ := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
//	store, err := pgvector.NewLoreStore(ctx, pool, 1536) // dim must match your embedder
//	weave, _ := eywa.NewWeaveBuilder(ctx).
//	    WithLoreStore(store).
//	    Build()
//
// The adapter auto-creates the eywa_lore_chunks table and a lore_id index on startup.
// A vector cosine index (ivfflat) can be added manually after sufficient data is loaded:
//
//	CREATE INDEX ON eywa_lore_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
package pgvector

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	eywa "github.com/wmulabs/eywa"
)

// compile-time interface checks
var (
	_ eywa.LoreStore           = (*LoreStore)(nil)
	_ eywa.FilterableLoreStore = (*LoreStore)(nil)
)

// LoreStore implements eywa.LoreStore using PostgreSQL with the pgvector extension.
// Vectors are stored as the pgvector `vector` type; similarity search uses cosine distance.
type LoreStore struct {
	pool *pgxpool.Pool
	dim  int
}

// NewLoreStore creates a LoreStore and runs schema migration.
// pool must point at a PostgreSQL database with the pgvector extension installed.
// dim is the embedding vector dimension and must match the configured LoreEmbedder.
func NewLoreStore(ctx context.Context, pool *pgxpool.Pool, dim int) (*LoreStore, error) {
	s := &LoreStore{pool: pool, dim: dim}
	if err := s.migrate(ctx); err != nil {
		return nil, fmt.Errorf("pgvector: migrate: %w", err)
	}
	return s, nil
}

// migrate creates the extension, table, and basic index if they do not exist.
func (s *LoreStore) migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`
		CREATE EXTENSION IF NOT EXISTS vector;

		CREATE TABLE IF NOT EXISTS eywa_lore_chunks (
			id         TEXT        PRIMARY KEY,
			lore_id    TEXT        NOT NULL,
			content    TEXT        NOT NULL,
			embedding  vector(%d),
			metadata   JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE INDEX IF NOT EXISTS idx_eywa_lore_chunks_lore_id
			ON eywa_lore_chunks (lore_id);
	`, s.dim))
	if err != nil {
		return fmt.Errorf("migrate lore schema: %w", err)
	}
	return nil
}

// Upsert inserts or replaces LoreChunks. Chunks with no embedding are still stored
// and will never surface in Search results.
func (s *LoreStore) Upsert(ctx context.Context, chunks []eywa.LoreChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	const sql = `
		INSERT INTO eywa_lore_chunks (id, lore_id, content, embedding, metadata, created_at)
		VALUES ($1, $2, $3, $4::vector, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			content    = EXCLUDED.content,
			embedding  = EXCLUDED.embedding,
			metadata   = EXCLUDED.metadata`

	now := time.Now().UTC()
	batch := &pgx.Batch{}
	for _, c := range chunks {
		meta, err := json.Marshal(c.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata for chunk %s: %w", c.ID, err)
		}
		batch.Queue(sql, c.ID, c.LoreID, c.Content, formatVector(c.Embedding), meta, now)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close() //nolint:errcheck

	for range chunks {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("delete lore chunks: %w", err)
		}
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("close batch results: %w", err)
	}
	return nil
}

// Search returns at most topK chunks from loreID whose cosine similarity to query is
// >= minScore (0–1). Results are ordered by descending similarity.
func (s *LoreStore) Search(ctx context.Context, loreID string, query []float32, topK int, minScore float64) ([]eywa.LoreChunk, error) {
	return s.SearchFiltered(ctx, loreID, query, eywa.LoreSearchOptions{TopK: topK, MinScore: minScore})
}

// SearchFiltered runs a vector search constrained by chunk metadata: Equals via jsonb containment,
// numeric Ranges via casts. Filter field names are bound as parameters, never interpolated.
func (s *LoreStore) SearchFiltered(ctx context.Context, loreID string, query []float32, opts eywa.LoreSearchOptions) ([]eywa.LoreChunk, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}
	if len(query) == 0 {
		return nil, fmt.Errorf("pgvector: search: empty query vector")
	}

	// pgvector cosine distance: 0 = identical, 2 = opposite
	// similarity = 1 − distance  →  distance threshold = 1 − minScore
	distThreshold := 1.0 - opts.MinScore

	where := "lore_id = $2 AND embedding <=> $1::vector <= $3"
	args := []any{formatVector(query), loreID, distThreshold}

	where, args, err := appendLoreFilter(where, args, opts.Filter)
	if err != nil {
		return nil, err
	}

	args = append(args, topK)
	sql := fmt.Sprintf(`
		SELECT id, lore_id, content, metadata, created_at,
		       1 - (embedding <=> $1::vector) AS score
		FROM eywa_lore_chunks
		WHERE %s
		ORDER BY embedding <=> $1::vector
		LIMIT $%d`, where, len(args))

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("pgvector: search: %w", err)
	}
	defer rows.Close()

	var result []eywa.LoreChunk
	for rows.Next() {
		var c eywa.LoreChunk
		var metaRaw []byte
		var score float64
		if err := rows.Scan(&c.ID, &c.LoreID, &c.Content, &metaRaw, &c.CreatedAt, &score); err != nil {
			return nil, fmt.Errorf("pgvector: scan chunk: %w", err)
		}
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &c.Metadata)
		}
		c.Score = score
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan lore search results: %w", err)
	}
	return result, nil
}

// appendLoreFilter extends the WHERE clause and args with metadata constraints. Equals becomes a
// single jsonb containment check; each numeric Range bound becomes a cast comparison. Field names are
// passed as parameters (metadata ->> $n) so they are never interpolated into SQL.
func appendLoreFilter(where string, args []any, filter *eywa.LoreFilter) (string, []any, error) {
	if filter == nil {
		return where, args, nil
	}

	if len(filter.Equals) > 0 {
		raw, err := json.Marshal(filter.Equals)
		if err != nil {
			return where, args, fmt.Errorf("pgvector: marshal filter equals: %w", err)
		}
		args = append(args, string(raw))
		where += fmt.Sprintf(" AND metadata @> $%d::jsonb", len(args))
	}

	for _, field := range sortedRangeFields(filter.Ranges) {
		r := filter.Ranges[field]
		if r.Min != nil {
			args = append(args, field)
			fieldIdx := len(args)
			args = append(args, *r.Min)
			where += fmt.Sprintf(" AND (metadata ->> $%d)::numeric >= $%d", fieldIdx, len(args))
		}
		if r.Max != nil {
			args = append(args, field)
			fieldIdx := len(args)
			args = append(args, *r.Max)
			where += fmt.Sprintf(" AND (metadata ->> $%d)::numeric <= $%d", fieldIdx, len(args))
		}
	}

	return where, args, nil
}

func sortedRangeFields(ranges map[string]eywa.LoreRange) []string {
	fields := make([]string, 0, len(ranges))
	for f := range ranges {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	return fields
}

// Delete removes all chunks belonging to loreID.
func (s *LoreStore) Delete(ctx context.Context, loreID string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM eywa_lore_chunks WHERE lore_id = $1", loreID)
	if err != nil {
		return fmt.Errorf("pgvector: delete: %w", err)
	}
	return nil
}

// formatVector encodes a []float32 as the PostgreSQL vector literal format "[x,y,z,...]".
// This avoids requiring pgvector-go type registration on the pool.
func formatVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	b := make([]byte, 0, 2+len(v)*12)
	b = append(b, '[')
	for i, f := range v {
		if i > 0 {
			b = append(b, ',')
		}
		b = strconv.AppendFloat(b, float64(f), 'f', -1, 32)
	}
	b = append(b, ']')
	return string(b)
}

// vectorFromString parses a pgvector literal "[x,y,z]" back to []float32.
// Used for testing round-trips.
func vectorFromString(s string) ([]float32, error) {
	s = strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(s), "]"), "[")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]float32, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("parse score: %w", err)
		}
		out[i] = float32(f)
	}
	return out, nil
}
