package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type HTTPToolRepository interface {
	List(ctx context.Context) ([]entities.HTTPTool, error)
	FindBySpiritID(ctx context.Context, spiritID string) ([]entities.HTTPTool, error)
	FindByID(ctx context.Context, id string) (*entities.HTTPTool, error)
	Save(ctx context.Context, tool entities.HTTPTool) error
	Update(ctx context.Context, tool entities.HTTPTool) error
	Delete(ctx context.Context, id string) error
}
