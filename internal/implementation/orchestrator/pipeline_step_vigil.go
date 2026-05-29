package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// VigilCheckStep checks whether an operator holds the session.
// Inserted immediately after LockAcquisitionStep. Fail-open: a repo error logs a warning
// and allows processing to continue — infrastructure failure must not block the user.
type VigilCheckStep struct {
	vigilRepo ports.VigilRepository
	logger    *zap.SugaredLogger
}

func NewVigilCheckStep(vigilRepo ports.VigilRepository, logger *zap.SugaredLogger) *VigilCheckStep {
	return &VigilCheckStep{vigilRepo: vigilRepo, logger: logger}
}

func (s *VigilCheckStep) Name() string           { return "VigilCheck" }
func (s *VigilCheckStep) Timeout() time.Duration { return 3 * time.Second }

func (s *VigilCheckStep) Execute(ctx context.Context, state *ProcessingState) error {
	if s.vigilRepo == nil {
		return nil
	}
	vigil, err := s.vigilRepo.Get(ctx, state.Event.MemoryKey)
	if err != nil {
		s.logger.Warnw("vigil check failed, allowing processing", "error", err, "memory_key", state.Event.MemoryKey)
		return nil // fail-open
	}
	if vigil != nil {
		state.ProcessingStatus = "session_held"
		return newErrSessionHeld()
	}
	return nil
}
