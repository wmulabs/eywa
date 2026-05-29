package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubScoutForEnrich struct {
	name       string
	applicable bool
	harvestErr error
	harvested  bool
}

var _ ports.Scout = (*stubScoutForEnrich)(nil)

func (s *stubScoutForEnrich) GetName() string                     { return s.name }
func (s *stubScoutForEnrich) IsApplicable(_ *entities.Pulse) bool { return s.applicable }
func (s *stubScoutForEnrich) Harvest(_ context.Context, _ *entities.Pulse) error {
	s.harvested = true
	return s.harvestErr
}

type stubScoutRegistry struct {
	scouts map[string]ports.Scout
	getErr error
}

var _ ports.ScoutRegistry = (*stubScoutRegistry)(nil)

func (r *stubScoutRegistry) Register(_ ports.Scout) error                       { return nil }
func (r *stubScoutRegistry) RegisterMultiple(_ ...ports.Scout) error            { return nil }
func (r *stubScoutRegistry) Harvest(_ context.Context, _ *entities.Pulse) error { return nil }
func (r *stubScoutRegistry) GetScout(name string) (ports.Scout, error) {
	if s, ok := r.scouts[name]; ok {
		return s, nil
	}
	return nil, errors.New("not found")
}
func (r *stubScoutRegistry) GetMultiple(names []string) ([]ports.Scout, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	var result []ports.Scout
	for _, name := range names {
		if s, ok := r.scouts[name]; ok {
			result = append(result, s)
		}
	}
	return result, nil
}
func (r *stubScoutRegistry) ListScouts() []string { return nil }
func (r *stubScoutRegistry) List() []ports.Scout  { return nil }

type stubLoreHarvester struct {
	err    error
	called bool
}

var _ ports.LoreHarvester = (*stubLoreHarvester)(nil)

func (h *stubLoreHarvester) Harvest(_ context.Context, _ *entities.Pulse, _ []string) error {
	h.called = true
	return h.err
}

type stubImprintHarvesterEnrich struct {
	err    error
	called bool
}

var _ ports.ImprintHarvester = (*stubImprintHarvesterEnrich)(nil)

func (h *stubImprintHarvesterEnrich) Harvest(_ context.Context, _ *entities.Pulse, _ string) error {
	h.called = true
	return h.err
}

// memory-key-changing scout
type memKeyScout struct {
	newKey string
}

var _ ports.Scout = (*memKeyScout)(nil)

func (s *memKeyScout) GetName() string                     { return "mem-key-changer" }
func (s *memKeyScout) IsApplicable(_ *entities.Pulse) bool { return true }
func (s *memKeyScout) Harvest(_ context.Context, pulse *entities.Pulse) error {
	pulse.MemoryKey = s.newKey
	return nil
}

// helpers

func enrichState(memoryKey string) *ProcessingState {
	return &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: memoryKey, UserMessage: "hello", Knowledge: map[string]any{}},
		EventConfig: &entities.Link{},
	}
}

func enrichStateWithScouts(memoryKey string, scouts []string) *ProcessingState {
	s := enrichState(memoryKey)
	s.EventConfig.RequireScouts = scouts
	return s
}

// =============================================================================
// LinkScoutStep
// =============================================================================

