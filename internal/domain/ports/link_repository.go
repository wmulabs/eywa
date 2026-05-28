package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type LinkRepository interface {
	FindAll(ctx context.Context) ([]*entities.Link, error)
	FindByKey(ctx context.Context, eventType string) (*entities.Link, error)
	Save(ctx context.Context, link *entities.Link) error
	Delete(ctx context.Context, eventType string) error
}
