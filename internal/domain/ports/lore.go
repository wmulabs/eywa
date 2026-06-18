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

// LoreRange constrains a numeric metadata field to [Min, Max]. A nil bound is open on that side.
type LoreRange struct {
	Min *float64
	Max *float64
}

// LoreFilter constrains a vector search by structured chunk Metadata: Equals matches a field exactly,
// Ranges constrains numeric fields. Combined with similarity it makes the store a queryable database —
// retrieve by semantic relevance AND structured constraints (status, tenant, price, date, …).
type LoreFilter struct {
	Equals map[string]any
	Ranges map[string]LoreRange
}

// LoreSearchOptions carries the parameters of a filtered vector search.
type LoreSearchOptions struct {
	TopK     int
	MinScore float64
	Filter   *LoreFilter // nil = no metadata filter
}

// FilterableLoreStore is the optional metadata-filtering capability of a LoreStore. Adapters whose
// backend supports payload/metadata filters implement it; callers fall back to plain Search otherwise.
type FilterableLoreStore interface {
	SearchFiltered(ctx context.Context, loreID string, query []float32, opts LoreSearchOptions) ([]entities.LoreChunk, error)
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