func TestLinkScoutStep_NilRegistry_NoOp(t *testing.T) {
	step := NewLinkScoutStep(nil, &stubBond{acquired: true}, time.Second, time.Second, testLogger(t))
	state := enrichStateWithScouts("user:1", []string{"s1"})
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLinkScoutStep_NilEventConfig_NoOp(t *testing.T) {
	reg := &stubScoutRegistry{}
	step := NewLinkScoutStep(reg, &stubBond{acquired: true}, time.Second, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: "user:1"},
		EventConfig: nil,
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLinkScoutStep_EmptyRequireScouts_NoOp(t *testing.T) {
	reg := &stubScoutRegistry{}
	step := NewLinkScoutStep(reg, &stubBond{acquired: true}, time.Second, time.Second, testLogger(t))
	state := enrichStateWithScouts("user:1", []string{})
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLinkScoutStep_GetMultipleFails_ReturnsError(t *testing.T) {
	reg := &stubScoutRegistry{getErr: errors.New("registry error")}
	step := NewLinkScoutStep(reg, &stubBond{acquired: true}, time.Second, time.Second, testLogger(t))
	state := enrichStateWithScouts("user:1", []string{"s1"})

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Error("expected error from GetMultiple failure")
	}
}

func TestLinkScoutStep_ScoutNotApplicable_NoHarvest(t *testing.T) {
	scout := &stubScoutForEnrich{name: "s1", applicable: false}
	reg := &stubScoutRegistry{scouts: map[string]ports.Scout{"s1": scout}}
	step := NewLinkScoutStep(reg, &stubBond{acquired: true}, time.Second, time.Second, testLogger(t))
	state := enrichStateWithScouts("user:1", []string{"s1"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scout.harvested {
		t.Error("non-applicable scout should not be harvested")
	}
}

func TestLinkScoutStep_ScoutApplicable_Harvests(t *testing.T) {
	scout := &stubScoutForEnrich{name: "s1", applicable: true}
	reg := &stubScoutRegistry{scouts: map[string]ports.Scout{"s1": scout}}
	step := NewLinkScoutStep(reg, &stubBond{acquired: true}, time.Second, time.Second, testLogger(t))
	state := enrichStateWithScouts("user:1", []string{"s1"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scout.harvested {
		t.Error("applicable scout should be harvested")
	}
}

func TestLinkScoutStep_ScoutHarvestFails_ContinuesPipeline(t *testing.T) {
	scout := &stubScoutForEnrich{name: "s1", applicable: true, harvestErr: errors.New("harvest fail")}
	reg := &stubScoutRegistry{scouts: map[string]ports.Scout{"s1": scout}}
	step := NewLinkScoutStep(reg, &stubBond{acquired: true}, time.Second, time.Second, testLogger(t))
	state := enrichStateWithScouts("user:1", []string{"s1"})

	// fail-open: Scout harvest errors must not abort the pipeline
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected no error from scout harvest failure (fail-open), got: %v", err)
	}
}

func TestLinkScoutStep_MemoryKeyChanged_AcquiresAdditionalLock(t *testing.T) {
	scout := &memKeyScout{newKey: "user:resolved"}
	reg := &stubScoutRegistry{scouts: map[string]ports.Scout{"mem-key-changer": scout}}
	bond := &stubBond{acquired: true}
	step := NewLinkScoutStep(reg, bond, time.Second, time.Second, testLogger(t))
	state := enrichStateWithScouts("user:1", []string{"mem-key-changer"})

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(state.AdditionalLocks) == 0 {
		t.Error("expected additional lock after memory_key change")
	}
}

func TestLinkScoutStep_MemoryKeyChanged_LockFails_ReturnsError(t *testing.T) {
	scout := &memKeyScout{newKey: "user:resolved"}
	reg := &stubScoutRegistry{scouts: map[string]ports.Scout{"mem-key-changer": scout}}
	bond := &stubBond{acquired: false, acquireErr: errors.New("lock fail")}
	step := NewLinkScoutStep(reg, bond, time.Second, time.Second, testLogger(t))
	state := enrichStateWithScouts("user:1", []string{"mem-key-changer"})

	if err := step.Execute(context.Background(), state); err == nil {
		t.Error("expected error when additional lock fails")
	}
}

func TestLinkScoutStep_MemoryKeyChanged_LockNotAcquired_ReturnsError(t *testing.T) {
	scout := &memKeyScout{newKey: "user:resolved"}
	reg := &stubScoutRegistry{scouts: map[string]ports.Scout{"mem-key-changer": scout}}
	bond := &stubBond{acquired: false} // not acquired, no error
	step := NewLinkScoutStep(reg, bond, time.Second, time.Second, testLogger(t))
	state := enrichStateWithScouts("user:1", []string{"mem-key-changer"})

	if err := step.Execute(context.Background(), state); err == nil {
		t.Error("expected error when lock not acquired")
	}
}

// =============================================================================
// SpiritScoutStep
// =============================================================================

func TestSpiritScoutStep_NilSpirit_NoOp(t *testing.T) {
	step := NewSpiritScoutStep(nil, nil, nil, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1"},
		Spirit: nil,
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpiritScoutStep_NilRegistry_SkipsScouts(t *testing.T) {
	step := NewSpiritScoutStep(nil, nil, nil, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1", Knowledge: map[string]any{}},
		Spirit: &entities.Spirit{Name: "support", RequireScouts: []string{"s1"}},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpiritScoutStep_ScoutRegistryFails_ReturnsError(t *testing.T) {
	reg := &stubScoutRegistry{getErr: errors.New("registry error")}
	step := NewSpiritScoutStep(reg, nil, nil, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1"},
		Spirit: &entities.Spirit{Name: "support", RequireScouts: []string{"s1"}},
	}
	if err := step.Execute(context.Background(), state); err == nil {
		t.Error("expected error from registry failure")
	}
}

func TestSpiritScoutStep_LoreHarvester_Called(t *testing.T) {
	harvester := &stubLoreHarvester{}
	step := NewSpiritScoutStep(nil, harvester, nil, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1", Knowledge: map[string]any{}},
		Spirit: &entities.Spirit{Name: "support", LoreIDs: []string{"lore-1"}},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !harvester.called {
		t.Error("expected lore harvester to be called")
	}
}

func TestSpiritScoutStep_LoreHarvester_Error_Swallowed(t *testing.T) {
	harvester := &stubLoreHarvester{err: errors.New("lore error")}
	step := NewSpiritScoutStep(nil, harvester, nil, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1", Knowledge: map[string]any{}},
		Spirit: &entities.Spirit{Name: "support", LoreIDs: []string{"lore-1"}},
	}
	// lore errors are swallowed (continue without RAG)
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("lore harvest error must be swallowed, got: %v", err)
	}
}

func TestSpiritScoutStep_LoreHarvester_EmptyLoreIDs_NotCalled(t *testing.T) {
	harvester := &stubLoreHarvester{}
	step := NewSpiritScoutStep(nil, harvester, nil, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1"},
		Spirit: &entities.Spirit{Name: "support", LoreIDs: []string{}},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if harvester.called {
		t.Error("lore harvester must not be called when LoreIDs is empty")
	}
}

func TestSpiritScoutStep_ImprintHarvester_Conversational(t *testing.T) {
	harvester := &stubImprintHarvesterEnrich{}
	step := NewSpiritScoutStep(nil, nil, harvester, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1", Knowledge: map[string]any{}},
		Spirit: &entities.Spirit{Name: "chat", Type: entities.SpiritTypeConversational},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !harvester.called {
		t.Error("expected imprint harvester to be called for conversational spirit")
	}
}

func TestSpiritScoutStep_ImprintHarvester_Orchestrator(t *testing.T) {
	harvester := &stubImprintHarvesterEnrich{}
	step := NewSpiritScoutStep(nil, nil, harvester, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1", Knowledge: map[string]any{}},
		Spirit: &entities.Spirit{Name: "orchestrator", Type: entities.SpiritTypeOrchestrator},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !harvester.called {
		t.Error("expected imprint harvester to be called for orchestrator spirit")
	}
}

func TestSpiritScoutStep_ImprintHarvester_NotConversational_NotCalled(t *testing.T) {
	harvester := &stubImprintHarvesterEnrich{}
	step := NewSpiritScoutStep(nil, nil, harvester, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1"},
		Spirit: &entities.Spirit{Name: "notifier", Type: entities.SpiritTypeNotifier},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if harvester.called {
		t.Error("imprint harvester must not be called for non-conversational/orchestrator spirits")
	}
}

func TestSpiritScoutStep_ImprintHarvester_Error_Swallowed(t *testing.T) {
	harvester := &stubImprintHarvesterEnrich{err: errors.New("imprint error")}
	step := NewSpiritScoutStep(nil, nil, harvester, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1", Knowledge: map[string]any{}},
		Spirit: &entities.Spirit{Name: "chat", Type: entities.SpiritTypeConversational},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("imprint harvest error must be swallowed, got: %v", err)
	}
}

func TestSpiritScoutStep_ScoutHarvestFails_ContinuesPipeline(t *testing.T) {
	scout := &stubScoutForEnrich{name: "s1", applicable: true, harvestErr: errors.New("scout fail")}
	reg := &stubScoutRegistry{scouts: map[string]ports.Scout{"s1": scout}}
	step := NewSpiritScoutStep(reg, nil, nil, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:  &entities.Pulse{MemoryKey: "user:1"},
		Spirit: &entities.Spirit{Name: "support", RequireScouts: []string{"s1"}},
	}
	// fail-open: Scout harvest errors must not abort the pipeline
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected no error from scout harvest failure (fail-open), got: %v", err)
	}
}

func TestSpiritScoutStep_RunsScoutsAndHarvesters_Together(t *testing.T) {
	scout := &stubScoutForEnrich{name: "s1", applicable: true}
	reg := &stubScoutRegistry{scouts: map[string]ports.Scout{"s1": scout}}
	loreHarvester := &stubLoreHarvester{}
	imprintHarvester := &stubImprintHarvesterEnrich{}
	step := NewSpiritScoutStep(reg, loreHarvester, imprintHarvester, time.Second, testLogger(t))
	state := &ProcessingState{
		Event: &entities.Pulse{MemoryKey: "user:1", Knowledge: map[string]any{}},
		Spirit: &entities.Spirit{
			Name:          "support",
			Type:          entities.SpiritTypeConversational,
			RequireScouts: []string{"s1"},
			LoreIDs:       []string{"lore-1"},
		},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scout.harvested {
		t.Error("expected scout to be harvested")
	}
	if !loreHarvester.called {
		t.Error("expected lore harvester to be called")
	}
	if !imprintHarvester.called {
		t.Error("expected imprint harvester to be called")
	}
}
