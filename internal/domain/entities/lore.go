package entities

import "time"

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
}

type LoreIngestion struct {
	LoreID   string
	Text     string
	FilePath string
	Metadata map[string]any
}
