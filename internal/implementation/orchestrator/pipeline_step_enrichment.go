package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// LinkScoutStep runs the Scouts declared on the Link before Spirit selection.
// These enrich the Pulse with routing context (user tier, session history, etc.)
// that the Pathfinder may need to select the right Spirit.
type LinkScoutStep struct {
	scoutRegistry   ports.ScoutRegistry
	distributedLock ports.Bond
	lockTTL         time.Duration
	timeout         time.Duration
	logger          *zap.SugaredLogger
}

func NewLinkScoutStep(
	scoutRegistry ports.ScoutRegistry,
	distributedLock ports.Bond,
	lockTTL time.Duration,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *LinkScoutStep {
	return &LinkScoutStep{
		scoutRegistry:   scoutRegistry,
		distributedLock: distributedLock,
		lockTTL:         lockTTL,
		timeout:         timeout,
		logger:          logger,
	}
}

func (s *LinkScoutStep) Name() string           { return "LinkScout" }
func (s *LinkScoutStep) Timeout() time.Duration { return s.timeout }

func (s *LinkScoutStep) Execute(ctx context.Context, state *ProcessingState) error {
	if s.scoutRegistry == nil || state.EventConfig == nil || len(state.EventConfig.RequireScouts) == 0 {
		return nil
	}

	originalMemoryKey := state.Event.MemoryKey

	scouts, err := s.scoutRegistry.GetMultiple(state.EventConfig.RequireScouts)
	if err != nil {
		return ErrScoutFailed(err)
	}

	for _, scout := range scouts {
		if err := runScout(ctx, scout, state.Event, s.logger); err != nil {
			return err
		}
	}

	if state.Event.MemoryKey != originalMemoryKey {
		s.logger.Infow("memory_key changed during link enrichment, acquiring additional lock",
			"original_memory_key", originalMemoryKey,
			"new_memory_key", state.Event.MemoryKey)

		newLockKey := fmt.Sprintf("memory:%s", state.Event.MemoryKey)
		acquired, err := s.distributedLock.AcquireLock(ctx, newLockKey, s.lockTTL)
		if err != nil || !acquired {
			return ErrLockAcquisitionFailed(state.Event.MemoryKey, err)
		}

		state.AdditionalLocks = append(state.AdditionalLocks, newLockKey)
		s.logger.Infow("additional lock acquired after link scout", "lock_key", newLockKey)
	}

	return nil
}

// SpiritScoutStep runs the Scouts declared on the Spirit after Spirit selection.
// These load domain-specific data that only the selected Spirit needs
// (order details, billing records, delivery history, etc.).
// When a LoreHarvester is configured and the Spirit has LoreIDs, it also runs RAG enrichment.
// When an ImprintHarvester is configured, it injects long-term user facts for conversational/orchestrator Spirits.
type SpiritScoutStep struct {
	scoutRegistry    ports.ScoutRegistry
	loreHarvester    ports.LoreHarvester    // optional; nil disables RAG enrichment
	imprintHarvester ports.ImprintHarvester // optional; nil disables long-term memory
	timeout          time.Duration
	logger           *zap.SugaredLogger
}

func NewSpiritScoutStep(
	scoutRegistry ports.ScoutRegistry,
	loreHarvester ports.LoreHarvester,
	imprintHarvester ports.ImprintHarvester,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *SpiritScoutStep {
	return &SpiritScoutStep{
		scoutRegistry:    scoutRegistry,
		loreHarvester:    loreHarvester,
		imprintHarvester: imprintHarvester,
		timeout:          timeout,
		logger:           logger,
	}
}

func (s *SpiritScoutStep) Name() string           { return "SpiritScout" }
func (s *SpiritScoutStep) Timeout() time.Duration { return s.timeout }

func (s *SpiritScoutStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Spirit == nil {
		return nil
	}

	if s.scoutRegistry != nil && len(state.Spirit.RequireScouts) > 0 {
		scouts, err := s.scoutRegistry.GetMultiple(state.Spirit.RequireScouts)
		if err != nil {
			return ErrScoutFailed(err)
		}
		for _, scout := range scouts {
			if err := runScout(ctx, scout, state.Event, s.logger); err != nil {
				return err
			}
		}
	}

	if s.loreHarvester != nil && len(state.Spirit.LoreIDs) > 0 {
		if err := s.loreHarvester.Harvest(ctx, state.Event, state.Spirit.LoreIDs); err != nil {
			s.logger.Warnw("lore harvest failed, continuing without RAG context",
				"spirit", state.Spirit.Name,
				"error", err)
		}
	}

	if s.imprintHarvester != nil && (state.Spirit.IsConversational() || state.Spirit.IsOrchestrator()) {
		if err := s.imprintHarvester.Harvest(ctx, state.Event, state.Spirit.Name); err != nil {
			s.logger.Warnw("imprint harvest failed, continuing without user memory",
				"spirit", state.Spirit.Name,
				"error", err)
		}
	}

	return nil
}

func runScout(ctx context.Context, scout ports.Scout, pulse *entities.Pulse, logger *zap.SugaredLogger) error {
	if !scout.IsApplicable(pulse) {
		logger.Debugw("Scout not applicable", "scout", scout.GetName(), "event_id", pulse.ID)
		return nil
	}

	logger.Infow("running Scout", "scout", scout.GetName(), "event_id", pulse.ID)

	if err := scout.Harvest(ctx, pulse); err != nil {
		logger.Errorw("Scout failed, continuing pipeline", "scout", scout.GetName(), "error", err, "event_id", pulse.ID)
		return nil // fail-open: Scout errors are non-fatal per documented contract
	}

	return nil
}
