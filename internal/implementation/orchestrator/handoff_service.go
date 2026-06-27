package orchestrator

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

const defaultMaxHandoffs = 3

// HandoffService transfers control of a conversation from one Spirit to a peer (transfer-and-continue).
// Unlike SummonService (call-and-return within the turn), it pins the target as the session's active
// Spirit via HandoffStore so subsequent turns route to it directly, then runs the target's reasoning to
// answer the current turn.
type HandoffService struct {
	spiritRepo       ports.SpiritRepository
	scoutRegistry    ports.ScoutRegistry
	reasoningService ReasoningExecutor
	handoffStore     ports.HandoffStore
	archivist        ports.Archivist // optional; enables ContextTransfer "summary"
	logger           *zap.SugaredLogger
}

func NewHandoffService(
	spiritRepo ports.SpiritRepository,
	scoutRegistry ports.ScoutRegistry,
	reasoningService ReasoningExecutor,
	handoffStore ports.HandoffStore,
	archivist ports.Archivist,
	logger *zap.SugaredLogger,
) *HandoffService {
	return &HandoffService{
		spiritRepo:       spiritRepo,
		scoutRegistry:    scoutRegistry,
		reasoningService: reasoningService,
		handoffStore:     handoffStore,
		archivist:        archivist,
		logger:           logger,
	}
}

// Handoff validates the transfer, pins the target as the session's active Spirit, and runs the target's
// reasoning for the current turn. parentSession and parentContext carry the conversation so far; what
// the target inherits is governed by parentSpirit.HandoffConfig.ContextTransfer.
func (s *HandoffService) Handoff(
	ctx context.Context,
	target string,
	parent *entities.Pulse,
	parentSpirit *entities.Spirit,
	parentSession *entities.Memory,
	parentContext []ports.OracleMessage,
) (*ReasoningResult, error) {
	if err := s.validateHandoff(target, parent, parentSpirit); err != nil {
		return nil, err
	}

	targetSpirit, err := s.spiritRepo.FindActiveByName(ctx, target)
	if err != nil {
		return nil, domainerrors.NewInfrastructureError(fmt.Sprintf("handoff: spirit %q not found", target), err)
	}

	// Pin first so subsequent turns route to the target even if this turn's reasoning then fails.
	if err := s.handoffStore.SetActiveSpirit(ctx, parent.MemoryKey, target); err != nil {
		return nil, domainerrors.NewInfrastructureError(fmt.Sprintf("handoff: pin spirit %q", target), err)
	}

	childPulse := s.buildChildPulse(parent)

	if s.scoutRegistry != nil && len(targetSpirit.RequireScouts) > 0 {
		scouts, err := s.scoutRegistry.GetMultiple(targetSpirit.RequireScouts)
		if err == nil {
			for _, scout := range scouts {
				_ = runScout(ctx, scout, childPulse, s.logger)
			}
		}
	}

	session, convCtx, promptAddendum := s.transferContext(ctx, parentSpirit, parentSession, parentContext)

	req := &ReasoningRequest{
		Event:               childPulse,
		Spirit:              targetSpirit,
		Session:             session,
		SystemPrompt:        buildSystemPrompt(targetSpirit, childPulse) + promptAddendum,
		ConversationContext: convCtx,
		ResponseFormat:      targetSpirit.ResponseFormat,
	}

	result, err := s.reasoningService.Execute(ctx, req)
	if err != nil {
		return nil, domainerrors.NewInfrastructureError(fmt.Sprintf("handoff: spirit %q reasoning failed", target), err)
	}

	s.logger.Infow("handoff completed",
		"from", parentSpirit.Name,
		"to", target,
		"memory_key", parent.MemoryKey,
		"context_transfer", parentSpirit.HandoffConfig.ContextTransfer)

	return result, nil
}

func (s *HandoffService) validateHandoff(target string, parent *entities.Pulse, parentSpirit *entities.Spirit) error {
	allowed := false
	for _, name := range parentSpirit.HandoffConfig.AllowedTargets {
		if name == target {
			allowed = true
			break
		}
	}
	if !allowed {
		return domainerrors.NewBusinessError(fmt.Sprintf("spirit %q is not an allowed handoff target", target))
	}

	maxHandoffs := parentSpirit.HandoffConfig.MaxHandoffs
	if maxHandoffs <= 0 {
		maxHandoffs = defaultMaxHandoffs
	}
	if parent.HandoffCount >= maxHandoffs {
		return domainerrors.NewBusinessError(fmt.Sprintf("handoff limit (%d) reached for this turn", maxHandoffs))
	}

	return nil
}

// transferContext resolves what the target inherits per the parent's ContextTransfer mode. It returns
// the session, conversation context, and an optional system-prompt addendum (used by "summary").
func (s *HandoffService) transferContext(
	ctx context.Context,
	parentSpirit *entities.Spirit,
	parentSession *entities.Memory,
	parentContext []ports.OracleMessage,
) (*entities.Memory, []ports.OracleMessage, string) {
	switch parentSpirit.HandoffConfig.ContextTransfer {
	case entities.HandoffContextSession:
		return parentSession, parentContext, ""

	case entities.HandoffContextSummary:
		if s.archivist == nil || parentSession == nil || len(parentSession.Threads) == 0 {
			s.logger.Warnw("handoff summary requested but unavailable, passing full session",
				"spirit", parentSpirit.Name)
			return parentSession, parentContext, ""
		}
		summary, err := s.archivist.Summarize(ctx, parentSession.MemoryKey, parentSession.SubjectKey, parentSession.Threads)
		if err != nil {
			s.logger.Warnw("handoff summary failed, passing full session", "error", err)
			return parentSession, parentContext, ""
		}
		session := &entities.Memory{MemoryKey: parentSession.MemoryKey, SubjectKey: parentSession.SubjectKey}
		return session, nil, "\n\nEarlier conversation summary:\n" + summary

	default: // HandoffContextNone or unset
		return &entities.Memory{}, nil, ""
	}
}

func (s *HandoffService) buildChildPulse(parent *entities.Pulse) *entities.Pulse {
	child := &entities.Pulse{
		MemoryKey:     parent.MemoryKey,
		ContactPhone:  parent.ContactPhone,
		Source:        parent.Source,
		EventType:     parent.EventType,
		UserMessage:   parent.UserMessage,
		Attachments:   parent.Attachments,
		ParentPulseID: parent.ID,
		HandoffCount:  parent.HandoffCount + 1,
		Knowledge:     make(map[string]any),
	}
	for k, v := range parent.Knowledge {
		child.Knowledge[k] = v
	}
	return child
}
