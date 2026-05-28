package archivists

import (
	"context"
	"fmt"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/tracer"
	"go.uber.org/zap"
)

const (
	defaultSummarizerMaxTokens   = 512
	defaultSummarizerTemperature = 0.2

	// promptLabelUser and promptLabelAssistant are the display labels used in the
	// summarization prompt. They are intentionally human-readable (title-case) and
	// distinct from the role constants in ports (lowercase) which are LLM API values.
	promptLabelUser      = "User"
	promptLabelAssistant = "Assistant"
	promptLabelTool      = "Tool"
)

// LLMArchivist compresses a Memory thread into a concise summary using an LLM.
// The summary replaces raw history so the Memory footprint stays bounded.
type LLMArchivist struct {
	provider   string
	model      string
	temp       float64
	maxTokens  int
	llmFactory ports.OracleFactory
	logger     *zap.SugaredLogger
}

func NewLLMArchivist(provider, model string, llmFactory ports.OracleFactory) *LLMArchivist {
	return &LLMArchivist{
		provider:   provider,
		model:      model,
		temp:       defaultSummarizerTemperature,
		maxTokens:  defaultSummarizerMaxTokens,
		llmFactory: llmFactory,
		logger:     dbg.GetLogger(),
	}
}

func (s *LLMArchivist) WithTemperature(temp float64) *LLMArchivist {
	s.temp = temp
	return s
}

func (s *LLMArchivist) WithMaxTokens(maxTokens int) *LLMArchivist {
	s.maxTokens = maxTokens
	return s
}

// memoryKey and subjectKey are used for observability only.
func (s *LLMArchivist) Summarize(ctx context.Context, memoryKey, subjectKey string, messages []entities.Thread) (string, error) {
	ctx, span := tracer.GetTracer().Start(ctx, "LLMArchivist/Summarize")
	defer span.End()

	if len(messages) == 0 {
		return "", nil
	}

	provider, err := s.getProvider()
	if err != nil {
		return "", fmt.Errorf("summarizer: failed to get LLM provider: %w", err)
	}

	prompt := s.buildPrompt(messages)

	s.logger.Infow("summarizing conversation segment",
		"memory_key", memoryKey,
		"subject_key", subjectKey,
		"messages_count", len(messages),
		"model", s.model)

	request := &ports.OracleRequest{
		Model: s.model,
		SystemPrompt: `You are a conversation summarizer. Your job is to produce a concise, factual summary of the conversation segment provided.

Guidelines:
- Capture key facts, decisions, confirmed data, and unresolved questions
- Write in third-person past tense (e.g. "The user confirmed...", "The assistant replied...")
- Do not include filler phrases or pleasantries — only meaningful content
- Be concise but complete: the summary replaces the original messages in memory
- Do not fabricate or infer facts not present in the conversation`,
		Messages: []ports.OracleMessage{
			{Role: ports.RoleUser, Content: prompt},
		},
		Temperature: s.temp,
		MaxTokens:   s.maxTokens,
		UseTools:    false,
	}

	response, err := provider.GenerateResponse(ctx, request)
	if err != nil {
		return "", fmt.Errorf("summarizer: LLM call failed: %w", err)
	}

	summary := strings.TrimSpace(response.Content)

	s.logger.Infow("conversation segment summarized",
		"memory_key", memoryKey,
		"subject_key", subjectKey,
		"summary_length", len(summary),
		"tokens_used", response.TokensUsed.TotalTokens)

	return summary, nil
}

func (s *LLMArchivist) getProvider() (ports.Oracle, error) {
	if s.provider != "" {
		provider, err := s.llmFactory.GetProvider(s.provider)
		if err == nil {
			return provider, nil
		}
		s.logger.Warnw("configured provider not found, falling back to model-based lookup",
			"provider", s.provider,
			"model", s.model,
			"error", err)
	}

	provider, err := s.llmFactory.GetProviderForModel(s.model)
	if err != nil {
		s.logger.Errorw("failed to get LLM provider for model",
			"provider", s.provider,
			"model", s.model,
			"error", err)
		return nil, fmt.Errorf("get oracle provider: %w", err)
	}
	return provider, nil
}

func (s *LLMArchivist) buildPrompt(messages []entities.Thread) string {
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation segment:\n\n")

	for _, msg := range messages {
		var label string
		switch msg.Role {
		case ports.RoleAssistant:
			label = promptLabelAssistant
		case ports.RoleTool:
			label = promptLabelTool
		default:
			label = promptLabelUser
		}
		fmt.Fprintf(&sb, "[%s]: %s\n\n", label, msg.Content)
	}

	sb.WriteString("---\nProvide a concise summary of the above conversation.")
	return sb.String()
}
