package orchestrator

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// injectionPatterns lists explicit prompt injection attempts.
// Deliberately conservative — creative prompts must not be flagged.
var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+|the\s+)?(previous|above|prior)\s+instruction`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+|the\s+)?(previous|above|prior)`),
	regexp.MustCompile(`(?i)forget\s+(all\s+|the\s+)?(previous|above|prior)\s+instruction`),
	regexp.MustCompile(`(?i)<\|im_start\|>`),         // OpenAI chat-format injection
	regexp.MustCompile(`(?i)\[INST\]|\[/INST\]`),     // Llama format injection
	regexp.MustCompile("[\x00]"),                     // null bytes
	regexp.MustCompile("[\u202E\u202D]"),             // RTL/LTR override characters
	regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]"), // zero-width characters
}

type EventValidator struct {
	config *WeaveConfig
	logger *zap.SugaredLogger
	tracer trace.Tracer
}

func NewEventValidator(config *WeaveConfig, logger *zap.SugaredLogger, tracer trace.Tracer) *EventValidator {
	return &EventValidator{
		config: config,
		logger: logger,
		tracer: tracer,
	}
}

func (v *EventValidator) Validate(ctx context.Context, event *entities.Pulse) error {
	_, span := v.tracer.Start(ctx, "EventValidator/Validate")
	defer span.End()

	if event.MemoryKey == "" {
		return ErrValidationFailed("memory_key", "cannot be empty")
	}

	if len(event.MemoryKey) > v.config.MaxSessionKeyLength {
		return ErrValidationFailed("memory_key",
			"exceeds maximum length")
	}

	if len(event.UserMessage) > v.config.MaxUserMessageLength {
		return ErrValidationFailed("user_message",
			"exceeds maximum length")
	}

	if event.UserMessage != "" {
		if err := v.validateInputGuard(event.UserMessage); err != nil {
			return err
		}
	}

	if event.Knowledge != nil {
		dataSize, err := v.estimateContextSize(event.Knowledge)
		if err != nil {
			v.logger.Warnw("failed to estimate knowledge size", "error", err)
		} else if dataSize > v.config.MaxEventContextSize {
			return ErrValidationFailed("knowledge",
				"exceeds maximum size")
		}
	}

	if event.ID == "" {
		return ErrValidationFailed("id", "cannot be empty")
	}

	return nil
}

func (v *EventValidator) validateInputGuard(message string) error {
	guard := v.config.InputGuard

	if guard.MaxLineCount > 0 && strings.Count(message, "\n") >= guard.MaxLineCount {
		return ErrValidationFailed("user_message", "exceeds maximum line count")
	}

	if guard.PromptInjectionDetection {
		for _, pattern := range injectionPatterns {
			if pattern.MatchString(message) {
				v.logger.Warnw("prompt injection pattern detected",
					"pattern", pattern.String())
				return ErrPromptInjectionDetected()
			}
		}

		for _, r := range message {
			if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
				return ErrValidationFailed("user_message", "contains disallowed control characters")
			}
		}
	}

	return nil
}

func (v *EventValidator) estimateContextSize(context map[string]any) (int, error) {
	data, err := json.Marshal(context)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func (v *EventValidator) ValidateEventConfiguration(config *entities.Link) error {
	if config.EventType == "" {
		return ErrValidationFailed("event_type", "cannot be empty")
	}

	if config.InboundConverterName == "" {
		return ErrValidationFailed("inbound_converter_name", "cannot be empty")
	}

	if len(config.AllowedSpirits) == 0 && config.DefaultSpirit == "" {
		return ErrValidationFailed("spirits", "must specify at least one allowed spirit or a default spirit")
	}

	return nil
}
