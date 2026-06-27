package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

type SpiritSelectionStep struct {
	selector     SpiritSelector
	handoffStore ports.HandoffStore // optional; pins the active Spirit after a handoff
	timeout      time.Duration
	logger       *zap.SugaredLogger
}

func NewSpiritSelectionStep(selector SpiritSelector, handoffStore ports.HandoffStore, timeout time.Duration, logger *zap.SugaredLogger) *SpiritSelectionStep {
	return &SpiritSelectionStep{selector: selector, handoffStore: handoffStore, timeout: timeout, logger: logger}
}

func (s *SpiritSelectionStep) Name() string           { return "SpiritSelection" }
func (s *SpiritSelectionStep) Timeout() time.Duration { return s.timeout }

func (s *SpiritSelectionStep) Execute(ctx context.Context, state *ProcessingState) error {
	// A prior handoff pins the active Spirit so the conversation stays with it across turns.
	if s.handoffStore != nil {
		if active, err := s.handoffStore.GetActiveSpirit(ctx, state.Event.MemoryKey); err != nil {
			s.logger.Warnw("handoff store lookup failed, falling back to selection", "error", err)
		} else if active != "" {
			state.SpiritName = active
			state.PathfinderUsed = "handoff"
			s.logger.Infow("Spirit pinned by handoff", "spirit_name", active, "memory_key", state.Event.MemoryKey)
			return nil
		}
	}

	state.SpiritName = s.selector.selectSpirit(ctx, state.Event, state.EventConfig)
	state.PathfinderUsed = state.EventConfig.PathfinderName
	s.logger.Infow("Spirit selected",
		"spirit_name", state.SpiritName,
		"available_spirits", state.EventConfig.AllowedSpirits,
	)
	return nil
}

type SpiritLoadStep struct {
	spiritRepo ports.SpiritRepository
	timeout    time.Duration
	logger     *zap.SugaredLogger
}

func NewSpiritLoadStep(spiritRepo ports.SpiritRepository, timeout time.Duration, logger *zap.SugaredLogger) *SpiritLoadStep {
	return &SpiritLoadStep{spiritRepo: spiritRepo, timeout: timeout, logger: logger}
}

func (s *SpiritLoadStep) Name() string           { return "SpiritLoad" }
func (s *SpiritLoadStep) Timeout() time.Duration { return s.timeout }

func (s *SpiritLoadStep) Execute(ctx context.Context, state *ProcessingState) error {
	spirit, err := s.spiritRepo.FindActiveByName(ctx, state.SpiritName)
	if err != nil {
		return ErrSpiritNotFound(state.SpiritName, err)
	}
	state.Spirit = spirit
	return nil
}
