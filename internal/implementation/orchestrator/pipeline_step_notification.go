package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// NotificationStep handles response delivery for notifier Spirits.
// In "template" mode it substitutes {{field}} tokens from Pulse.Knowledge and sends immediately.
// In "llm" mode it makes a single Oracle call (no tool loop) and sends the result.
// Skipped when state.Skipped is true (conditions not met) or Spirit is not a notifier.
type NotificationStep struct {
	voiceRegistry ports.VoiceRegistry
	oracleFactory ports.OracleFactory
	timeout       time.Duration
	logger        *zap.SugaredLogger
}

func NewNotificationStep(
	voiceRegistry ports.VoiceRegistry,
	oracleFactory ports.OracleFactory,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *NotificationStep {
	return &NotificationStep{
		voiceRegistry: voiceRegistry,
		oracleFactory: oracleFactory,
		timeout:       timeout,
		logger:        logger,
	}
}

func (s *NotificationStep) Name() string           { return "Notification" }
func (s *NotificationStep) Timeout() time.Duration { return s.timeout }

func (s *NotificationStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Spirit == nil || !state.Spirit.IsNotifier() {
		return nil
	}
	if state.Skipped {
		return nil
	}

	voiceName := state.EventConfig.VoiceName
	if voiceName == "" {
		s.logger.Warnw("notifier Spirit has no Voice configured, skipping notification",
			"spirit", state.Spirit.Name,
			"memory_key", state.Event.MemoryKey)
		return nil
	}

	voice, err := s.voiceRegistry.Get(voiceName)
	if err != nil {
		s.logger.Warnw("notifier Voice not found",
			"voice", voiceName,
			"spirit", state.Spirit.Name)
		return nil
	}

	cfg := state.Spirit.NotifierConfig
	var message string

	switch cfg.Mode {
	case "template":
		message = renderTemplate(cfg.Template, state.Event.Knowledge)
	case "llm":
		message, err = s.callOracle(ctx, state)
		if err != nil {
			s.logger.Errorw("notifier LLM call failed",
				"spirit", state.Spirit.Name,
				"error", err)
			return nil
		}
	default:
		s.logger.Warnw("unknown notifier mode, skipping",
			"mode", cfg.Mode,
			"spirit", state.Spirit.Name)
		return nil
	}

	if message == "" {
		s.logger.Infow("notifier produced empty message, skipping delivery",
			"spirit", state.Spirit.Name,
			"memory_key", state.Event.MemoryKey)
		state.Skipped = true
		state.ProcessingStatus = "skipped"
		return nil
	}

	state.Response = message

	if err := voice.SendResponse(ctx, state.Event, message); err != nil {
		s.logger.Errorw("notifier failed to send response",
			"voice", voiceName,
			"spirit", state.Spirit.Name,
			"error", err)
		return nil
	}

	state.ResponseDelivered = true
	s.logger.Infow("notifier response sent",
		"spirit", state.Spirit.Name,
		"mode", cfg.Mode,
		"memory_key", state.Event.MemoryKey)

	return nil
}

func renderTemplate(template string, knowledge map[string]any) string {
	result := template
	for k, v := range knowledge {
		token := "{{" + k + "}}"
		result = strings.ReplaceAll(result, token, toString(v))
	}
	return result
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if n, ok := v.(float64); ok && n == float64(int64(n)) {
		return fmt.Sprintf("%d", int64(n))
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

func (s *NotificationStep) callOracle(ctx context.Context, state *ProcessingState) (string, error) {
	provider, err := s.oracleFactory.GetProvider(state.Spirit.ModelConfig.Provider)
	if err != nil {
		return "", fmt.Errorf("get oracle provider: %w", err)
	}

	// Use buildSystemPrompt so the notifier sees the event Knowledge — same context the template
	// mode renders from. Without it, webhook-triggered notifiers (empty UserMessage) have nothing to act on.
	req := ports.OracleRequest{
		Model:        state.Spirit.ModelConfig.Model,
		Temperature:  state.Spirit.ModelConfig.Temperature,
		SystemPrompt: buildSystemPrompt(state.Spirit, state.Event),
		Messages: []ports.OracleMessage{
			{
				Role:    ports.RoleUser,
				Content: state.Event.UserMessage,
			},
		},
	}

	resp, err := provider.GenerateResponse(ctx, &req)
	if err != nil {
		return "", fmt.Errorf("generate oracle response: %w", err)
	}

	if state.ReasoningResult == nil {
		state.ReasoningResult = &ReasoningResult{}
	}
	state.ReasoningResult.accumulateTokens(resp.TokensUsed)

	return resp.Content, nil
}
