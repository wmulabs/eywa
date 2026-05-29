package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	eywa "github.com/wmulabs/eywa"
)

const (
	ProviderName           = "openai"
	ProviderNameOllama     = "ollama"
	ProviderNameGroq       = "groq"
	ProviderNameMistral    = "mistral"
	ProviderNameTogether   = "together"
	ProviderNameOpenRouter = "openrouter"
	ProviderNameXAI        = "xai"

	defaultMaxRetries = 3
	defaultTimeout    = 60 // seconds

	// defaultImageMimeType is used when no MIME type is present on an attachment.
	defaultImageMimeType = "image/jpeg"

	roleUser      = eywa.RoleUser
	roleAssistant = eywa.RoleAssistant
	roleSystem    = eywa.RoleSystem
	roleTool      = eywa.RoleTool
)

type OpenAIOracle struct {
	client *openai.Client
	apiKey string
	config Config
}

type Config struct {
	// Name overrides the provider name returned by GetName(). Defaults to "openai".
	// Set this when using an OpenAI-compatible endpoint so multiple providers
	// can coexist in the same Weave (e.g. "groq", "ollama").
	Name       string
	APIKey     string
	OrgID      string
	BaseURL    string
	MaxRetries int
	Timeout    int // seconds
}

func NewOracle(apiKey string) *OpenAIOracle {
	return NewOracleWithConfig(Config{
		APIKey:     apiKey,
		MaxRetries: defaultMaxRetries,
		Timeout:    defaultTimeout,
	})
}

func NewOracleWithConfig(config Config) *OpenAIOracle {
	if config.Name == "" {
		config.Name = ProviderName
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = defaultMaxRetries
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	client := createOpenAIClient(config)

	return &OpenAIOracle{
		client: &client,
		apiKey: config.APIKey,
		config: config,
	}
}

// NewOllamaOracle creates an Oracle for a local Ollama instance.
// baseURL is the Ollama server address (e.g. "http://localhost:11434").
// Spirit.ModelConfig.Provider must be set to "ollama".
func NewOllamaOracle(baseURL string) *OpenAIOracle {
	return NewOracleWithConfig(Config{
		Name:    ProviderNameOllama,
		APIKey:  "ollama", // Ollama ignores the key; SDK requires a non-empty value
		BaseURL: baseURL + "/v1",
	})
}

// NewGroqOracle creates an Oracle backed by Groq's inference API (OpenAI-compatible).
// Spirit.ModelConfig.Provider must be set to "groq".
func NewGroqOracle(apiKey string) *OpenAIOracle {
	return NewOracleWithConfig(Config{
		Name:    ProviderNameGroq,
		APIKey:  apiKey,
		BaseURL: "https://api.groq.com/openai/v1",
	})
}

// NewMistralOracle creates an Oracle backed by Mistral AI's API (OpenAI-compatible).
// Spirit.ModelConfig.Provider must be set to "mistral".
func NewMistralOracle(apiKey string) *OpenAIOracle {
	return NewOracleWithConfig(Config{
		Name:    ProviderNameMistral,
		APIKey:  apiKey,
		BaseURL: "https://api.mistral.ai/v1",
	})
}

// NewTogetherOracle creates an Oracle backed by Together AI (OpenAI-compatible).
// Spirit.ModelConfig.Provider must be set to "together".
func NewTogetherOracle(apiKey string) *OpenAIOracle {
	return NewOracleWithConfig(Config{
		Name:    ProviderNameTogether,
		APIKey:  apiKey,
		BaseURL: "https://api.together.xyz/v1",
	})
}

// NewOpenRouterOracle creates an Oracle backed by OpenRouter (openrouter.ai).
// OpenRouter proxies 100+ models — set Spirit.ModelConfig.Model to any supported model ID
// (e.g. "anthropic/claude-3-5-sonnet", "meta-llama/llama-3.3-70b-instruct").
// Spirit.ModelConfig.Provider must be set to "openrouter".
func NewOpenRouterOracle(apiKey string) *OpenAIOracle {
	return NewOracleWithConfig(Config{
		Name:    ProviderNameOpenRouter,
		APIKey:  apiKey,
		BaseURL: "https://openrouter.ai/api/v1",
	})
}

// NewXAIOracle creates an Oracle backed by xAI's Grok models (OpenAI-compatible).
// Spirit.ModelConfig.Provider must be set to "xai".
func NewXAIOracle(apiKey string) *OpenAIOracle {
	return NewOracleWithConfig(Config{
		Name:    ProviderNameXAI,
		APIKey:  apiKey,
		BaseURL: "https://api.x.ai/v1",
	})
}

func createOpenAIClient(config Config) openai.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(config.APIKey),
	}

	if config.OrgID != "" {
		opts = append(opts, option.WithOrganization(config.OrgID))
	}

	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}

	if config.MaxRetries > 0 {
		opts = append(opts, option.WithMaxRetries(config.MaxRetries))
	}

	return openai.NewClient(opts...)
}

