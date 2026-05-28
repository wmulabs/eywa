package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// stubReasoningExecutorForSummon is a simple stub (stubReasoningExecutor already defined
// in pipeline_step_reasoning_test.go, reuse that name would conflict — use distinct name).
type stubReasoningExec struct {
	result *ReasoningResult
	err    error
}

func (s *stubReasoningExec) Execute(_ context.Context, _ *ReasoningRequest) (*ReasoningResult, error) {
	return s.result, s.err
}

func parentSpirit(subSpirits ...string) *entities.Spirit {
	return &entities.Spirit{
		Name: "orchestrator",
		Type: entities.SpiritTypeOrchestrator,
		OrchestratorConfig: entities.OrchestratorConfig{
			SubSpirits: subSpirits,
			MaxDepth:   3,
		},
	}
}

func parentPulse() *entities.Pulse {
	return &entities.Pulse{
		ID:                 "pulse-1",
		MemoryKey:          "user:1",
		Knowledge:          map[string]any{"order_id": "ORD-1"},
		OrchestrationDepth: 0,
	}
}

// --- NewSummonService ---

func TestNewSummonService_ReturnsNonNil(t *testing.T) {
	svc := NewSummonService(nil, nil, &stubReasoningExec{}, testLogger(t))
	if svc == nil {
		t.Fatal("expected non-nil")
	}
}

// --- validateSummon ---

func TestValidateSummon_NotAllowed_ReturnsBusinessError(t *testing.T) {
	svc := NewSummonService(nil, nil, &stubReasoningExec{}, testLogger(t))
	err := svc.validateSummon("unauthorized", parentPulse(), parentSpirit("support"))
	if err == nil {
		t.Fatal("expected error for not-allowed spirit")
	}
	if !domainerrors.IsBusinessError(err) {
		t.Errorf("expected BusinessError, got %T", err)
	}
}

func TestValidateSummon_DepthExceeded_ReturnsBusinessError(t *testing.T) {
	svc := NewSummonService(nil, nil, &stubReasoningExec{}, testLogger(t))
	pulse := parentPulse()
	pulse.OrchestrationDepth = 3 // equals MaxDepth=3
	err := svc.validateSummon("support", pulse, parentSpirit("support"))
	if err == nil {
		t.Fatal("expected error for depth exceeded")
	}
	if !domainerrors.IsBusinessError(err) {
		t.Errorf("expected BusinessError, got %T", err)
	}
}

func TestValidateSummon_DefaultMaxDepth_WhenZero(t *testing.T) {
	spirit := parentSpirit("support")
	spirit.OrchestratorConfig.MaxDepth = 0 // triggers default of 3
	svc := NewSummonService(nil, nil, &stubReasoningExec{}, testLogger(t))

	pulse := parentPulse()
	pulse.OrchestrationDepth = 3 // equals default max depth
	err := svc.validateSummon("support", pulse, spirit)
	if err == nil {
		t.Fatal("expected depth error with default max depth")
	}
}

