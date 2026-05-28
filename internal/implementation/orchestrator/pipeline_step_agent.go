package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

type SpiritSelectionStep struct {
	selector SpiritSelector
	timeout  time.Duration
	logger   *zap.SugaredLogger
}

func NewSpiritSelectionStep(selector SpiritSelector, timeout time.Duration, logger *zap.SugaredLogger) *SpiritSelectionStep {
	return &SpiritSelectionStep{selector: selector, timeout: timeout, logger: logger}
}

func (s *SpiritSelectionStep) Name() string           { return "SpiritSelection" }
func (s *SpiritSelectionStep) Timeout() time.Duration { return s.timeout }

func (s *SpiritSelectionStep) Execute(ctx context.Context, state *ProcessingState) error {
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
