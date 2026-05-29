package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// ResponseDeliveryStep ensures the user receives a response when no
// ResponseDelivery-category tool was called during reasoning.
// If ResponseChannel is configured and ShouldAutoRespond is true,
// it sends the final LLM response directly via the channel.
type ResponseDeliveryStep struct {
	responseChannelRegistry ports.VoiceRegistry
	timeout                 time.Duration
	logger                  *zap.SugaredLogger
}

func NewResponseDeliveryStep(
	responseChannelRegistry ports.VoiceRegistry,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *ResponseDeliveryStep {
	return &ResponseDeliveryStep{
		responseChannelRegistry: responseChannelRegistry,
		timeout:                 timeout,
		logger:                  logger,
	}
}

func (s *ResponseDeliveryStep) Name() string           { return "ResponseDelivery" }
func (s *ResponseDeliveryStep) Timeout() time.Duration { return s.timeout }

func (s *ResponseDeliveryStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.ResponseDelivered {
		s.logger.Infow("response already delivered during reasoning, skipping auto-response",
			"memory_key", state.Event.MemoryKey)
		return nil
	}

	if state.EventConfig.VoiceName == "" {
		s.logger.Debugw("no response channel configured, skipping auto-response")
		return nil
	}

	if state.Response == "" {
		s.logger.Debugw("no final response available, skipping auto-response")
		return nil
	}

	channel, err := s.responseChannelRegistry.Get(state.EventConfig.VoiceName)
	if err != nil {
		s.logger.Warnw("response channel not found",
			"channel_name", state.EventConfig.VoiceName,
			"error", err)
		return nil // Non-critical: don't fail the pipeline
	}

	if !channel.ShouldAutoRespond() {
		s.logger.Debugw("channel does not auto-respond, skipping", "channel", channel.GetName())
		return nil
	}

	shouldForce := s.shouldForceResponse(state)
	if shouldForce {
		s.logger.Warnw("forcing response delivery after max iterations",
			"channel", channel.GetName(),
			"iterations_used", state.ReasoningResult.IterationsUsed,
			"memory_key", state.Event.MemoryKey)
	}

	s.logger.Infow("sending automatic response",
		"channel", channel.GetName(),
		"memory_key", state.Event.MemoryKey,
		"forced", shouldForce)

	if err := channel.SendResponse(ctx, state.Event, state.Response); err != nil {
		s.logger.Errorw("failed to send automatic response",
			"channel", channel.GetName(),
			"error", err,
			"memory_key", state.Event.MemoryKey)
		return nil
	}

	state.ResponseDelivered = true

	s.logger.Infow("automatic response sent successfully",
		"channel", channel.GetName(),
		"metadata", channel.GetChannelMetadata(state.Event))

	return nil
}

func (s *ResponseDeliveryStep) shouldForceResponse(state *ProcessingState) bool {
	if state.ReasoningResult == nil {
		return false
	}
	return state.ReasoningResult.FinalError != "" &&
		len(state.ReasoningResult.Iterations) > 0
}
