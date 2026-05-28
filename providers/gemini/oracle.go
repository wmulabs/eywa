package gemini

import (
	"context"
	"fmt"

	eywa "github.com/wmulabs/eywa"
	"google.golang.org/genai"
)

const (
	ProviderName = "gemini"

	defaultMaxRetries = 3
	defaultTimeout    = 60 // seconds

	defaultStopReason = "stop"
	roleUser          = eywa.RoleUser
	roleModel         = "model" // Gemini uses "model" instead of "assistant"
	roleAssistant     = eywa.RoleAssistant
)

type GeminiOracle struct {
	client *genai.Client
	apiKey string
	config Config
}

type Config struct {
	// Name overrides the provider name returned by GetName(). Defaults to "gemini".
	Name       string
	APIKey     string
	BaseURL    string
	MaxRetries int
	Timeout    int // seconds

	// Vertex AI fields — set these instead of APIKey to use BackendVertexAI.
	// Uses Application Default Credentials (ADC); no API key needed.
	Project  string
	Location string
}

func NewOracle(apiKey string) (*GeminiOracle, error) {
	return NewOracleWithConfig(Config{
		APIKey:     apiKey,
		MaxRetries: defaultMaxRetries,
		Timeout:    defaultTimeout,
	})
}

func NewOracleWithConfig(config Config) (*GeminiOracle, error) {
	client, err := createGeminiClient(context.Background(), config)
	if err != nil {
		return nil, err
	}

	return &GeminiOracle{
		client: client,
		apiKey: config.APIKey,
		config: config,
	}, nil
}

// NewVertexOracle creates a GeminiOracle backed by Vertex AI.
// Uses Application Default Credentials — no API key required.
// Supports all Gemini models deployed on Vertex AI.
// Spirit.ModelConfig.Provider must be set to "vertexai".
func NewVertexOracle(ctx context.Context, project, location string) (*GeminiOracle, error) {
	client, err := createGeminiClient(ctx, Config{
		Name:       "vertexai",
		Project:    project,
		Location:   location,
		MaxRetries: defaultMaxRetries,
		Timeout:    defaultTimeout,
	})
	if err != nil {
		return nil, err
	}

	return &GeminiOracle{
		client: client,
		config: Config{
			Name:       "vertexai",
			Project:    project,
			Location:   location,
			MaxRetries: defaultMaxRetries,
			Timeout:    defaultTimeout,
		},
	}, nil
}

func createGeminiClient(ctx context.Context, config Config) (*genai.Client, error) {
	var clientConfig *genai.ClientConfig

	if config.Project != "" {
		clientConfig = &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  config.Project,
			Location: config.Location,
		}
	} else {
		clientConfig = &genai.ClientConfig{
			APIKey:  config.APIKey,
			Backend: genai.BackendGeminiAPI,
		}
	}

	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	return client, nil
}

func (p *GeminiOracle) GetName() string {
	if p.config.Name != "" {
		return p.config.Name
	}
	return ProviderName
}

func (p *GeminiOracle) GetAvailableModels() []string {
	return []string{
		// Gemini 3.1 (released February 2026)
		"gemini-3.1-pro-preview",
		"gemini-3.1-pro-preview-customtools",
		"gemini-3.1-flash-lite-preview",
		"gemini-3.1-flash-tts-preview",
		"gemini-3.1-flash-image-preview",
		"gemini-3.1-flash-live-preview",

		// Gemini 3.0
		"gemini-3-flash-preview",

		// Gemini 2.5 (stable)
		"gemini-2.5-pro",
		"gemini-2.5-pro-latest",
		"gemini-2.5-pro-002",
		"gemini-2.5-flash",
		"gemini-2.5-flash-latest",
		"gemini-2.5-flash-002",
		"gemini-2.5-flash-lite",
		"gemini-2.5-flash-lite-latest",
	}
}

func (p *GeminiOracle) GenerateResponse(ctx context.Context, req *eywa.OracleRequest) (*eywa.OracleResponse, error) {
	config := p.buildGenerationConfig(req)

	if p.hasConversationHistory(req) {
		return p.generateWithChat(ctx, req, config)
	}

	return p.generateSimple(ctx, req, config)
}

