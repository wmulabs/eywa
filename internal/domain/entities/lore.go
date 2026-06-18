package entities

import "time"

// LoreContextKnowledgeKey is the Pulse.Knowledge key under which LoreScout stores the formatted
// retrieved chunks. Shared so the reasoning loop can read it for citation grounding.
const LoreContextKnowledgeKey = "_lore_context"

// LoreDocumentIDKey is the chunk Metadata key holding the source record's stable ID when a document
// is ingested with a DocumentID. It groups a record's chunks and enables distinct-object retrieval.
const LoreDocumentIDKey = "_document_id"

type Lore struct {
	ID          string    `bson:"_id" json:"id"`
	Name        string    `bson:"name" json:"name"`
	Description string    `bson:"description,omitempty" json:"description,omitempty"`
	SpiritIDs   []string  `bson:"spirit_ids,omitempty" json:"spirit_ids,omitempty"`
	ChunkSize   int       `bson:"chunk_size,omitempty" json:"chunk_size,omitempty"`
	Overlap     int       `bson:"overlap,omitempty" json:"overlap,omitempty"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
}

type LoreChunk struct {
	ID        string         `bson:"_id" json:"id"`
	LoreID    string         `bson:"lore_id" json:"lore_id"`
	Content   string         `bson:"content" json:"content"`
	Embedding []float32      `bson:"embedding,omitempty" json:"embedding,omitempty"`
	Metadata  map[string]any `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt time.Time      `bson:"created_at" json:"created_at"`

	// Score is the similarity of this chunk to the search query (1 = identical), set on search results.
	// Zero on stored/ingested chunks.
	Score float64 `bson:"-" json:"score,omitempty"`
}

type LoreIngestion struct {
	LoreID   string
	Text     string
	FilePath string
	Metadata map[string]any

	// DocumentID is a stable, caller-supplied ID for the source record. When set, re-ingesting the
	// same DocumentID upserts in place (no duplicates), the ID is stored in chunk metadata under
	// LoreDocumentIDKey, and a single-chunk ingest uses DocumentID as the chunk ID. Empty = legacy
	// positional IDs (loreID_i).
	DocumentID string

	// NoChunk indexes the whole Text as one vector (record mode) instead of splitting it — use it when
	// one object should be one searchable record (then top-K results are distinct objects).
	NoChunk bool
}
