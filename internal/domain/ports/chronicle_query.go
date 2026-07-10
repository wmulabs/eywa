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
	Date             time.Time `json:"date"`
	SpiritName       string    `json:"spirit_name"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
}

type ActionStats struct {
	ActionName   string  `json:"action_name"`
	CallCount    int     `json:"call_count"`
	ErrorCount   int     `json:"error_count"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P95LatencyMs float64 `json:"p95_latency_ms"`
}

type SpiritStats struct {
	SpiritName    string  `json:"spirit_name"`
	AvgIterations float64 `json:"avg_iterations"`
	ErrorRate     float64 `json:"error_rate"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

type ChronicleQueryRepository interface {
	List(ctx context.Context, opts ChronicleListOptions) ([]*entities.Chronicle, int64, error)
	FindByID(ctx context.Context, id string) (*entities.Chronicle, error)
	AggregateTokens(ctx context.Context, spiritName string, from, to time.Time, granularity string) ([]TokenSeries, error)
	AggregateActions(ctx context.Context, spiritName string, from, to time.Time) ([]ActionStats, error)
	AggregateSpirits(ctx context.Context, from, to time.Time) ([]SpiritStats, error)
}
