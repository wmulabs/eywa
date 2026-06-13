package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// IdempotencyStep drops Pulses whose IdempotencyKey was already processed within the TTL window.
// Runs after Validation and before RateLimit so duplicate redeliveries are rejected before any
// lock, enrichment, or Oracle cost is incurred. Events without an IdempotencyKey are passed through.
// On store failure it fails open (processes the event) to avoid dropping legitimate traffic.
type IdempotencyStep struct {
	store   ports.IdempotencyStore
	ttl     time.Duration
	timeout time.Duration
	logger  *zap.SugaredLogger
}

func NewIdempotencyStep(store ports.IdempotencyStore, ttl, timeout time.Duration, logger *zap.SugaredLogger) *IdempotencyStep {
	return &IdempotencyStep{store: store, ttl: ttl, timeout: timeout, logger: logger}
}

func (s *IdempotencyStep) Name() string           { return "IdempotencyCheck" }
func (s *IdempotencyStep) Timeout() time.Duration { return s.timeout }

func (s *IdempotencyStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Event.IdempotencyKey == "" {
		return nil
	}

	seen, err := s.store.Seen(ctx, state.Event.IdempotencyKey, s.ttl)
	if err != nil {
		s.logger.Warnw("idempotency store error, failing open",
			"error", err,
			"memory_key", state.Event.MemoryKey,
			"idempotency_key", state.Event.IdempotencyKey)
		return nil
	}
	if seen {
		s.logger.Infow("duplicate event dropped",
			"memory_key", state.Event.MemoryKey,
			"idempotency_key", state.Event.IdempotencyKey)
		return ErrDuplicateEvent(state.Event.IdempotencyKey)
	}
	return nil
}
