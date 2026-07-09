package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubLedgerRepo struct {
	budget          entities.TokenBudget
	budgetErr       error
	usage           entities.LedgerEntry
	usageErr        error
	incrementErr    error
	incrementCalled bool
}

var _ ports.LedgerRepository = (*stubLedgerRepo)(nil)

func (r *stubLedgerRepo) GetBudget(_ context.Context, _ string) (entities.TokenBudget, error) {
	return r.budget, r.budgetErr
}
func (r *stubLedgerRepo) GetMonthUsage(_ context.Context, _, _ string) (entities.LedgerEntry, error) {
	return r.usage, r.usageErr
}
func (r *stubLedgerRepo) IncrementUsage(_ context.Context, _ string, _ int64, _ float64) error {
	r.incrementCalled = true
	return r.incrementErr
}
func (r *stubLedgerRepo) SetBudget(_ context.Context, _ entities.TokenBudget) error {
	panic("not implemented")
}

func (r *stubLedgerRepo) ListMonthUsage(_ context.Context, _ string) ([]entities.LedgerEntry, error) {
	return nil, nil
}

func (r *stubLedgerRepo) ListBudgets(_ context.Context) ([]entities.TokenBudget, error) {
	return nil, nil
}

type stubCostAlertHook struct {
	mu              sync.Mutex
	thresholdCalled bool
	exceededCalled  bool
}

var _ ports.CostAlertHook = (*stubCostAlertHook)(nil)

func (h *stubCostAlertHook) OnBudgetThreshold(_ context.Context, _ string, _ float64, _ entities.LedgerEntry) {
	h.mu.Lock()
	h.thresholdCalled = true
	h.mu.Unlock()
}
func (h *stubCostAlertHook) OnBudgetExceeded(_ context.Context, _ string, _ string, _ entities.LedgerEntry) {
	h.mu.Lock()
	h.exceededCalled = true
	h.mu.Unlock()
}
func (h *stubCostAlertHook) wasThresholdCalled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.thresholdCalled
}
func (h *stubCostAlertHook) wasExceededCalled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exceededCalled
}

func costState(spirit *entities.Spirit) *ProcessingState {
	return &ProcessingState{
		Event:       &entities.Pulse{EventType: "user_message", MemoryKey: "user:1"},
		Spirit:      spirit,
		EventConfig: &entities.Link{},
	}
}

// --- CostEnforcementStep ---

