package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// OrchestratorRoutingStep handles orchestrator Spirits operating in "logic" mode.
// It calls the configured LogicRouter to select a sub-Spirit, then reloads the Spirit
// so the rest of the pipeline runs as if that sub-Spirit were selected directly.
// No-op for non-orchestrator Spirits or non-"logic" modes.
type OrchestratorRoutingStep struct {
	logicRouterRegistry ports.LogicRouterRegistry
	spiritRepo          ports.SpiritRepository
	timeout             time.Duration
	logger              *zap.SugaredLogger
}

func NewOrchestratorRoutingStep(
	logicRouterRegistry ports.LogicRouterRegistry,
	spiritRepo ports.SpiritRepository,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *OrchestratorRoutingStep {
	return &OrchestratorRoutingStep{
		logicRouterRegistry: logicRouterRegistry,
		spiritRepo:          spiritRepo,
		timeout:             timeout,
		logger:              logger,
	}
}

func (s *OrchestratorRoutingStep) Name() string           { return "OrchestratorRouting" }
func (s *OrchestratorRoutingStep) Timeout() time.Duration { return s.timeout }

func (s *OrchestratorRoutingStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Spirit == nil || !state.Spirit.IsOrchestrator() {
		return nil
	}
	if state.Spirit.OrchestratorConfig.Mode != "logic" {
		return nil
	}
	if s.logicRouterRegistry == nil {
		return nil
	}

	routerName := state.Spirit.OrchestratorConfig.LogicRouterName
	if routerName == "" {
		return nil
	}

	router, err := s.logicRouterRegistry.Get(routerName)
	if err != nil {
		s.logger.Warnw("logic router not found, orchestrator falls through",
			"router", routerName,
			"spirit", state.Spirit.Name)
		return nil
	}

	subSpiritName := router.Route(ctx, state.Event)
	if subSpiritName == "" {
		s.logger.Infow("logic router returned no match, orchestrator falls through",
			"router", routerName,
			"memory_key", state.Event.MemoryKey)
		return nil
	}

	subSpirit, err := s.spiritRepo.FindActiveByName(ctx, subSpiritName)
	if err != nil {
		s.logger.Warnw("logic router target spirit not found, falling through",
			"target_spirit", subSpiritName,
			"error", err)
		return nil
	}

	s.logger.Infow("orchestrator routed via logic router",
		"router", routerName,
		"from_spirit", state.Spirit.Name,
		"to_spirit", subSpiritName,
		"memory_key", state.Event.MemoryKey)

	state.Spirit = subSpirit
	state.SpiritName = subSpiritName
	return nil
}
