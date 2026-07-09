package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type LedgerRepository interface {
	IncrementUsage(ctx context.Context, spiritName string, tokens int64, costUSD float64) error
	GetMonthUsage(ctx context.Context, spiritName string, month string) (entities.LedgerEntry, error)
	// ListMonthUsage returns every Spirit's ledger entry for a month ("2026-07").
	ListMonthUsage(ctx context.Context, month string) ([]entities.LedgerEntry, error)
	GetBudget(ctx context.Context, spiritName string) (entities.TokenBudget, error)
	SetBudget(ctx context.Context, budget entities.TokenBudget) error
	ListBudgets(ctx context.Context) ([]entities.TokenBudget, error)
}

type CostAlertHook interface {
	OnBudgetThreshold(ctx context.Context, spiritName string, pctUsed float64, entry entities.LedgerEntry)
	OnBudgetExceeded(ctx context.Context, spiritName string, action string, entry entities.LedgerEntry)
}