func (p *OpenAIOracle) GetName() string {
	return p.config.Name
}

func (p *OpenAIOracle) GetAvailableModels() []string {
	return []string{
		// GPT-5 (current flagship)
		"gpt-5.4",
		"gpt-5.4-mini",
		"gpt-5.4-nano",

		// GPT-4.1
		"gpt-4.1",
		"gpt-4.1-mini",

		// GPT-4o (stable)
		"gpt-4o",
		"gpt-4o-2024-11-20",
		"gpt-4o-2024-08-06",
		"gpt-4o-mini",
		"gpt-4o-mini-2024-07-18",

		// Reasoning models
		"o4-mini",
		"o3",
		"o3-pro",
		"o3-mini",
		"o3-mini-2025-01-31",
		"o1",
		"o1-2024-12-17",
	}
}

func (p *OpenAIOracle) GenerateResponse(ctx context.Context, req *eywa.OracleRequest) (*eywa.OracleResponse, error) {
	params := p.buildChatCompletionParams(req)

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai api error: %w", err)
	}

	return p.parseResponse(resp)
}

func (p *OpenAIOracle) buildChatCompletionParams(req *eywa.OracleRequest) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model:    req.Model,
		Messages: p.convertMessages(req),
	}

	p.addSamplingParameters(&params, req)
	p.addTools(&params, req)

	return params
}

func (p *OpenAIOracle) addSamplingParameters(params *openai.ChatCompletionNewParams, req *eywa.OracleRequest) {
	if req.Temperature > 0 {
		params.Temperature = openai.Float(float64(req.Temperature))
	}

	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}

	if req.TopP > 0 {
		params.TopP = openai.Float(float64(req.TopP))
	}

	if len(req.StopSequences) > 0 {
		params.Stop = p.buildStopSequencesParam(req.StopSequences)
	}
}

func (p *OpenAIOracle) buildStopSequencesParam(stopSequences []string) openai.ChatCompletionNewParamsStopUnion {
	if len(stopSequences) == 1 {
		return openai.ChatCompletionNewParamsStopUnion{
			OfString: openai.String(stopSequences[0]),
		}
	}

	return openai.ChatCompletionNewParamsStopUnion{
		OfStringArray: stopSequences,
	}
}

func (p *OpenAIOracle) addTools(params *openai.ChatCompletionNewParams, req *eywa.OracleRequest) {
	if !req.UseTools || len(req.Tools) == 0 {
		return
	}

	params.Tools = p.convertTools(req.Tools)
}

func (p *OpenAIOracle) parseResponse(resp *openai.ChatCompletion) (*eywa.OracleResponse, error) {
	if err := p.validateResponse(resp); err != nil {
		return nil, err
	}

	choice := resp.Choices[0]

	toolCalls, err := p.extractToolCalls(choice.Message.ToolCalls)
	if err != nil {
		return nil, err
	}

	return &eywa.OracleResponse{
		Content:    choice.Message.Content,
		ToolCalls:  toolCalls,
		StopReason: p.normalizeStopReason(choice.FinishReason),
		TokensUsed: p.extractTokenUsage(resp.Usage),
	}, nil
}

func (p *OpenAIOracle) validateResponse(resp *openai.ChatCompletion) error {
	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response choices from openai")
	}
	return nil
}

func (p *OpenAIOracle) extractToolCalls(toolCalls []openai.ChatCompletionMessageToolCall) ([]eywa.OracleToolCall, error) {
	if len(toolCalls) == 0 {
		return nil, nil
	}

	result := make([]eywa.OracleToolCall, 0, len(toolCalls))

	for _, tc := range toolCalls {
		toolCall, err := p.parseToolCall(tc)
		if err != nil {
			return nil, err
		}
		result = append(result, toolCall)
	}

	return result, nil
}

