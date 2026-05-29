package ports

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type SessionListOptions struct {
	MemoryKey string // non-empty = filter to this specific session
	DateFrom  *time.Time
	DateTo    *time.Time
	Page      int
	Limit     int
}

type SessionSummary struct {
	MemoryKey      string    `json:"memory_key"`
	LastSpiritName string    `json:"last_spirit_name"`
	MessageCount   int64     `json:"message_count"`
	LastMessageAt  time.Time `json:"last_message_at"`
}

type EchoQueryRepository interface {
	ListSessions(ctx context.Context, opts SessionListOptions) ([]*SessionSummary, int64, error)
	FindByMemoryKeyBefore(ctx context.Context, memoryKey, beforeID string, limit int) ([]*entities.Echo, error)
}
