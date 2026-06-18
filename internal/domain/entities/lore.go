package entities

import "time"

// LoreContextKnowledgeKey is the Pulse.Knowledge key under which LoreScout stores the formatted
// retrieved chunks. Shared so the reasoning loop can read it for citation grounding.
const LoreContextKnowledgeKey = "_lore_context"

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
}
