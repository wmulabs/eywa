package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type LoreRepository interface {
	Create(ctx context.Context, lore entities.Lore) error
	GetByID(ctx context.Context, id string) (entities.Lore, error)
	GetByName(ctx context.Context, name string) (entities.Lore, error)
	GetBySpiritID(ctx context.Context, spiritID string) ([]entities.Lore, error)
	GetByIDs(ctx context.Context, ids []string) ([]entities.Lore, error)
	Update(ctx context.Context, lore entities.Lore) error
	Delete(ctx context.Context, id string) error
}

type LoreStore interface {
	Upsert(ctx context.Context, chunks []entities.LoreChunk) error
	Search(ctx context.Context, loreID string, query []float32, topK int, minScore float64) ([]entities.LoreChunk, error)
	Delete(ctx context.Context, loreID string) error
}

type LoreEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

// LoreHarvester enriches a Pulse with relevant Lore chunks via vector search.
// Implemented by LoreScout; injected into SpiritScoutStep when configured.
type LoreHarvester interface {
	Harvest(ctx context.Context, pulse *entities.Pulse, loreIDs []string) error
}