func (p *GeminiOracle) buildGenerationConfig(req *eywa.OracleRequest) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{}

	p.addSamplingParameters(config, req)
	p.addSystemInstruction(config, req.SystemPrompt)
	p.addTools(config, req)

	return config
}

func (p *GeminiOracle) addSamplingParameters(config *genai.GenerateContentConfig, req *eywa.OracleRequest) {
	if req.Temperature > 0 {
		config.Temperature = genai.Ptr(float32(req.Temperature))
	}

	if req.MaxTokens > 0 {
		config.MaxOutputTokens = int32(req.MaxTokens)
	}

	if req.TopP > 0 {
		config.TopP = genai.Ptr(float32(req.TopP))
	}

	if req.TopK > 0 {
		topKFloat := float32(req.TopK)
		config.TopK = &topKFloat
	}

	if len(req.StopSequences) > 0 {
		config.StopSequences = req.StopSequences
	}
}

func (p *GeminiOracle) addSystemInstruction(config *genai.GenerateContentConfig, systemPrompt string) {
	if systemPrompt == "" {
		return
	}

	config.SystemInstruction = &genai.Content{
		Parts: []*genai.Part{{Text: systemPrompt}},
	}
}

func (p *GeminiOracle) addTools(config *genai.GenerateContentConfig, req *eywa.OracleRequest) {
	if !req.UseTools || len(req.Tools) == 0 {
		return
	}

	config.Tools = p.convertTools(req.Tools)
}

func (p *GeminiOracle) hasConversationHistory(req *eywa.OracleRequest) bool {
	return len(req.Messages) > 0
}

func (p *GeminiOracle) IsAvailable() bool {
	return p.client != nil
}

func (p *GeminiOracle) GetConfig() map[string]any {
	return map[string]any{
		"provider":    ProviderName,
		"base_url":    p.config.BaseURL,
		"has_api_key": p.apiKey != "",
		"max_retries": p.config.MaxRetries,
		"timeout":     p.config.Timeout,
	}
}

