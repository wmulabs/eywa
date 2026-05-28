package orchestrator

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

const defaultMaxDepth = 3

// SummonService executes a sub-Spirit pipeline on behalf of an orchestrator Spirit.
// It bypasses locking, inbox, and session setup — sub-tasks run stateless and return a result string.
type SummonService struct {
	spiritRepo       ports.SpiritRepository
	scoutRegistry    ports.ScoutRegistry
	reasoningService ReasoningExecutor
	logger           *zap.SugaredLogger
}

func NewSummonService(
	spiritRepo ports.SpiritRepository,
	scoutRegistry ports.ScoutRegistry,
	reasoningService ReasoningExecutor,
	logger *zap.SugaredLogger,
) *SummonService {
	return &SummonService{
		spiritRepo:       spiritRepo,
		scoutRegistry:    scoutRegistry,
		reasoningService: reasoningService,
		logger:           logger,
	}
}

func (s *SummonService) Summon(
	ctx context.Context,
	spiritName string,
	task string,
	parent *entities.Pulse,
	parentSpirit *entities.Spirit,
) (string, error) {
	if err := s.validateSummon(spiritName, parent, parentSpirit); err != nil {
		return "", err
	}

	subSpirit, err := s.spiritRepo.FindActiveByName(ctx, spiritName)
	if err != nil {
		return "", domainerrors.NewInfrastructureError(fmt.Sprintf("summon: spirit %q not found", spiritName), err)
	}

	childPulse := s.buildChildPulse(task, parent)

	if s.scoutRegistry != nil && len(subSpirit.RequireScouts) > 0 {
		scouts, err := s.scoutRegistry.GetMultiple(subSpirit.RequireScouts)
		if err == nil {
			for _, scout := range scouts {
				_ = runScout(ctx, scout, childPulse, s.logger)
			}
		}
	}

	req := &ReasoningRequest{
		Event:        childPulse,
		Spirit:       subSpirit,
		Session:      &entities.Memory{},
		SystemPrompt: buildSystemPrompt(subSpirit, childPulse),
	}

	result, err := s.reasoningService.Execute(ctx, req)
	if err != nil {
		return "", domainerrors.NewInfrastructureError(fmt.Sprintf("summon: spirit %q reasoning failed", spiritName), err)
	}

	return result.FinalResponse, nil
}

func (s *SummonService) validateSummon(spiritName string, parent *entities.Pulse, parentSpirit *entities.Spirit) error {
	allowed := parentSpirit.OrchestratorConfig.SubSpirits
	found := false
	for _, name := range allowed {
		if name == spiritName {
			found = true
			break
		}
	}
	if !found {
		return domainerrors.NewBusinessError(
			fmt.Sprintf("spirit %q is not in the allowed sub-spirits list", spiritName),
		)
	}

	maxDepth := parentSpirit.OrchestratorConfig.MaxDepth
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}
	if parent.OrchestrationDepth >= maxDepth {
		return domainerrors.NewBusinessError(
			fmt.Sprintf("orchestration depth limit (%d) reached", maxDepth),
		)
	}

	return nil
}

func (s *SummonService) buildChildPulse(task string, parent *entities.Pulse) *entities.Pulse {
	child := &entities.Pulse{
		MemoryKey:          parent.MemoryKey,
		ContactPhone:       parent.ContactPhone,
		Source:             parent.Source,
		EventType:          parent.EventType,
		UserMessage:        task,
		ParentPulseID:      parent.ID,
		OrchestrationDepth: parent.OrchestrationDepth + 1,
		Knowledge:          make(map[string]any),
	}
	for k, v := range parent.Knowledge {
		child.Knowledge[k] = v
	}
	return child
}
