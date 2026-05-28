package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	eywa "github.com/wmulabs/eywa"
)

const (
	ProviderName = "anthropic"

	defaultMaxRetries = 3
	defaultTimeout    = 60 // seconds

	// defaultImageMimeType is used when no MIME type is present on an attachment.
	defaultImageMimeType = "image/jpeg"
)

type AnthropicOracle struct {
	client anthropic.Client
	apiKey string
	config Config
}

type Config struct {
	APIKey     string
	BaseURL    string
	MaxRetries int
	Timeout    int // seconds
}

func NewOracle(apiKey string) *AnthropicOracle {
	return NewOracleWithConfig(Config{
		APIKey:     apiKey,
		MaxRetries: defaultMaxRetries,
		Timeout:    defaultTimeout,
	})
}

func NewOracleWithConfig(config Config) *AnthropicOracle {
	client := createAnthropicClient(config)

	return &AnthropicOracle{
		client: client,
		apiKey: config.APIKey,
		config: config,
	}
}

func createAnthropicClient(config Config) anthropic.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(config.APIKey),
	}

	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}

	if config.MaxRetries > 0 {
		opts = append(opts, option.WithMaxRetries(config.MaxRetries))
	}

	return anthropic.NewClient(opts...)
}

func (p *AnthropicOracle) GetName() string {
	return ProviderName
}

func (p *AnthropicOracle) GetAvailableModels() []string {
	return []string{
		// Claude 4.x (current generation)
		"claude-opus-4-7",
		"claude-sonnet-4-6",
		"claude-haiku-4-5",
		"claude-haiku-4-5-20251001",
		"claude-opus-4-6",
		"claude-opus-4-5",
		"claude-opus-4-5-20251101",
		"claude-opus-4-1",
		"claude-opus-4-1-20250805",
		"claude-sonnet-4-5",
		"claude-sonnet-4-5-20250929",

		// Claude 3.x (still available)
		"claude-3-7-sonnet-20250219",
		"claude-3-7-sonnet-latest",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-sonnet-latest",
		"claude-3-5-haiku-20241022",
		"claude-3-5-haiku-latest",
	}
}

func (p *AnthropicOracle) GenerateResponse(ctx context.Context, req *eywa.OracleRequest) (*eywa.OracleResponse, error) {
	params := p.buildRequestParams(req)

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic api error: %w", err)
	}

	return p.parseResponse(resp)
}

func (p *AnthropicOracle) buildRequestParams(req *eywa.OracleRequest) anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		Model:       req.Model,
		Messages:    p.convertMessages(req),
		MaxTokens:   int64(req.MaxTokens),
		Temperature: anthropic.Float(float64(req.Temperature)),
	}

	p.addSystemPrompt(&params, req.SystemPrompt)
	p.addSamplingParameters(&params, req)
	p.addTools(&params, req)

	return params
}

func (p *AnthropicOracle) addSystemPrompt(params *anthropic.MessageNewParams, systemPrompt string) {
	if systemPrompt == "" {
		return
	}

	params.System = []anthropic.TextBlockParam{
		{
			Type: "text",
			Text: systemPrompt,
		},
	}
}

func (p *AnthropicOracle) addSamplingParameters(params *anthropic.MessageNewParams, req *eywa.OracleRequest) {
	if req.TopP > 0 {
		params.TopP = anthropic.Float(float64(req.TopP))
	}

	if req.TopK > 0 {
		params.TopK = anthropic.Int(int64(req.TopK))
	}

	if len(req.StopSequences) > 0 {
		params.StopSequences = req.StopSequences
	}
}

func (p *AnthropicOracle) addTools(params *anthropic.MessageNewParams, req *eywa.OracleRequest) {
	if !req.UseTools || len(req.Tools) == 0 {
		return
	}

	params.Tools = p.convertTools(req.Tools)
}

func (p *AnthropicOracle) parseResponse(resp *anthropic.Message) (*eywa.OracleResponse, error) {
	content, toolCalls, err := p.extractContentAndToolCalls(resp.Content)
	if err != nil {
		return nil, err
	}

	return &eywa.OracleResponse{
		Content:    content,
		ToolCalls:  toolCalls,
		StopReason: p.normalizeStopReason(string(resp.StopReason)),
		TokensUsed: p.extractTokenUsage(resp.Usage),
	}, nil
}