// Gemini 1.5+ models support inline image input.
func (p *GeminiOracle) SupportsImages(model string) bool {
	multimodalPrefixes := []string{"gemini-1.5", "gemini-2", "gemini-3"}
	for _, prefix := range multimodalPrefixes {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// Gemini 1.5+ models support inline audio input.
func (p *GeminiOracle) SupportsAudio(model string) bool {
	return p.SupportsImages(model)
}

// Gemini 1.5+ models support inline PDF input.
func (p *GeminiOracle) SupportsDocuments(model string) bool {
	return p.SupportsImages(model)
}

func (p *GeminiOracle) generateWithChat(ctx context.Context, req *eywa.OracleRequest, config *genai.GenerateContentConfig) (*eywa.OracleResponse, error) {
	history, sendParts := p.splitMessagesForChat(req.Messages, req.Attachments)

	chat, err := p.client.Chats.Create(ctx, req.Model, config, history)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat session: %w", err)
	}

	resp, err := chat.SendMessage(ctx, sendParts...)
	if err != nil {
		return nil, fmt.Errorf("gemini api error: %w", err)
	}

	return p.parseResponse(resp)
}

// splitMessagesForChat splits messages into chat history and the parts to send via SendMessage,
// merging any trailing RoleTool batch into a single user turn as required by the Gemini API.
func (p *GeminiOracle) splitMessagesForChat(messages []eywa.OracleMessage, attachments []eywa.LLMAttachment) ([]*genai.Content, []genai.Part) {
	n := len(messages)

	batchStart := n
	for batchStart > 0 && messages[batchStart-1].Role == eywa.RoleTool {
		batchStart--
	}
	if batchStart == n && n > 0 && messages[n-1].Role == eywa.RoleUser {
		batchStart = n - 1
	}

	history := p.convertToHistory(messages[:batchStart])

	var sendParts []genai.Part
	for i := batchStart; i < n; i++ {
		for _, pt := range p.buildMessageParts(messages[i]) {
			sendParts = append(sendParts, *pt)
		}
	}

	if len(attachments) > 0 {
		sendParts = append(sendParts, p.convertAttachmentsToParts(attachments)...)
	}

	if len(sendParts) == 0 {
		sendParts = []genai.Part{{Text: " "}}
	}

	return history, sendParts
}

func (p *GeminiOracle) generateSimple(ctx context.Context, req *eywa.OracleRequest, config *genai.GenerateContentConfig) (*eywa.OracleResponse, error) {
	content := p.buildSimpleContent(req)

	resp, err := p.client.Models.GenerateContent(ctx, req.Model, []*genai.Content{content}, config)
	if err != nil {
		return nil, fmt.Errorf("gemini api error: %w", err)
	}

	return p.parseResponse(resp)
}

func (p *GeminiOracle) buildSimpleContent(req *eywa.OracleRequest) *genai.Content {
	parts := make([]*genai.Part, 0)

	if req.SystemPrompt != "" {
		parts = append(parts, &genai.Part{Text: req.SystemPrompt})
	}

	if len(req.Attachments) > 0 {
		attachmentParts := p.convertAttachmentsToParts(req.Attachments)
		for i := range attachmentParts {
			parts = append(parts, &attachmentParts[i])
		}
	}

	return &genai.Content{
		Parts: parts,
		Role:  roleUser,
	}
}

func (p *GeminiOracle) parseResponse(resp *genai.GenerateContentResponse) (*eywa.OracleResponse, error) {
	if err := p.validateResponse(resp); err != nil {
		return nil, err
	}

	candidate := resp.Candidates[0]

	content, toolCalls := p.extractContentAndToolCalls(candidate)
	stopReason := p.normalizeStopReason(candidate.FinishReason)

	if len(toolCalls) > 0 {
		stopReason = eywa.StopReasonToolCalls
	}

	tokensUsed := p.extractTokenUsage(resp.UsageMetadata)

	return &eywa.OracleResponse{
		Content:    content,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		TokensUsed: tokensUsed,
	}, nil
}

func (p *GeminiOracle) validateResponse(resp *genai.GenerateContentResponse) error {
	if len(resp.Candidates) == 0 {
		return fmt.Errorf("no response candidates from gemini")
	}
	return nil
}

func (p *GeminiOracle) extractContentAndToolCalls(candidate *genai.Candidate) (string, []eywa.OracleToolCall) {
	var content string
	var toolCalls []eywa.OracleToolCall

	if candidate.Content == nil {
		return content, toolCalls
	}

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			content += part.Text
		}

		if part.FunctionCall != nil {
			toolCalls = append(toolCalls, p.convertPartToToolCall(part))
		}
	}

	return content, toolCalls
}

// convertPartToToolCall converts a Gemini Part containing a FunctionCall to standard format,
// preserving ThoughtSignature for reasoning models (Gemini 2.5+/3.x).
func (p *GeminiOracle) convertPartToToolCall(part *genai.Part) eywa.OracleToolCall {
	fc := part.FunctionCall
	return eywa.OracleToolCall{
		ID:               fc.Name, // Gemini uses name as ID
		Name:             fc.Name,
		Arguments:        fc.Args,
		ThoughtSignature: part.ThoughtSignature,
	}
}

// normalizeStopReason maps Gemini finish reasons to standard StopReason constants.
// https://ai.google.dev/api/generate-content#finishreason
func (p *GeminiOracle) normalizeStopReason(finishReason genai.FinishReason) string {
	switch finishReason {
	case genai.FinishReasonStop:
		return eywa.StopReasonComplete
	case genai.FinishReasonMaxTokens:
		return eywa.StopReasonLength
	case genai.FinishReasonSafety, genai.FinishReasonRecitation:
		return eywa.StopReasonContentFilter
	default:
		return eywa.StopReasonComplete
	}
}

func (p *GeminiOracle) extractTokenUsage(metadata *genai.GenerateContentResponseUsageMetadata) eywa.OracleUsage {
	if metadata == nil {
		return eywa.OracleUsage{}
	}

	return eywa.OracleUsage{
		PromptTokens:     int(metadata.PromptTokenCount),
		CompletionTokens: int(metadata.CandidatesTokenCount),
		TotalTokens:      int(metadata.TotalTokenCount),
	}
}

