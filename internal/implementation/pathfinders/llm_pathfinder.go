package pathfinders

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/tracer"
	"go.uber.org/zap"
)

const (
	DefaultPathfinderName = "default_llm_pathfinder"
)

// internalSpirit is routing-only — not persisted in DB.
type LLMPathfinder struct {
	name           string
	internalSpirit *entities.Spirit // routing-only Spirit, not stored in DB
	llmFactory     ports.OracleFactory
	spiritRepo     ports.SpiritRepository
	logger         *zap.SugaredLogger
}

// temperature defaults to 0.2 — low value keeps routing deterministic.
func NewDefaultLLMPathfinder(provider, model string, temperature float64, llmFactory ports.OracleFactory, spiritRepo ports.SpiritRepository) *LLMPathfinder {
	if temperature == 0 {
		temperature = 0.2
	}

	internalSpirit := &entities.Spirit{
		Name:        "__internal_llm_pathfinder",
		Description: "Built-in LLM Pathfinder for Spirit routing",
		SystemPrompt: `You are an intelligent Spirit routing system. Your job is to analyze the user's message and context to select the most appropriate Spirit.

You will receive:
1. User's message
2. Knowledge (if available)
3. Available Spirits with their descriptions and specializations

Analyze the information carefully and respond with ONLY the name of the selected Spirit. Do not include any explanation or additional text.

Guidelines:
- Match user intent to Spirit specialization
- Consider knowledge for better decisions
- Be consistent in your routing choices

Examples:
- User asks about pricing/features → select sales Spirit
- User reports bug/technical issue → select support Spirit
- User asks about invoice/payment → select billing Spirit`,
		ModelConfig: entities.SpiritModel{
			Provider:    provider,
			Model:       model,
			Temperature: temperature,
			MaxTokens:   50,
		},
		AllowedActions: []entities.AllowedAction{},
		IsActive:       true,
	}

	return &LLMPathfinder{
		name:           DefaultPathfinderName,
		internalSpirit: internalSpirit,
		llmFactory:     llmFactory,
		spiritRepo:     spiritRepo,
		logger:         dbg.GetLogger(),
	}
}

func NewLLMPathfinder(name string, spirit *entities.Spirit, llmFactory ports.OracleFactory, spiritRepo ports.SpiritRepository) *LLMPathfinder {
	return &LLMPathfinder{
		name:           name,
		internalSpirit: spirit,
		llmFactory:     llmFactory,
		spiritRepo:     spiritRepo,
		logger:         dbg.GetLogger(),
	}
}

func (c *LLMPathfinder) GetName() string {
	return c.name
}

func (c *LLMPathfinder) SelectSpirit(ctx context.Context, event *entities.Pulse, availableSpirits []string) string {
	ctx, span := tracer.GetTracer().Start(ctx, "LLMPathfinder/SelectSpirit")
	defer span.End()

	if len(availableSpirits) == 0 {
		c.logger.Warnw("no available Spirits provided", "memory_key", event.MemoryKey)
		return ""
	}

	if len(availableSpirits) == 1 {
		c.logger.Infow("only one Spirit available, skipping LLM", "spirit", availableSpirits[0], "memory_key", event.MemoryKey)
		return availableSpirits[0]
	}

	spiritDescriptions, err := c.fetchSpiritDescriptions(ctx, availableSpirits)
	if err != nil {
		c.logger.Errorw("failed to fetch Spirit descriptions", "error", err)
		return ""
	}

	provider, err := c.getOracle()
	if err != nil {
		return ""
	}

	prompt := c.buildRoutingPrompt(event, spiritDescriptions)
	response, err := c.callLLM(ctx, provider, prompt, event.MemoryKey, len(availableSpirits))
	if err != nil {
		return ""
	}

	selectedSpirit := c.extractSpiritName(response.Content)
	if !c.isValidSpirit(selectedSpirit, availableSpirits) {
		c.logger.Warnw("LLM returned invalid Spirit name", "selected", selectedSpirit, "available", availableSpirits, "memory_key", event.MemoryKey)
		return ""
	}

	c.logger.Infow("Spirit selected by LLM", "pathfinder", c.name, "selected_spirit", selectedSpirit, "tokens_used", response.TokensUsed.TotalTokens, "memory_key", event.MemoryKey)
	return selectedSpirit
}

func (c *LLMPathfinder) getOracle() (ports.Oracle, error) {
	provider, err := c.llmFactory.GetProvider(c.internalSpirit.ModelConfig.Provider)
	if err != nil {
		provider, err = c.llmFactory.GetProviderForModel(c.internalSpirit.ModelConfig.Model)
		if err != nil {
			c.logger.Errorw("failed to get LLM provider", "provider", c.internalSpirit.ModelConfig.Provider, "model", c.internalSpirit.ModelConfig.Model, "error", err)
			return nil, err
		}
	}
	return provider, nil
}

