package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

type PersistenceStep struct {
	memoryManager  MemoryManager
	messageManager MessageManager
	timeout        time.Duration
	logger         *zap.SugaredLogger
}

func NewPersistenceStep(
	memoryManager MemoryManager,
	messageManager MessageManager,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *PersistenceStep {
	return &PersistenceStep{
		memoryManager:  memoryManager,
		messageManager: messageManager,
		timeout:        timeout,
		logger:         logger,
	}
}

func (s *PersistenceStep) Name() string           { return "Persistence" }
func (s *PersistenceStep) Timeout() time.Duration { return s.timeout }

func (s *PersistenceStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Session == nil {
		return nil
	}

	if state.Response != "" {
		s.addAssistantMessageToMemory(ctx, state.Session, state.Response)
	}

	if err := s.memoryManager.Save(ctx, state.Session); err != nil {
		return ErrPersistenceFailed("save_memory", err)
	}

	topicKey := state.Session.SubjectKey

	if state.Event.UserMessage != "" {
		s.appendMessage(ctx, state.Event.MemoryKey, topicKey, ports.RoleUser, state.Event.UserMessage)
	}

	if state.Response != "" {
		s.appendMessage(ctx, state.Event.MemoryKey, topicKey, ports.RoleAssistant, state.Response)
	}

	return nil
}

func (s *PersistenceStep) addAssistantMessageToMemory(ctx context.Context, session *entities.Memory, response string) {
	err := s.memoryManager.UpdateMemory(ctx, session, entities.Thread{
		Role:         ports.RoleAssistant,
		Content:      response,
		IsUserFacing: true,
	})
	if err != nil {
		s.logger.Warnw("failed to update session memory with assistant response", "error", err)
	}
}

// appendMessage persists on a best-effort basis — failure is logged but not propagated.
func (s *PersistenceStep) appendMessage(ctx context.Context, memoryKey, topicKey, role, content string) {
	if err := s.messageManager.Append(ctx, entities.Echo{
		MemoryKey:    memoryKey,
		SubjectKey:   topicKey,
		Role:         role,
		Content:      content,
		IsUserFacing: true,
	}); err != nil {
		s.logger.Errorw("failed to append message to store (non-critical)",
			"role", role,
			"memory_key", memoryKey,
			"error", err)
	}
}

// AuditLogStep logs the interaction for audit — always non-blocking, failure is swallowed.
type AuditLogStep struct {
	interactionLogger InteractionLogger
	timeout           time.Duration
	logger            *zap.SugaredLogger
}

func NewAuditLogStep(interactionLogger InteractionLogger, timeout time.Duration, logger *zap.SugaredLogger) *AuditLogStep {
	return &AuditLogStep{interactionLogger: interactionLogger, timeout: timeout, logger: logger}
}

func (s *AuditLogStep) Name() string           { return "AuditLog" }
func (s *AuditLogStep) Timeout() time.Duration { return s.timeout }

func (s *AuditLogStep) Execute(ctx context.Context, state *ProcessingState) error {
	if err := s.interactionLogger.logInteraction(ctx, state); err != nil {
		s.logger.Errorw("failed to log interaction", "error", err)
	}
	return nil
}