// convertToHistory converts messages to Gemini chat history,
// merging consecutive RoleTool messages into a single user Content as required by Gemini.
func (p *GeminiOracle) convertToHistory(messages []eywa.OracleMessage) []*genai.Content {
	history := make([]*genai.Content, 0, len(messages))

	for i := 0; i < len(messages); {
		msg := messages[i]

		if msg.Role == eywa.RoleTool {
			var parts []*genai.Part
			for i < len(messages) && messages[i].Role == eywa.RoleTool {
				parts = append(parts, p.buildMessageParts(messages[i])...)
				i++
			}
			history = append(history, &genai.Content{Parts: parts, Role: roleUser})
			continue
		}

		if content := p.convertMessageToContent(msg); content != nil {
			history = append(history, content)
		}
		i++
	}

	return history
}

func (p *GeminiOracle) convertMessageToContent(msg eywa.OracleMessage) *genai.Content {
	parts := p.buildMessageParts(msg)

	if len(parts) == 0 {
		return nil
	}

	return &genai.Content{
		Parts: parts,
		Role:  p.convertRole(msg.Role),
	}
}

// buildMessageParts constructs Gemini parts for a message: text, function calls, or function responses.
func (p *GeminiOracle) buildMessageParts(msg eywa.OracleMessage) []*genai.Part {
	if msg.Role == eywa.RoleTool {
		return []*genai.Part{p.createFunctionResponsePart(msg)}
	}

	var parts []*genai.Part

	if msg.Content != "" {
		parts = append(parts, &genai.Part{Text: msg.Content})
	}

	if len(msg.ToolCalls) > 0 {
		parts = append(parts, p.convertToolCallsToParts(msg.ToolCalls)...)
	}

	return parts
}

func (p *GeminiOracle) convertRole(role string) string {
	if role == roleAssistant {
		return roleModel
	}
	return roleUser
}

// convertToolCallsToParts converts tool calls to Gemini function call parts.
// ThoughtSignature is restored when present — required for reasoning models (Gemini 2.5+/3.x).
func (p *GeminiOracle) convertToolCallsToParts(toolCalls []eywa.OracleToolCall) []*genai.Part {
	parts := make([]*genai.Part, 0, len(toolCalls))

	for _, tc := range toolCalls {
		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: tc.Name,
				Args: tc.Arguments,
			},
			ThoughtSignature: tc.ThoughtSignature,
		})
	}

	return parts
}

func (p *GeminiOracle) createFunctionResponsePart(msg eywa.OracleMessage) *genai.Part {
	return &genai.Part{
		FunctionResponse: &genai.FunctionResponse{
			Name: msg.ToolName,
			Response: map[string]any{
				"result": msg.Content,
			},
		},
	}
}

// convertAttachmentsToParts converts LLM attachments to Gemini Parts for multimodal input.
// Supports images, audio, video, and documents via inline binary data.
// URL-only attachments are skipped — the Gemini SDK does not support URL references directly;
// enrichers should populate the Data field before reaching here.
func (p *GeminiOracle) convertAttachmentsToParts(attachments []eywa.LLMAttachment) []genai.Part {
	parts := make([]genai.Part, 0, len(attachments))

	for _, att := range attachments {
		part := p.createAttachmentPart(att)
		if part != nil {
			parts = append(parts, *part)
		}
	}

	return parts
}

func (p *GeminiOracle) createAttachmentPart(att eywa.LLMAttachment) *genai.Part {
	if len(att.Data) > 0 {
		return p.createInlineDataPart(att)
	}
	return nil
}

func (p *GeminiOracle) createInlineDataPart(att eywa.LLMAttachment) *genai.Part {
	return &genai.Part{
		InlineData: &genai.Blob{
			MIMEType: att.MimeType,
			Data:     att.Data,
		},
	}
}

func (p *GeminiOracle) convertTools(tools []eywa.OracleTool) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	functionDeclarations := make([]*genai.FunctionDeclaration, 0, len(tools))

	for _, tool := range tools {
		functionDeclarations = append(functionDeclarations, p.createFunctionDeclaration(tool))
	}

	return []*genai.Tool{{
		FunctionDeclarations: functionDeclarations,
	}}
}

func (p *GeminiOracle) createFunctionDeclaration(tool eywa.OracleTool) *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:                 tool.Name,
		Description:          tool.Description,
		ParametersJsonSchema: tool.Parameters,
	}
}
