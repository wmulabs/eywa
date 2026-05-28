package ports

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type ChronicleListOptions struct {
	SpiritName    string
	MemoryKey     string
	HasError      bool // true = filter to non-success statuses only
	MinIterations int  // 0 = no filter
	DateFrom      *time.Time
	DateTo        *time.Time
	Page          int
	Limit         int
}

type TokenSeries struct {
	Date             time.Time
	SpiritName       string
	PromptTokens     int
	CompletionTokens int
}

type ActionStats struct {
	ActionName   string
	CallCount    int
	ErrorCount   int
	AvgLatencyMs float64
	P95LatencyMs float64
}

type SpiritStats struct {
	SpiritName    string
	AvgIterations float64
	ErrorRate     float64
	AvgDurationMs float64
}

type ChronicleQueryRepository interface {
	List(ctx context.Context, opts ChronicleListOptions) ([]*entities.Chronicle, int64, error)
	FindByID(ctx context.Context, id string) (*entities.Chronicle, error)
	AggregateTokens(ctx context.Context, spiritName string, from, to time.Time, granularity string) ([]TokenSeries, error)
	AggregateActions(ctx context.Context, spiritName string, from, to time.Time) ([]ActionStats, error)
	AggregateSpirits(ctx context.Context, from, to time.Time) ([]SpiritStats, error)
}
