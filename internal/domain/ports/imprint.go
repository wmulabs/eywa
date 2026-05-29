package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// ImprintListOptions filters for the admin imprint list endpoint.
type ImprintListOptions struct {
	UserKey    string
	SpiritName string
	Category   string
	Limit      int
	Offset     int
}

type ImprintRepository interface {
	Save(ctx context.Context, imprint entities.Imprint) error
	GetByUserKey(ctx context.Context, userKey string, spiritName string) ([]entities.Imprint, error)
	// List returns imprints for the management API with optional filters.
	List(ctx context.Context, opts ImprintListOptions) ([]entities.Imprint, int64, error)
	Delete(ctx context.Context, id string) error
	Prune(ctx context.Context, userKey string, spiritName string, maxCount int) error
}

// ImprintHarvester loads long-term user facts into a Pulse for a specific Spirit.
// Implemented by ImprintScout; injected into SpiritScoutStep when configured.
type ImprintHarvester interface {
	Harvest(ctx context.Context, pulse *entities.Pulse, spiritName string) error
}
