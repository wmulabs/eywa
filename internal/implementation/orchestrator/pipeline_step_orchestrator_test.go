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

type stubLogicRouter struct {
	name   string
	result string
}

var _ ports.LogicRouter = (*stubLogicRouter)(nil)

func (r *stubLogicRouter) GetName() string                                   { return r.name }
func (r *stubLogicRouter) Route(_ context.Context, _ *entities.Pulse) string { return r.result }

type stubLogicRouterRegistry struct {
	router ports.LogicRouter
	err    error
}

var _ ports.LogicRouterRegistry = (*stubLogicRouterRegistry)(nil)

func (reg *stubLogicRouterRegistry) Register(_ ports.LogicRouter) error { return nil }
func (reg *stubLogicRouterRegistry) Get(_ string) (ports.LogicRouter, error) {
	return reg.router, reg.err
}
func (reg *stubLogicRouterRegistry) List() []string { return nil }

func orchestratorState(mode, routerName string) *ProcessingState {
	return &ProcessingState{
		Event: &entities.Pulse{EventType: "user_message", MemoryKey: "user:1"},
		Spirit: &entities.Spirit{
			Name: "orchestrator-bot",
			Type: entities.SpiritTypeOrchestrator,
			OrchestratorConfig: entities.OrchestratorConfig{
				Mode:            mode,
				LogicRouterName: routerName,
			},
		},
		EventConfig: &entities.Link{},
	}
}

// --- OrchestratorRoutingStep ---

func TestOrchestratorRoutingStep_NilSpirit_NoOp(t *testing.T) {
	step := NewOrchestratorRoutingStep(nil, nil, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: "user:1"},
		Spirit:      nil,
		EventConfig: &entities.Link{},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrchestratorRoutingStep_NonOrchestratorSpirit_NoOp(t *testing.T) {
	step := NewOrchestratorRoutingStep(nil, nil, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: "user:1"},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeConversational},
		EventConfig: &entities.Link{},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrchestratorRoutingStep_NonLogicMode_NoOp(t *testing.T) {
	step := NewOrchestratorRoutingStep(nil, nil, time.Second, testLogger(t))
	state := orchestratorState("scout", "my-router")
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrchestratorRoutingStep_NilRegistry_NoOp(t *testing.T) {
	step := NewOrchestratorRoutingStep(nil, nil, time.Second, testLogger(t))
	state := orchestratorState("logic", "my-router")
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrchestratorRoutingStep_EmptyRouterName_NoOp(t *testing.T) {
	registry := &stubLogicRouterRegistry{}
	step := NewOrchestratorRoutingStep(registry, nil, time.Second, testLogger(t))
	state := orchestratorState("logic", "")
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrchestratorRoutingStep_RouterNotFound_NoOp(t *testing.T) {
	registry := &stubLogicRouterRegistry{err: errors.New("not found")}
	step := NewOrchestratorRoutingStep(registry, nil, time.Second, testLogger(t))
	state := orchestratorState("logic", "missing-router")
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrchestratorRoutingStep_RouterReturnsEmpty_NoOp(t *testing.T) {
	router := &stubLogicRouter{name: "my-router", result: ""}
	registry := &stubLogicRouterRegistry{router: router}
	step := NewOrchestratorRoutingStep(registry, nil, time.Second, testLogger(t))
	state := orchestratorState("logic", "my-router")
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.Name != "orchestrator-bot" {
		t.Error("expected spirit unchanged when router returns empty")
	}
}

func TestOrchestratorRoutingStep_SpiritNotFound_NoOp(t *testing.T) {
	router := &stubLogicRouter{name: "my-router", result: "support-bot"}
	registry := &stubLogicRouterRegistry{router: router}
	spiritRepo := &stubSpiritRepo{err: errors.New("not found")}
	step := NewOrchestratorRoutingStep(registry, spiritRepo, time.Second, testLogger(t))
	state := orchestratorState("logic", "my-router")
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.Name != "orchestrator-bot" {
		t.Error("expected original spirit kept when sub-spirit not found")
	}
}

func TestOrchestratorRoutingStep_SuccessfulRoute_UpdatesSpirit(t *testing.T) {
	subSpirit := &entities.Spirit{Name: "support-bot", Type: entities.SpiritTypeConversational}
	router := &stubLogicRouter{name: "my-router", result: "support-bot"}
	registry := &stubLogicRouterRegistry{router: router}
	spiritRepo := &stubSpiritRepo{spirit: subSpirit}
	step := NewOrchestratorRoutingStep(registry, spiritRepo, time.Second, testLogger(t))
	state := orchestratorState("logic", "my-router")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.Name != "support-bot" {
		t.Errorf("expected spirit=support-bot, got %s", state.Spirit.Name)
	}
	if state.SpiritName != "support-bot" {
		t.Errorf("expected SpiritName=support-bot, got %s", state.SpiritName)
	}
}
