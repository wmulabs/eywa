package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
	"go.uber.org/zap"
)

// ImprintExtractionStep extracts user facts from the completed conversation asynchronously.
// It runs a single cheap Oracle call to identify memorable facts, deduplicates against
// existing imprints, saves new ones, then prunes to enforce MaxImprints.
// Runs as a best-effort goroutine — pipeline does not wait for it.
type ImprintExtractionStep struct {
	imprintRepo   ports.ImprintRepository
	oracleFactory ports.OracleFactory
	cfg           ImprintExtractionConfig
	timeout       time.Duration
	logger        *zap.SugaredLogger
}

func NewImprintExtractionStep(
	imprintRepo ports.ImprintRepository,
	oracleFactory ports.OracleFactory,
	cfg ImprintExtractionConfig,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *ImprintExtractionStep {
	return &ImprintExtractionStep{
		imprintRepo:   imprintRepo,
		oracleFactory: oracleFactory,
		cfg:           cfg,
		timeout:       timeout,
		logger:        logger,
	}
}

func (s *ImprintExtractionStep) Name() string           { return "ImprintExtraction" }
func (s *ImprintExtractionStep) Timeout() time.Duration { return 0 } // async — no pipeline timeout

func (s *ImprintExtractionStep) Execute(ctx context.Context, state *ProcessingState) error {
	if !s.cfg.Enabled {
		return nil
	}
	if s.imprintRepo == nil || s.oracleFactory == nil {
		return nil
	}
	if state.Spirit == nil || (!state.Spirit.IsConversational() && !state.Spirit.IsOrchestrator()) {
		return nil
	}
	if state.Response == "" || state.Event.ContactPhone == "" {
		return nil
	}

	spiritName := state.Spirit.Name
	userKey := state.Event.ContactPhone
	conversation := fmt.Sprintf("User: %s\nAssistant: %s", state.Event.UserMessage, state.Response)

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()

		s.extract(bgCtx, userKey, spiritName, conversation)
	}()

	return nil
}

func (s *ImprintExtractionStep) extract(ctx context.Context, userKey, spiritName, conversation string) {
	provider, err := s.oracleFactory.GetProvider(s.cfg.Model.Provider)
	if err != nil {
		s.logger.Warnw("imprint extraction: provider not found", "error", err)
		return
	}

	req := &ports.OracleRequest{
		Model:       s.cfg.Model.Model,
		Temperature: s.cfg.Model.Temperature,
		SystemPrompt: "Extract memorable facts about the user from the conversation. " +
			"Return one fact per line, category prefix in square brackets: [preference], [personal], [business], or [context]. " +
			"Example: [preference] Prefers morning delivery. " +
			"Only extract clear, durable facts. Return empty if none.",
		Messages: []ports.OracleMessage{
			{Role: ports.RoleUser, Content: conversation},
		},
	}

	resp, err := provider.GenerateResponse(ctx, req)
	if err != nil || resp.Content == "" {
		return
	}

	existing, _ := s.imprintRepo.GetByUserKey(ctx, userKey, spiritName)
	existingFacts := make(map[string]bool, len(existing))
	for _, imp := range existing {
		existingFacts[strings.ToLower(imp.Fact)] = true
	}

	lines := strings.Split(strings.TrimSpace(resp.Content), "\n")
	now := time.Now().UTC()
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		category, fact := parseFact(line)
		if fact == "" {
			continue
		}
		if existingFacts[strings.ToLower(fact)] {
			continue
		}
		_ = s.imprintRepo.Save(ctx, entities.Imprint{
			ID:         helpers.GenerateRandomID(),
			UserKey:    userKey,
			SpiritName: spiritName,
			Fact:       fact,
			Category:   category,
			Source:     entities.ImprintSourceExtracted,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	_ = s.imprintRepo.Prune(ctx, userKey, spiritName, s.cfg.maxImprints())
}

func parseFact(line string) (category entities.ImprintCategory, fact string) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") {
		return entities.ImprintCategoryContext, line
	}
	end := strings.Index(line, "]")
	if end < 0 {
		return entities.ImprintCategoryContext, line
	}
	category = entities.ImprintCategory(strings.ToLower(strings.TrimSpace(line[1:end])))
	fact = strings.TrimSpace(line[end+1:])
	return category, fact
}
