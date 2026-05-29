package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

type ValidationStep struct {
	validator EventValidatorIface
	timeout   time.Duration
	logger    *zap.SugaredLogger
}

func NewValidationStep(validator EventValidatorIface, timeout time.Duration, logger *zap.SugaredLogger) *ValidationStep {
	return &ValidationStep{validator: validator, timeout: timeout, logger: logger}
}

func (s *ValidationStep) Name() string           { return "Validation" }
func (s *ValidationStep) Timeout() time.Duration { return s.timeout }

func (s *ValidationStep) Execute(ctx context.Context, state *ProcessingState) error {
	if err := s.validator.Validate(ctx, state.Event); err != nil {
		return fmt.Errorf("validate event: %w", err)
	}
	return nil
}

// RateLimitStep rejects requests that exceed the configured rate for a session.
// Runs after Validation and before LockAcquisition so the lock is never acquired
// for throttled requests. On Redis failure it fails open to avoid blocking traffic.
type RateLimitStep struct {
	rateLimiter ports.Limiter
	timeout     time.Duration
	logger      *zap.SugaredLogger
}

func NewRateLimitStep(rateLimiter ports.Limiter, timeout time.Duration, logger *zap.SugaredLogger) *RateLimitStep {
	return &RateLimitStep{rateLimiter: rateLimiter, timeout: timeout, logger: logger}
}

func (s *RateLimitStep) Name() string           { return "RateLimit" }
func (s *RateLimitStep) Timeout() time.Duration { return s.timeout }

func (s *RateLimitStep) Execute(ctx context.Context, state *ProcessingState) error {
	allowed, err := s.rateLimiter.Allow(ctx, state.Event.MemoryKey)
	if err != nil {
		s.logger.Warnw("rate limiter error, failing open",
			"error", err,
			"memory_key", state.Event.MemoryKey)
		return nil
	}
	if !allowed {
		s.logger.Warnw("rate limit exceeded", "memory_key", state.Event.MemoryKey)
		return ErrRateLimitExceeded(state.Event.MemoryKey)
	}
	return nil
}

// FilterStep evaluates per-event FilterRules against resolved event field values.
// Rules are defined on EventConfiguration — no global config needed.
// All rules must pass (AND). Missing or empty field values skip the rule.
type FilterStep struct {
	timeout time.Duration
	logger  *zap.SugaredLogger
}

func NewFilterStep(timeout time.Duration, logger *zap.SugaredLogger) *FilterStep {
	return &FilterStep{timeout: timeout, logger: logger}
}

func (s *FilterStep) Name() string           { return "Filter" }
func (s *FilterStep) Timeout() time.Duration { return s.timeout }

func (s *FilterStep) Execute(ctx context.Context, state *ProcessingState) error {
	if len(state.EventConfig.CompiledRules) == 0 {
		return nil
	}

	for i := range state.EventConfig.CompiledRules {
		rule := &state.EventConfig.CompiledRules[i]
		value := resolveEventField(state.Event, rule.Field)
		if value == "" {
			continue // missing field → rule skipped
		}

		if rule.IsBlocked(value) {
			s.logger.Warnw("event blocked by filter rule", "field", rule.Field, "value", value)
			return ErrFilterBlocked(rule.Field, value)
		}

		if rule.IsNotAllowed(value) {
			s.logger.Warnw("event not in filter allowlist", "field", rule.Field, "value", value)
			return ErrFilterNotAllowed(rule.Field, value)
		}
	}

	return nil
}

// resolveEventField resolves a dot-notation field path against an event.
// Top-level fields: "contact_phone", "source", "sub_type", "memory_key",
//
//	"subject_key", "event_type", "idempotency_key", "user_message"
//
// Map fields: "data.<key>", "metadata.<key>", "payload.<key>"
// Nested maps: "data.address.city"
func resolveEventField(event *entities.Pulse, field string) string {
	prefix, rest, hasDot := strings.Cut(field, ".")
	if !hasDot {
		return resolveTopLevelField(event, field)
	}
	switch prefix {
	case "knowledge":
		return resolveMapPath(event.Knowledge, rest)
	case "metadata":
		return resolveMapPath(event.Metadata, rest)
	case "payload":
		return resolveMapPath(event.Payload, rest)
	}
	return ""
}

func resolveTopLevelField(event *entities.Pulse, field string) string {
	switch field {
	case "contact_phone":
		return event.ContactPhone
	case "memory_key":
		return event.MemoryKey
	case "source":
		return event.Source
	case "sub_type":
		return event.SubType
	case "event_type":
		return event.EventType
	case "subject_key":
		return event.SubjectKey
	case "idempotency_key":
		return event.IdempotencyKey
	case "user_message":
		return event.UserMessage
	}
	return ""
}

func resolveMapPath(m map[string]any, path string) string {
	key, rest, nested := strings.Cut(path, ".")
	val, ok := m[key]
	if !ok || val == nil {
		return ""
	}
	if !nested {
		switch v := val.(type) {
		case string:
			return v
		case fmt.Stringer:
			return v.String()
		default:
			return ""
		}
	}
	sub, ok := val.(map[string]any)
	if !ok {
		return ""
	}
	return resolveMapPath(sub, rest)
}