func (c *LLMPathfinder) callLLM(ctx context.Context, provider ports.Oracle, prompt, memoryKey string, spiritCount int) (*ports.OracleResponse, error) {
	request := &ports.OracleRequest{
		Model:        c.internalSpirit.ModelConfig.Model,
		SystemPrompt: c.internalSpirit.SystemPrompt,
		Messages: []ports.OracleMessage{
			{Role: ports.RoleUser, Content: prompt},
		},
		Temperature: c.internalSpirit.ModelConfig.Temperature,
		MaxTokens:   c.internalSpirit.ModelConfig.MaxTokens,
		UseTools:    false,
	}

	c.logger.Infow("calling LLM for Spirit routing", "pathfinder", c.name, "model", c.internalSpirit.ModelConfig.Model, "available_spirits", spiritCount, "memory_key", memoryKey)

	response, err := provider.GenerateResponse(ctx, request)
	if err != nil {
		c.logger.Errorw("LLM routing failed", "error", err)
		return nil, err
	}

	return response, nil
}

func (c *LLMPathfinder) extractSpiritName(content string) string {
	trimmed := strings.TrimSpace(content)
	return strings.Trim(trimmed, `"'`)
}

func (c *LLMPathfinder) fetchSpiritDescriptions(ctx context.Context, spiritNames []string) (map[string]*entities.Spirit, error) {
	spirits, err := c.spiritRepo.FindActiveByNames(ctx, spiritNames)
	if err != nil {
		c.logger.Errorw("failed to fetch Spirits in bulk", "spirit_names", spiritNames, "error", err)
		return nil, err
	}

	c.addMissingSpirits(spirits, spiritNames)
	return spirits, nil
}

func (c *LLMPathfinder) addMissingSpirits(spiritMap map[string]*entities.Spirit, spiritNames []string) {
	for _, name := range spiritNames {
		if _, exists := spiritMap[name]; !exists {
			c.logger.Warnw("Spirit not found, using minimal placeholder", "spirit_name", name)
			spiritMap[name] = &entities.Spirit{
				Name:        name,
				Description: "No description available",
			}
		}
	}
}

func (c *LLMPathfinder) buildRoutingPrompt(event *entities.Pulse, spirits map[string]*entities.Spirit) string {
	var sb strings.Builder

	c.writeUserMessage(&sb, event.UserMessage)
	c.writeKnowledge(&sb, event.Knowledge)
	c.writeEventMetadata(&sb, event.Metadata)
	c.writeAvailableSpirits(&sb, spirits)

	sb.WriteString("---\n\n")
	sb.WriteString("Based on the above information, which Spirit should handle this request? Reply with ONLY the Spirit name (e.g., 'sales' or 'support').")

	return sb.String()
}

func (c *LLMPathfinder) writeUserMessage(sb *strings.Builder, message string) {
	sb.WriteString("## User Message\n")
	if message != "" {
		sb.WriteString(message)
	} else {
		sb.WriteString("(No message provided)")
	}
	sb.WriteString("\n\n")
}

func (c *LLMPathfinder) writeKnowledge(sb *strings.Builder, knowledge map[string]any) {
	if len(knowledge) == 0 {
		return
	}

	sb.WriteString("## Knowledge\n")
	if knowledgeJSON, err := json.MarshalIndent(knowledge, "", "  "); err == nil {
		sb.Write(knowledgeJSON)
	}
	sb.WriteString("\n\n")
}

func (c *LLMPathfinder) writeEventMetadata(sb *strings.Builder, metadata map[string]any) {
	if len(metadata) == 0 {
		return
	}

	sb.WriteString("## Event Metadata\n")
	if metadataJSON, err := json.MarshalIndent(metadata, "", "  "); err == nil {
		sb.Write(metadataJSON)
	}
	sb.WriteString("\n\n")
}

func (c *LLMPathfinder) writeAvailableSpirits(sb *strings.Builder, spirits map[string]*entities.Spirit) {
	sb.WriteString("## Available Spirits\n\n")
	for name, spirit := range spirits {
		sb.WriteString(fmt.Sprintf("### %s\n", name))
		if spirit.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", spirit.Description))
		}
		if spirit.Specialization != "" {
			sb.WriteString(fmt.Sprintf("Specialization: %s\n", spirit.Specialization))
		}
		sb.WriteString("\n")
	}
}

func (c *LLMPathfinder) isValidSpirit(selected string, available []string) bool {
	for _, spirit := range available {
		if strings.EqualFold(selected, spirit) {
			return true
		}
	}
	return false
}
