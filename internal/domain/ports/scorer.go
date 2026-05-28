package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type Scorer interface {
	GetName() string
	Score(ctx context.Context, tc entities.TrialCase, result entities.Response) (float64, error)
}
