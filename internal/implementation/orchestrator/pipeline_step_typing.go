package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

type TypingStartStep struct {
	indicator ports.TypingIndicator
	logger    *zap.SugaredLogger
}

func NewTypingStartStep(indicator ports.TypingIndicator, logger *zap.SugaredLogger) *TypingStartStep {
	return &TypingStartStep{indicator: indicator, logger: logger}
}

func (s *TypingStartStep) Name() string           { return "TypingStart" }
func (s *TypingStartStep) Timeout() time.Duration { return 3 * time.Second }

func (s *TypingStartStep) Execute(ctx context.Context, state *ProcessingState) error {
	if s.indicator == nil || state.Event.ContactPhone == "" {
		return nil
	}
	if err := s.indicator.StartTyping(ctx, state.Event.ContactPhone); err != nil {
		s.logger.Warnw("typing start failed", "error", err, "phone", state.Event.ContactPhone)
	}
	return nil
}

type TypingStopStep struct {
	indicator ports.TypingIndicator
	logger    *zap.SugaredLogger
}

func NewTypingStopStep(indicator ports.TypingIndicator, logger *zap.SugaredLogger) *TypingStopStep {
	return &TypingStopStep{indicator: indicator, logger: logger}
}

func (s *TypingStopStep) Name() string           { return "TypingStop" }
func (s *TypingStopStep) Timeout() time.Duration { return 3 * time.Second }

func (s *TypingStopStep) Execute(ctx context.Context, state *ProcessingState) error {
	if s.indicator == nil || state.Event.ContactPhone == "" {
		return nil
	}
	if err := s.indicator.StopTyping(ctx, state.Event.ContactPhone); err != nil {
		s.logger.Warnw("typing stop failed", "error", err, "phone", state.Event.ContactPhone)
	}
	return nil
}
