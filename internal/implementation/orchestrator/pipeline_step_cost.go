package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// CostEnforcementStep checks token budget before reasoning.
// On threshold breach: fires CostAlertHook.
// On limit breach: blocks (sets error response) or downgrades model per TokenBudget.OnExceed.
// No-op when LedgerRepository is not configured.
type CostEnforcementStep struct {
	ledgerRepo ports.LedgerRepository
	alertHook  ports.CostAlertHook
	timeout    time.Duration
	logger     *zap.SugaredLogger
}

func NewCostEnforcementStep(
	ledgerRepo ports.LedgerRepository,
	alertHook ports.CostAlertHook,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *CostEnforcementStep {
	return &CostEnforcementStep{
		ledgerRepo: ledgerRepo,
		alertHook:  alertHook,
		timeout:    timeout,
		logger:     logger,
	}
}

func (s *CostEnforcementStep) Name() string           { return "CostEnforcement" }
func (s *CostEnforcementStep) Timeout() time.Duration { return s.timeout }

func (s *CostEnforcementStep) Execute(ctx context.Context, state *ProcessingState) error {
	if s.ledgerRepo == nil || state.Spirit == nil {
		return nil
	}

	month := time.Now().UTC().Format("2006-01")
	spiritName := state.Spirit.Name

	budget, err := s.ledgerRepo.GetBudget(ctx, spiritName)
	if err != nil || budget.MonthlyTokenLimit <= 0 {
		return nil // no budget configured
	}

	entry, err := s.ledgerRepo.GetMonthUsage(ctx, spiritName, month)
	if err != nil {
		return nil // non-fatal; proceed without enforcement
	}

	used := entry.TokensUsed
	limit := budget.MonthlyTokenLimit
	pct := float64(used) / float64(limit)

	atThreshold := budget.AlertThreshold > 0 && pct >= budget.AlertThreshold

	if atThreshold && s.alertHook != nil {
		go func() {
			alertCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			s.alertHook.OnBudgetThreshold(alertCtx, spiritName, pct, entry)
		}()
	}

	if used < limit {
		if atThreshold && budget.DowngradeAtThreshold && budget.DowngradeModel.Model != "" {
			s.downgradeModel(state, budget.DowngradeModel, "threshold reached — downgrading model proactively")
		}
		return nil
	}

	// Budget exceeded.
	if s.alertHook != nil {
		go func() {
			alertCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			s.alertHook.OnBudgetExceeded(alertCtx, spiritName, budget.OnExceed, entry)
		}()
	}

	switch budget.OnExceed {
	case "block":
		s.logger.Warnw("token budget exceeded — blocking request",
			"spirit", spiritName,
			"used", used,
			"limit", limit)
		state.Response = "Monthly request limit reached. Please try again next month."
		state.ProcessingStatus = "budget_exceeded"
		state.ResponseDelivered = true
		state.Skipped = true
		return nil

	case "downgrade":
		if budget.DowngradeModel.Model != "" {
			s.downgradeModel(state, budget.DowngradeModel, "exceeded — downgrading model")
		}

	default:
		s.logger.Infow("token budget exceeded — no enforcement action",
			"spirit", spiritName,
			"on_exceed", budget.OnExceed)
	}

	return nil
}

func (s *CostEnforcementStep) downgradeModel(state *ProcessingState, to entities.SpiritModel, reason string) {
	s.logger.Infow("token budget "+reason,
		"spirit", state.Spirit.Name,
		"from_model", state.Spirit.ModelConfig.Model,
		"to_model", to.Model)
	state.Spirit.ModelConfig = to
}

// LedgerUpdateStep records token usage and estimated cost after reasoning completes.
// Best-effort: failures are logged but never fail the pipeline.
type LedgerUpdateStep struct {
	ledgerRepo ports.LedgerRepository
	timeout    time.Duration
	logger     *zap.SugaredLogger
}

func NewLedgerUpdateStep(
	ledgerRepo ports.LedgerRepository,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *LedgerUpdateStep {
	return &LedgerUpdateStep{
		ledgerRepo: ledgerRepo,
		timeout:    timeout,
		logger:     logger,
	}
}

func (s *LedgerUpdateStep) Name() string           { return "LedgerUpdate" }
func (s *LedgerUpdateStep) Timeout() time.Duration { return s.timeout }

func (s *LedgerUpdateStep) Execute(ctx context.Context, state *ProcessingState) error {
	if s.ledgerRepo == nil || state.Spirit == nil || state.ReasoningResult == nil {
		return nil
	}

	usage := state.ReasoningResult.TokensUsed
	total := usage.TotalTokens
	if total == 0 {
		return nil
	}

	costUSD := estimateCost(
		state.Spirit.ModelConfig.Provider,
		state.Spirit.ModelConfig.Model,
		int64(usage.PromptTokens),
		int64(usage.CompletionTokens),
	)

	spiritName := state.Spirit.Name
	if err := s.ledgerRepo.IncrementUsage(ctx, spiritName, int64(total), costUSD); err != nil {
		s.logger.Warnw("ledger update failed", "spirit", spiritName, "error", err)
	} else {
		s.logger.Infow("ledger updated",
			"spirit", spiritName,
			"tokens", total,
			"cost_usd", fmt.Sprintf("%.6f", costUSD))
	}

	return nil
}