func TestCostEnforcementStep_NilLedger_NoOp(t *testing.T) {
	step := NewCostEnforcementStep(nil, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCostEnforcementStep_NilSpirit_NoOp(t *testing.T) {
	repo := &stubLedgerRepo{}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(nil)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCostEnforcementStep_NoBudgetConfigured_Passes(t *testing.T) {
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{MonthlyTokenLimit: 0},
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Skipped {
		t.Error("expected not skipped when no budget limit configured")
	}
}

func TestCostEnforcementStep_BudgetGetError_Passes(t *testing.T) {
	repo := &stubLedgerRepo{budgetErr: errors.New("db down")}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCostEnforcementStep_UsageGetError_Passes(t *testing.T) {
	repo := &stubLedgerRepo{
		budget:   entities.TokenBudget{MonthlyTokenLimit: 1000},
		usageErr: errors.New("redis down"),
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCostEnforcementStep_UnderLimit_Passes(t *testing.T) {
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{MonthlyTokenLimit: 1000},
		usage:  entities.LedgerEntry{TokensUsed: 500},
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Skipped {
		t.Error("expected not skipped when under limit")
	}
}

func TestCostEnforcementStep_BudgetExceeded_Block_SetsSkippedAndResponse(t *testing.T) {
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{MonthlyTokenLimit: 100, OnExceed: "block"},
		usage:  entities.LedgerEntry{TokensUsed: 200},
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !state.Skipped {
		t.Error("expected state.Skipped=true on block")
	}
	if state.Response == "" {
		t.Error("expected response set on block")
	}
	if state.ProcessingStatus != "budget_exceeded" {
		t.Errorf("expected processing_status=budget_exceeded, got %s", state.ProcessingStatus)
	}
}

func TestCostEnforcementStep_BudgetExceeded_Downgrade_ChangesModel(t *testing.T) {
	cheapModel := entities.SpiritModel{Model: "gpt-3.5-turbo"}
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{
			MonthlyTokenLimit: 100,
			OnExceed:          "downgrade",
			DowngradeModel:    cheapModel,
		},
		usage: entities.LedgerEntry{TokensUsed: 200},
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot", ModelConfig: entities.SpiritModel{Model: "gpt-4"}})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.ModelConfig.Model != "gpt-3.5-turbo" {
		t.Errorf("expected model downgraded, got %s", state.Spirit.ModelConfig.Model)
	}
	if state.Skipped {
		t.Error("expected not skipped on downgrade")
	}
}

func TestCostEnforcementStep_ThresholdAlert_FiresHook(t *testing.T) {
	hook := &stubCostAlertHook{}
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{
			MonthlyTokenLimit: 100,
			AlertThreshold:    0.8,
		},
		usage: entities.LedgerEntry{TokensUsed: 90}, // 90% — above threshold, under limit
	}
	step := NewCostEnforcementStep(repo, hook, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Hook fires in goroutine — give it a moment
	time.Sleep(10 * time.Millisecond)
	if !hook.wasThresholdCalled() {
		t.Error("expected threshold hook called when usage >= alert threshold")
	}
}

func TestCostEnforcementStep_ThresholdDowngrade_ChangesModel(t *testing.T) {
	cheapModel := entities.SpiritModel{Model: "gpt-3.5-turbo"}
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{
			MonthlyTokenLimit:    100,
			AlertThreshold:       0.8,
			DowngradeModel:       cheapModel,
			DowngradeAtThreshold: true,
		},
		usage: entities.LedgerEntry{TokensUsed: 90}, // above threshold, under limit
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot", ModelConfig: entities.SpiritModel{Model: "gpt-4"}})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.ModelConfig.Model != "gpt-3.5-turbo" {
		t.Errorf("expected proactive downgrade, got %s", state.Spirit.ModelConfig.Model)
	}
	if state.Skipped {
		t.Error("proactive downgrade must not skip the turn")
	}
}

func TestCostEnforcementStep_ThresholdDowngrade_DisabledByDefault(t *testing.T) {
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{
			MonthlyTokenLimit: 100,
			AlertThreshold:    0.8,
			DowngradeModel:    entities.SpiritModel{Model: "gpt-3.5-turbo"},
			// DowngradeAtThreshold left false
		},
		usage: entities.LedgerEntry{TokensUsed: 90},
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot", ModelConfig: entities.SpiritModel{Model: "gpt-4"}})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.ModelConfig.Model != "gpt-4" {
		t.Errorf("model must stay unchanged when DowngradeAtThreshold is off, got %s", state.Spirit.ModelConfig.Model)
	}
}

func TestCostEnforcementStep_ThresholdDowngrade_NoModelConfigured_NoOp(t *testing.T) {
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{
			MonthlyTokenLimit:    100,
			AlertThreshold:       0.8,
			DowngradeAtThreshold: true, // but no DowngradeModel
		},
		usage: entities.LedgerEntry{TokensUsed: 90},
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot", ModelConfig: entities.SpiritModel{Model: "gpt-4"}})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.ModelConfig.Model != "gpt-4" {
		t.Errorf("model must stay unchanged with no DowngradeModel, got %s", state.Spirit.ModelConfig.Model)
	}
}

// --- LedgerUpdateStep ---

func TestLedgerUpdateStep_NilLedger_NoOp(t *testing.T) {
	step := NewLedgerUpdateStep(nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})
	state.ReasoningResult = &ReasoningResult{TokensUsed: ports.OracleUsage{TotalTokens: 100}}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLedgerUpdateStep_NilSpirit_NoOp(t *testing.T) {
	repo := &stubLedgerRepo{}
	step := NewLedgerUpdateStep(repo, time.Second, testLogger(t))
	state := costState(nil)
	state.ReasoningResult = &ReasoningResult{TokensUsed: ports.OracleUsage{TotalTokens: 100}}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.incrementCalled {
		t.Error("expected no increment when spirit is nil")
	}
}

func TestLedgerUpdateStep_NilReasoningResult_NoOp(t *testing.T) {
	repo := &stubLedgerRepo{}
	step := NewLedgerUpdateStep(repo, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.incrementCalled {
		t.Error("expected no increment when reasoning result is nil")
	}
}

func TestLedgerUpdateStep_ZeroTokens_NoOp(t *testing.T) {
	repo := &stubLedgerRepo{}
	step := NewLedgerUpdateStep(repo, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})
	state.ReasoningResult = &ReasoningResult{TokensUsed: ports.OracleUsage{TotalTokens: 0}}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.incrementCalled {
		t.Error("expected no increment for zero tokens")
	}
}

func TestLedgerUpdateStep_WithUsage_CallsIncrement(t *testing.T) {
	repo := &stubLedgerRepo{}
	step := NewLedgerUpdateStep(repo, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot", ModelConfig: entities.SpiritModel{Provider: "openai", Model: "gpt-4o"}})
	state.ReasoningResult = &ReasoningResult{
		TokensUsed: ports.OracleUsage{
			PromptTokens:     500,
			CompletionTokens: 200,
			TotalTokens:      700,
		},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.incrementCalled {
		t.Error("expected IncrementUsage to be called")
	}
}

func TestLedgerUpdateStep_IncrementError_IsSwallowed(t *testing.T) {
	repo := &stubLedgerRepo{incrementErr: errors.New("db timeout")}
	step := NewLedgerUpdateStep(repo, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})
	state.ReasoningResult = &ReasoningResult{
		TokensUsed: ports.OracleUsage{TotalTokens: 100},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected increment error to be swallowed, got: %v", err)
	}
}

func TestCostEnforcementStep_BudgetExceeded_WithAlertHook_FiresExceededHook(t *testing.T) {
	hook := &stubCostAlertHook{}
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{MonthlyTokenLimit: 100, OnExceed: "block"},
		usage:  entities.LedgerEntry{TokensUsed: 200},
	}
	step := NewCostEnforcementStep(repo, hook, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Hook fires in goroutine — give it a moment
	time.Sleep(10 * time.Millisecond)
	if !hook.wasExceededCalled() {
		t.Error("expected OnBudgetExceeded hook called when budget exceeded and hook present")
	}
}

func TestCostEnforcementStep_BudgetExceeded_DefaultAction(t *testing.T) {
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{MonthlyTokenLimit: 100, OnExceed: "log_only"},
		usage:  entities.LedgerEntry{TokensUsed: 200},
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Skipped {
		t.Error("default action should not skip the request")
	}
}

func TestCostEnforcementStep_BudgetExceeded_Downgrade_NoModel(t *testing.T) {
	repo := &stubLedgerRepo{
		budget: entities.TokenBudget{
			MonthlyTokenLimit: 100,
			OnExceed:          "downgrade",
			DowngradeModel:    entities.SpiritModel{Model: ""}, // empty → no-op
		},
		usage: entities.LedgerEntry{TokensUsed: 200},
	}
	step := NewCostEnforcementStep(repo, nil, time.Second, testLogger(t))
	state := costState(&entities.Spirit{Name: "bot", ModelConfig: entities.SpiritModel{Model: "gpt-4"}})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.ModelConfig.Model != "gpt-4" {
		t.Errorf("model should not change when DowngradeModel.Model is empty, got %q", state.Spirit.ModelConfig.Model)
	}
}
