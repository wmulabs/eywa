package orchestrator

import (
	"context"
	"errors"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

const ritualStepTimeout = 5 * time.Second

// RitualMarkStep transitions a ritual to running after lock acquisition.
// Terminal statuses abort the pipeline without retrying — transient DB errors retry.
type RitualMarkStep struct {
	manager ports.RitualManager
	timeout time.Duration
	logger  *zap.SugaredLogger
}

func NewRitualMarkStep(manager ports.RitualManager, timeout time.Duration, logger *zap.SugaredLogger) *RitualMarkStep {
	return &RitualMarkStep{manager: manager, timeout: timeout, logger: logger}
}

func (s *RitualMarkStep) Name() string           { return "RitualMark" }
func (s *RitualMarkStep) Timeout() time.Duration { return s.timeout }

func (s *RitualMarkStep) Execute(ctx context.Context, state *ProcessingState) error {
	taskID, ok := state.Event.GetMetadataString(entities.MetadataKeyRitualID)
	if !ok || taskID == "" {
		return nil
	}

	if err := s.manager.MarkRunning(ctx, taskID); err != nil {
		if errors.Is(err, domainerrors.ErrRitualTerminal) {
			s.logger.Warnw("task already terminal, skipping",
				"task_id", taskID,
				"memory_key", state.Event.MemoryKey,
				"error", err,
			)
			return &OrchestrationError{Code: "SCHEDULED_TASK_SKIP", Message: err.Error(), Retriable: false}
		}
		s.logger.Warnw("failed to mark task as running, proceeding anyway",
			"task_id", taskID,
			"memory_key", state.Event.MemoryKey,
			"error", err,
		)
	}

	return nil
}

type MarkExecutedStep struct {
	manager ports.RitualManager
	timeout time.Duration
	logger  *zap.SugaredLogger
}

func NewMarkExecutedStep(manager ports.RitualManager, timeout time.Duration, logger *zap.SugaredLogger) *MarkExecutedStep {
	return &MarkExecutedStep{manager: manager, timeout: timeout, logger: logger}
}

func (s *MarkExecutedStep) Name() string           { return "MarkExecuted" }
func (s *MarkExecutedStep) Timeout() time.Duration { return s.timeout }

func (s *MarkExecutedStep) Execute(ctx context.Context, state *ProcessingState) error {
	taskID, ok := state.Event.GetMetadataString(entities.MetadataKeyRitualID)
	if !ok || taskID == "" {
		return nil
	}

	if err := s.manager.MarkExecuted(ctx, taskID); err != nil {
		s.logger.Warnw("failed to mark task as executed",
			"task_id", taskID,
			"memory_key", state.Event.MemoryKey,
			"error", err,
		)
	}

	return nil
}