func (p *OpenAIOracle) parseToolCall(tc openai.ChatCompletionMessageToolCall) (eywa.OracleToolCall, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return eywa.OracleToolCall{}, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	return eywa.OracleToolCall{
		ID:        tc.ID,
		Name:      tc.Function.Name,
		Arguments: args,
	}, nil
}

func (p *OpenAIOracle) extractTokenUsage(usage openai.CompletionUsage) eywa.OracleUsage {
	return eywa.OracleUsage{
		PromptTokens:     int(usage.PromptTokens),
		CompletionTokens: int(usage.CompletionTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
}

// normalizeStopReason maps OpenAI finish reasons to standard StopReason constants.
// https://platform.openai.com/docs/guides/text-generation/chat-completions-api
func (p *OpenAIOracle) normalizeStopReason(finishReason string) string {
	switch finishReason {
	case "stop":
		return eywa.StopReasonComplete
	case "length":
		return eywa.StopReasonLength
	case "tool_calls":
		return eywa.StopReasonToolCalls
	case "content_filter":
		return eywa.StopReasonContentFilter
	default:
		return finishReason
	}
}

func (p *OpenAIOracle) IsAvailable() bool {
	// Compat providers (Ollama) use a dummy key; availability is determined by client presence.
	return p.client != nil
}

func (p *OpenAIOracle) GetConfig() map[string]any {
	return map[string]any{
		"provider":    p.config.Name,
		"base_url":    p.config.BaseURL,
		"has_api_key": p.apiKey != "",
		"max_retries": p.config.MaxRetries,
		"timeout":     p.config.Timeout,
	}
}

// Legacy text-only models (gpt-3.5, text-davinci) are not in the supported model list.
func (p *OpenAIOracle) SupportsImages(model string) bool {
	noVision := []string{"gpt-3.5", "text-davinci", "text-ada", "text-babbage", "text-curie"}
	for _, prefix := range noVision {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return false
		}
	}
	return model != ""
}

// OpenAI audio transcription uses the Whisper API separately, not the chat completion endpoint.
func (p *OpenAIOracle) SupportsAudio(_ string) bool {
	return false
}

// OpenAI does not support document attachments in the chat completion API.
func (p *OpenAIOracle) SupportsDocuments(_ string) bool {
	return false
}

func (p *OpenAIOracle) convertMessages(req *eywa.OracleRequest) []openai.ChatCompletionMessageParamUnion {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)

	if req.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(req.SystemPrompt))
	}

	for i, msg := range req.Messages {
		convertedMsg := p.convertMessage(msg, i, req)
		if convertedMsg != nil {
			messages = append(messages, *convertedMsg)
		}
	}

	return messages
}

func (p *OpenAIOracle) convertMessage(msg eywa.OracleMessage, index int, req *eywa.OracleRequest) *openai.ChatCompletionMessageParamUnion {
	if msg.Role == eywa.RoleTool && msg.Content != "" {
		result := openai.ToolMessage(msg.Content, msg.ToolName)
		return &result
	}

	if len(msg.ToolCalls) > 0 {
		result := p.createAssistantMessageWithToolCalls(msg)
		return &result
	}

	return p.convertMessageByRole(msg, index, req)
}

func (p *OpenAIOracle) createAssistantMessageWithToolCalls(msg eywa.OracleMessage) openai.ChatCompletionMessageParamUnion {
	toolCalls := p.convertToolCallsToParams(msg.ToolCalls)

	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: &openai.ChatCompletionAssistantMessageParam{
			Content: openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: openai.String(msg.Content),
			},
			ToolCalls: toolCalls,
		},
	}
}

func (p *OpenAIOracle) convertToolCallsToParams(toolCalls []eywa.OracleToolCall) []openai.ChatCompletionMessageToolCallParam {
	params := make([]openai.ChatCompletionMessageToolCallParam, len(toolCalls))

	for i, tc := range toolCalls {
		argsJSON, _ := json.Marshal(tc.Arguments)
		params[i] = openai.ChatCompletionMessageToolCallParam{
			ID: tc.ID,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      tc.Name,
				Arguments: string(argsJSON),
			},
		}
	}

	return params
}