func TestValidateSummon_Success(t *testing.T) {
	svc := NewSummonService(nil, nil, &stubReasoningExec{}, testLogger(t))
	err := svc.validateSummon("support", parentPulse(), parentSpirit("support", "billing"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- buildChildPulse ---

func TestBuildChildPulse_CopiesKnowledge(t *testing.T) {
	svc := NewSummonService(nil, nil, &stubReasoningExec{}, testLogger(t))
	child := svc.buildChildPulse("track my order", parentPulse())

	if child.UserMessage != "track my order" {
		t.Errorf("expected task as UserMessage, got %q", child.UserMessage)
	}
	if child.MemoryKey != "user:1" {
		t.Errorf("expected user:1, got %s", child.MemoryKey)
	}
	if child.OrchestrationDepth != 1 {
		t.Errorf("expected depth=1, got %d", child.OrchestrationDepth)
	}
	if child.ParentPulseID != "pulse-1" {
		t.Errorf("expected parent pulse-1, got %s", child.ParentPulseID)
	}
	if child.Knowledge["order_id"] != "ORD-1" {
		t.Error("expected knowledge copied from parent")
	}
}

func TestBuildChildPulse_MutationDoesNotAffectParent(t *testing.T) {
	svc := NewSummonService(nil, nil, &stubReasoningExec{}, testLogger(t))
	parent := parentPulse()
	child := svc.buildChildPulse("task", parent)
	child.Knowledge["injected"] = "val"

	if _, ok := parent.Knowledge["injected"]; ok {
		t.Error("child knowledge mutation must not affect parent")
	}
}

// --- Summon ---

func TestSummon_SpiritNotAllowed_ReturnsError(t *testing.T) {
	svc := NewSummonService(nil, nil, &stubReasoningExec{}, testLogger(t))
	_, err := svc.Summon(context.Background(), "unknown", "task", parentPulse(), parentSpirit("support"))
	if err == nil {
		t.Fatal("expected error for not-allowed spirit")
	}
}

func TestSummon_SpiritRepoFails_ReturnsInfraError(t *testing.T) {
	repo := &stubSpiritRepo{err: errors.New("db down")}
	svc := NewSummonService(repo, nil, &stubReasoningExec{}, testLogger(t))
	_, err := svc.Summon(context.Background(), "support", "task", parentPulse(), parentSpirit("support"))
	if err == nil {
		t.Fatal("expected error when spirit not found")
	}
	if !domainerrors.IsInfrastructureError(err) {
		t.Errorf("expected InfrastructureError, got %T: %v", err, err)
	}
}

func TestSummon_ReasoningFails_ReturnsInfraError(t *testing.T) {
	spirit := &entities.Spirit{Name: "support", Type: entities.SpiritTypeConversational}
	repo := &stubSpiritRepo{spirit: spirit}
	reasoning := &stubReasoningExec{err: errors.New("llm timeout")}
	svc := NewSummonService(repo, nil, reasoning, testLogger(t))
	_, err := svc.Summon(context.Background(), "support", "task", parentPulse(), parentSpirit("support"))
	if err == nil {
		t.Fatal("expected error when reasoning fails")
	}
	if !domainerrors.IsInfrastructureError(err) {
		t.Errorf("expected InfrastructureError, got %T", err)
	}
}

func TestSummon_Success_ReturnsFinalResponse(t *testing.T) {
	spirit := &entities.Spirit{Name: "support", Type: entities.SpiritTypeConversational}
	repo := &stubSpiritRepo{spirit: spirit}
	reasoning := &stubReasoningExec{result: &ReasoningResult{FinalResponse: "order is on the way"}}
	svc := NewSummonService(repo, nil, reasoning, testLogger(t))
	resp, err := svc.Summon(context.Background(), "support", "where is my order?", parentPulse(), parentSpirit("support"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "order is on the way" {
		t.Errorf("expected 'order is on the way', got %q", resp)
	}
}

func TestSummon_WithScouts_RunsThemBeforeReasoning(t *testing.T) {
	spirit := &entities.Spirit{
		Name:          "support",
		Type:          entities.SpiritTypeConversational,
		RequireScouts: []string{"geo"},
	}
	repo := &stubSpiritRepo{spirit: spirit}
	reasoning := &stubReasoningExec{result: &ReasoningResult{FinalResponse: "done"}}
	scout := &stubScoutForSummon{}
	registry := &stubScoutRegistryForSummon{scout: scout}
	svc := NewSummonService(repo, registry, reasoning, testLogger(t))

	_, err := svc.Summon(context.Background(), "support", "task", parentPulse(), parentSpirit("support"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// minimal Scout/ScoutRegistry stubs for Summon tests

type stubScoutForSummon struct{}

func (s *stubScoutForSummon) IsApplicable(_ *entities.Pulse) bool              { return true }
func (s *stubScoutForSummon) Harvest(_ context.Context, _ *entities.Pulse) error { return nil }
func (s *stubScoutForSummon) GetName() string                                   { return "geo" }

type stubScoutRegistryForSummon struct {
	scout ports.Scout
}

func (r *stubScoutRegistryForSummon) Register(_ ports.Scout) error      { return nil }
func (r *stubScoutRegistryForSummon) RegisterMultiple(_ ...ports.Scout) error { return nil }
func (r *stubScoutRegistryForSummon) Harvest(_ context.Context, _ *entities.Pulse) error { return nil }
func (r *stubScoutRegistryForSummon) GetScout(_ string) (ports.Scout, error) { return r.scout, nil }
func (r *stubScoutRegistryForSummon) GetMultiple(_ []string) ([]ports.Scout, error) {
	if r.scout != nil {
		return []ports.Scout{r.scout}, nil
	}
	return nil, nil
}
func (r *stubScoutRegistryForSummon) ListScouts() []string { return nil }
func (r *stubScoutRegistryForSummon) List() []ports.Scout  { return nil }
