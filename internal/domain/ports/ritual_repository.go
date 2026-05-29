package ports

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type RitualRepository interface {
	Save(ctx context.Context, task *entities.Ritual) error
	FindByID(ctx context.Context, id string) (*entities.Ritual, error)
	FindPendingByMemoryKey(ctx context.Context, memoryKey string, limit, offset int) ([]*entities.Ritual, error)
	CountByMemoryKey(ctx context.Context, memoryKey string) (int64, error)
	UpdateStatus(ctx context.Context, id string, status entities.RitualStatus, ts time.Time) error
	UpdateRunning(ctx context.Context, id string, ts time.Time) error
	UpdateFailed(ctx context.Context, id string, ts time.Time, reason string) error
	UpdateKeeperTaskID(ctx context.Context, id string, keeperTaskID string) error
}