func (p *OpenAIOracle) convertMessageByRole(msg eywa.OracleMessage, index int, req *eywa.OracleRequest) *openai.ChatCompletionMessageParamUnion {
	switch msg.Role {
	case roleUser:
		return p.createUserMessage(msg, index, req)
	case roleAssistant:
		result := openai.AssistantMessage(msg.Content)
		return &result
	case roleSystem:
		result := openai.SystemMessage(msg.Content)
		return &result
	default:
		return nil
	}
}

func (p *OpenAIOracle) createUserMessage(msg eywa.OracleMessage, index int, req *eywa.OracleRequest) *openai.ChatCompletionMessageParamUnion {
	isLastUserMsg := index == len(req.Messages)-1
	hasAttachments := len(req.Attachments) > 0

	if isLastUserMsg && hasAttachments {
		result := p.createMultimodalUserMessage(msg.Content, req.Attachments)
		return &result
	}

	result := openai.UserMessage(msg.Content)
	return &result
}

func (p *OpenAIOracle) createMultimodalUserMessage(text string, attachments []eywa.LLMAttachment) openai.ChatCompletionMessageParamUnion {
	contentParts := p.buildMultimodalContentParts(text, attachments)

	return openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Role: roleUser,
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfArrayOfContentParts: contentParts,
			},
		},
	}
}

func (p *OpenAIOracle) buildMultimodalContentParts(text string, attachments []eywa.LLMAttachment) []openai.ChatCompletionContentPartUnionParam {
	contentParts := make([]openai.ChatCompletionContentPartUnionParam, 0)

	if text != "" {
		contentParts = append(contentParts, p.createTextContentPart(text))
	}

	for _, att := range attachments {
		if p.isImageAttachment(att) {
			imagePart := p.createImageContentPart(att)
			if imagePart != nil {
				contentParts = append(contentParts, *imagePart)
			}
		}
	}

	return contentParts
}

func (p *OpenAIOracle) createTextContentPart(text string) openai.ChatCompletionContentPartUnionParam {
	textPart := openai.ChatCompletionContentPartTextParam{
		Type: "text",
		Text: text,
	}

	return openai.ChatCompletionContentPartUnionParam{
		OfText: &textPart,
	}
}

func (p *OpenAIOracle) isImageAttachment(att eywa.LLMAttachment) bool {
	return att.Type == eywa.ArtifactTypeImage
}

func (p *OpenAIOracle) createImageContentPart(att eywa.LLMAttachment) *openai.ChatCompletionContentPartUnionParam {
	// Prefer binary data over URL for privacy and to avoid expiry issues.
	if len(att.Data) > 0 {
		return p.createBase64ImagePart(att)
	}

	if att.URL != "" {
		return p.createURLImagePart(att)
	}

	return nil
}

func (p *OpenAIOracle) createBase64ImagePart(att eywa.LLMAttachment) *openai.ChatCompletionContentPartUnionParam {
	base64Data := base64.StdEncoding.EncodeToString(att.Data)
	mimeType := att.MimeType
	if mimeType == "" {
		mimeType = defaultImageMimeType
	}

	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	imagePart := openai.ChatCompletionContentPartImageParam{
		Type: "image_url",
		ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
			URL: dataURL,
		},
	}

	return &openai.ChatCompletionContentPartUnionParam{
		OfImageURL: &imagePart,
	}
}

func (p *OpenAIOracle) createURLImagePart(att eywa.LLMAttachment) *openai.ChatCompletionContentPartUnionParam {
	imagePart := openai.ChatCompletionContentPartImageParam{
		Type: "image_url",
		ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
			URL: att.URL,
		},
	}

	return &openai.ChatCompletionContentPartUnionParam{
		OfImageURL: &imagePart,
	}
}

func (p *OpenAIOracle) convertTools(tools []eywa.OracleTool) []openai.ChatCompletionToolParam {
	oaiTools := make([]openai.ChatCompletionToolParam, len(tools))

	for i, tool := range tools {
		oaiTools[i] = p.createToolParam(tool)
	}

	return oaiTools
}

func (p *OpenAIOracle) createToolParam(tool eywa.OracleTool) openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  tool.Parameters,
		},
	}
}
