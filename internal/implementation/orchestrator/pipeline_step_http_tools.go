package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

type HTTPToolLoadStep struct {
	httpToolRepo   ports.HTTPToolRepository
	actionRegistry ports.ActionRegistry
	timeout        time.Duration
	logger         *zap.SugaredLogger
}

func NewHTTPToolLoadStep(
	repo ports.HTTPToolRepository,
	registry ports.ActionRegistry,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *HTTPToolLoadStep {
	return &HTTPToolLoadStep{
		httpToolRepo:   repo,
		actionRegistry: registry,
		timeout:        timeout,
		logger:         logger,
	}
}

func (s *HTTPToolLoadStep) Name() string           { return "HTTPToolLoad" }
func (s *HTTPToolLoadStep) Timeout() time.Duration { return s.timeout }

func (s *HTTPToolLoadStep) Execute(ctx context.Context, state *ProcessingState) error {
	tools, err := s.httpToolRepo.FindBySpiritID(ctx, state.Spirit.ID)
	if err != nil {
		s.logger.Warnw("failed to load HTTP tools for spirit, skipping",
			"spirit_id", state.Spirit.ID, "error", err)
		return nil
	}

	for _, tool := range tools {
		executor := NewHTTPToolExecutor(tool)
		if err := s.actionRegistry.Register(executor); err != nil {
			// Already registered from a previous cycle — still append name so the Spirit can call it.
			s.logger.Warnw("HTTP tool already registered", "tool_name", tool.Name)
		}
		state.Spirit.AllowedActions = append(state.Spirit.AllowedActions, entities.AllowedAction{Name: tool.Name})
	}

	return nil
}
