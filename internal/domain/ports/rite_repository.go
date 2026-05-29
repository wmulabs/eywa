package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type RiteListOptions struct {
	MemoryKey string
	Status    entities.RiteStatus
	Page      int
	Limit     int
}

type RiteRepository interface {
	Create(ctx context.Context, rite *entities.Rite) error
	FindByID(ctx context.Context, id string) (*entities.Rite, error)
	List(ctx context.Context, opts RiteListOptions) ([]*entities.Rite, int64, error)
	Decide(ctx context.Context, id, operatorID string, status entities.RiteStatus) error
}