func (p *AnthropicOracle) extractContentAndToolCalls(contentBlocks []anthropic.ContentBlockUnion) (string, []eywa.OracleToolCall, error) {
	var content string
	var toolCalls []eywa.OracleToolCall

	for _, block := range contentBlocks {
		switch block.Type {
		case "text":
			content += block.Text

		case "tool_use":
			toolCall, err := p.parseToolUseBlock(block)
			if err != nil {
				return "", nil, err
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	return content, toolCalls, nil
}

func (p *AnthropicOracle) parseToolUseBlock(block anthropic.ContentBlockUnion) (eywa.OracleToolCall, error) {
	var args map[string]any
	if err := json.Unmarshal(block.Input, &args); err != nil {
		return eywa.OracleToolCall{}, fmt.Errorf("failed to parse tool input: %w", err)
	}

	return eywa.OracleToolCall{
		ID:        block.ID,
		Name:      block.Name,
		Arguments: args,
	}, nil
}

func (p *AnthropicOracle) extractTokenUsage(usage anthropic.Usage) eywa.OracleUsage {
	return eywa.OracleUsage{
		PromptTokens:     int(usage.InputTokens),
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens:      int(usage.InputTokens + usage.OutputTokens),
	}
}

// normalizeStopReason maps Anthropic stop reasons to standard StopReason constants.
// https://docs.anthropic.com/claude/reference/messages_post
func (p *AnthropicOracle) normalizeStopReason(stopReason string) string {
	switch stopReason {
	case "end_turn":
		return eywa.StopReasonComplete
	case "max_tokens":
		return eywa.StopReasonLength
	case "tool_use":
		return eywa.StopReasonToolCalls
	case "stop_sequence":
		// Custom stop sequence hit — treat as natural completion.
		return eywa.StopReasonComplete
	default:
		return stopReason
	}
}

func (p *AnthropicOracle) IsAvailable() bool {
	return p.apiKey != ""
}

func (p *AnthropicOracle) GetConfig() map[string]any {
	return map[string]any{
		"provider":    ProviderName,
		"base_url":    p.config.BaseURL,
		"has_api_key": p.apiKey != "",
		"max_retries": p.config.MaxRetries,
		"timeout":     p.config.Timeout,
	}
}

// Claude 3+ models include native vision support.
func (p *AnthropicOracle) SupportsImages(model string) bool {
	visionPrefixes := []string{"claude-3", "claude-opus-4", "claude-sonnet-4", "claude-haiku-4"}
	for _, prefix := range visionPrefixes {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// Anthropic does not support audio input in the messages API.
func (p *AnthropicOracle) SupportsAudio(_ string) bool {
	return false
}

// Claude 3+ models support PDF via base64 in the messages API.
func (p *AnthropicOracle) SupportsDocuments(model string) bool {
	return p.SupportsImages(model)
}

func (p *AnthropicOracle) convertMessages(req *eywa.OracleRequest) []anthropic.MessageParam {
	messages := make([]anthropic.MessageParam, 0, len(req.Messages))

	for _, msg := range req.Messages {
		contentBlocks := p.buildContentBlocks(msg, req)

		if len(contentBlocks) > 0 {
			messages = append(messages, anthropic.MessageParam{
				Role:    p.convertRole(msg.Role),
				Content: contentBlocks,
			})
		}
	}

	return messages
}

func (p *AnthropicOracle) buildContentBlocks(msg eywa.OracleMessage, req *eywa.OracleRequest) []anthropic.ContentBlockParamUnion {
	var contentBlocks []anthropic.ContentBlockParamUnion

	if msg.Content != "" {
		contentBlocks = append(contentBlocks, anthropic.NewTextBlock(msg.Content))
	}

	if msg.Role == eywa.RoleUser && len(req.Attachments) > 0 {
		imageBlocks := p.convertAttachmentsToImageBlocks(req.Attachments)
		contentBlocks = append(contentBlocks, imageBlocks...)
	}

	if len(msg.ToolCalls) > 0 {
		contentBlocks = append(contentBlocks, p.convertToolCallsToBlocks(msg.ToolCalls)...)
	}

	// Tool result is stored in the Content field and must be sent as a user-role message.
	if msg.Role == eywa.RoleTool && msg.Content != "" {
		contentBlocks = append(contentBlocks, p.createToolResultBlock(msg))
	}

	return contentBlocks
}

// Anthropic expects tool results inside user messages.
func (p *AnthropicOracle) convertRole(role string) anthropic.MessageParamRole {
	if role == eywa.RoleTool {
		return anthropic.MessageParamRoleUser
	}
	return anthropic.MessageParamRole(role)
}

func (p *AnthropicOracle) convertToolCallsToBlocks(toolCalls []eywa.OracleToolCall) []anthropic.ContentBlockParamUnion {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(toolCalls))

	for _, tc := range toolCalls {
		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
	}

	return blocks
}

func (p *AnthropicOracle) createToolResultBlock(msg eywa.OracleMessage) anthropic.ContentBlockParamUnion {
	// Fall back to ToolName when ToolCallID is absent (older history entries).
	toolCallID := msg.ToolCallID
	if toolCallID == "" {
		toolCallID = msg.ToolName
	}
	return anthropic.NewToolResultBlock(toolCallID, msg.Content, msg.IsError)
}

func (p *AnthropicOracle) convertTools(tools []eywa.OracleTool) []anthropic.ToolUnionParam {
	anthropicTools := make([]anthropic.ToolUnionParam, 0, len(tools))

	for _, tool := range tools {
		toolParam := p.createToolParam(tool)
		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{
			OfTool: &toolParam,
		})
	}

	return anthropicTools
}

func (p *AnthropicOracle) createToolParam(tool eywa.OracleTool) anthropic.ToolParam {
	toolParam := anthropic.ToolParam{
		Name:        tool.Name,
		InputSchema: p.buildInputSchema(tool.Parameters),
	}

	if tool.Description != "" {
		toolParam.Description = anthropic.String(tool.Description)
	}

	return toolParam
}

func (p *AnthropicOracle) buildInputSchema(parameters map[string]any) anthropic.ToolInputSchemaParam {
	schema := anthropic.ToolInputSchemaParam{
		Type: "object",
	}

	if props, ok := parameters["properties"]; ok {
		schema.Properties = props
	}

	schema.Required = p.extractRequiredFields(parameters)

	return schema
}

func (p *AnthropicOracle) extractRequiredFields(parameters map[string]any) []string {
	required, ok := parameters["required"]
	if !ok {
		return nil
	}

	if requiredStrs, ok := required.([]string); ok {
		return requiredStrs
	}

	if requiredInterfaces, ok := required.([]any); ok {
		return p.convertInterfaceSliceToStringSlice(requiredInterfaces)
	}

	return nil
}

func (p *AnthropicOracle) convertInterfaceSliceToStringSlice(interfaces []any) []string {
	strings := make([]string, 0, len(interfaces))

	for _, item := range interfaces {
		if str, ok := item.(string); ok {
			strings = append(strings, str)
		}
	}

	return strings
}

// Only image attachments are processed; other types are handled upstream.
func (p *AnthropicOracle) convertAttachmentsToImageBlocks(attachments []eywa.LLMAttachment) []anthropic.ContentBlockParamUnion {
	blocks := make([]anthropic.ContentBlockParamUnion, 0)

	for _, att := range attachments {
		if !p.isImageAttachment(att) {
			continue
		}

		block := p.createImageBlock(att)
		if block != nil {
			blocks = append(blocks, *block)
		}
	}

	return blocks
}

func (p *AnthropicOracle) isImageAttachment(att eywa.LLMAttachment) bool {
	return att.Type == eywa.ArtifactTypeImage
}

func (p *AnthropicOracle) createImageBlock(att eywa.LLMAttachment) *anthropic.ContentBlockParamUnion {
	// Prefer binary data over URL for privacy and to avoid expiry issues.
	if len(att.Data) > 0 {
		return p.createBase64ImageBlock(att)
	}

	if att.URL != "" {
		return p.createURLImageBlock(att)
	}

	return nil
}

func (p *AnthropicOracle) createBase64ImageBlock(att eywa.LLMAttachment) *anthropic.ContentBlockParamUnion {
	base64Data := base64.StdEncoding.EncodeToString(att.Data)
	mediaType := p.determineImageMediaType(att.MimeType)

	return &anthropic.ContentBlockParamUnion{
		OfImage: &anthropic.ImageBlockParam{
			Source: anthropic.ImageBlockParamSourceUnion{
				OfBase64: &anthropic.Base64ImageSourceParam{
					Type:      "base64",
					MediaType: mediaType,
					Data:      base64Data,
				},
			},
		},
	}
}

func (p *AnthropicOracle) createURLImageBlock(att eywa.LLMAttachment) *anthropic.ContentBlockParamUnion {
	return &anthropic.ContentBlockParamUnion{
		OfImage: &anthropic.ImageBlockParam{
			Source: anthropic.ImageBlockParamSourceUnion{
				OfURL: &anthropic.URLImageSourceParam{
					URL: att.URL,
				},
			},
		},
	}
}

func (p *AnthropicOracle) determineImageMediaType(mimeType string) anthropic.Base64ImageSourceMediaType {
	if mimeType == "" {
		return anthropic.Base64ImageSourceMediaTypeImageJPEG
	}

	switch mimeType {
	case "image/jpeg", "image/jpg":
		return anthropic.Base64ImageSourceMediaTypeImageJPEG
	case "image/png":
		return anthropic.Base64ImageSourceMediaTypeImagePNG
	case "image/gif":
		return anthropic.Base64ImageSourceMediaTypeImageGIF
	case "image/webp":
		return anthropic.Base64ImageSourceMediaTypeImageWebP
	default:
		return anthropic.Base64ImageSourceMediaTypeImageJPEG
	}
}
