package bedrock

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	eywa "github.com/wmulabs/eywa"
)

const ProviderName = "bedrock"

// BedrockOracle implements ports.Oracle via AWS Bedrock's Converse API.
// The Converse API provides a uniform interface for all Bedrock models —
// Claude, Llama, Nova, Titan — without model-specific request formats.
type BedrockOracle struct {
	client *bedrockruntime.Client
	config Config
}

type Config struct {
	Region string
	// Optional explicit credentials. Defaults to the standard AWS credential chain
	// (env vars → ~/.aws/credentials → IAM instance role → ECS/EKS task role).
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// NewOracle creates a BedrockOracle using the standard AWS credential chain.
func NewOracle(ctx context.Context, region string) (*BedrockOracle, error) {
	return NewOracleWithConfig(ctx, Config{Region: region})
}

func NewOracleWithConfig(ctx context.Context, cfg Config) (*BedrockOracle, error) {
	opts := []func(*awsconfig.LoadOptions) error{}

	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("bedrock: load aws config: %w", err)
	}

	return &BedrockOracle{
		client: bedrockruntime.NewFromConfig(awsCfg),
		config: cfg,
	}, nil
}

func (o *BedrockOracle) GetName() string   { return ProviderName }
func (o *BedrockOracle) IsAvailable() bool { return o.client != nil }

func (o *BedrockOracle) GetAvailableModels() []string {
	return []string{
		"anthropic.claude-3-5-sonnet-20241022-v2:0",
		"anthropic.claude-3-5-haiku-20241022-v1:0",
		"anthropic.claude-3-opus-20240229-v1:0",
		"anthropic.claude-3-sonnet-20240229-v1:0",
		"meta.llama3-2-90b-instruct-v1:0",
		"meta.llama3-2-11b-instruct-v1:0",
		"meta.llama3-1-70b-instruct-v1:0",
		"amazon.nova-pro-v1:0",
		"amazon.nova-lite-v1:0",
		"amazon.nova-micro-v1:0",
	}
}

func (o *BedrockOracle) GetConfig() map[string]any {
	return map[string]any{
		"provider": ProviderName,
		"region":   o.config.Region,
	}
}

func (o *BedrockOracle) SupportsImages(_ string) bool    { return true }
func (o *BedrockOracle) SupportsAudio(_ string) bool     { return false }
func (o *BedrockOracle) SupportsDocuments(_ string) bool { return false }

func (o *BedrockOracle) GenerateResponse(ctx context.Context, req *eywa.OracleRequest) (*eywa.OracleResponse, error) {
	input := &bedrockruntime.ConverseInput{
		ModelId:  aws.String(req.Model),
		Messages: o.convertMessages(req),
	}

	if req.SystemPrompt != "" {
		input.System = []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: req.SystemPrompt},
		}
	}

	if req.Temperature > 0 || req.MaxTokens > 0 || req.TopP > 0 {
		input.InferenceConfig = &types.InferenceConfiguration{}
		if req.Temperature > 0 {
			input.InferenceConfig.Temperature = aws.Float32(float32(req.Temperature))
		}
		if req.MaxTokens > 0 {
			input.InferenceConfig.MaxTokens = aws.Int32(int32(req.MaxTokens))
		}
		if req.TopP > 0 {
			input.InferenceConfig.TopP = aws.Float32(float32(req.TopP))
		}
	}

	if req.UseTools && len(req.Tools) > 0 {
		toolCfg, err := o.convertTools(req.Tools)
		if err != nil {
			return nil, err
		}
		input.ToolConfig = toolCfg
	}

	output, err := o.client.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock converse: %w", err)
	}

	return o.parseResponse(output)
}

// ── Message conversion ────────────────────────────────────────────────────────

func (o *BedrockOracle) convertMessages(req *eywa.OracleRequest) []types.Message {
	msgs := make([]types.Message, 0, len(req.Messages))

	for i, msg := range req.Messages {
		converted := o.convertMessage(msg, i == len(req.Messages)-1, req.Attachments)
		if converted != nil {
			msgs = append(msgs, *converted)
		}
	}

	return msgs
}

func (o *BedrockOracle) convertMessage(msg eywa.OracleMessage, isLast bool, attachments []eywa.LLMAttachment) *types.Message {
	switch msg.Role {
	case eywa.RoleUser:
		return o.convertUserMessage(msg, isLast, attachments)
	case eywa.RoleAssistant:
		return o.convertAssistantMessage(msg)
	case eywa.RoleTool:
		return o.convertToolResultMessage(msg)
	default:
		return nil
	}
}

func (o *BedrockOracle) convertUserMessage(msg eywa.OracleMessage, isLast bool, attachments []eywa.LLMAttachment) *types.Message {
	var content []types.ContentBlock

	if msg.Content != "" {
		content = append(content, &types.ContentBlockMemberText{Value: msg.Content})
	}

	if isLast {
		for _, att := range attachments {
			if att.Type == eywa.ArtifactTypeImage && len(att.Data) > 0 {
				content = append(content, o.imageBlock(att))
			}
		}
	}

	if len(content) == 0 {
		return nil
	}

	return &types.Message{
		Role:    types.ConversationRoleUser,
		Content: content,
	}
}

func (o *BedrockOracle) convertAssistantMessage(msg eywa.OracleMessage) *types.Message {
	var content []types.ContentBlock

	if msg.Content != "" {
		content = append(content, &types.ContentBlockMemberText{Value: msg.Content})
	}

	for _, tc := range msg.ToolCalls {
		content = append(content, &types.ContentBlockMemberToolUse{
			Value: types.ToolUseBlock{
				ToolUseId: aws.String(tc.ID),
				Name:      aws.String(tc.Name),
				Input:     document.NewLazyDocument(tc.Arguments),
			},
		})
	}

	if len(content) == 0 {
		return nil
	}

	return &types.Message{
		Role:    types.ConversationRoleAssistant,
		Content: content,
	}
}

// convertToolResultMessage converts a tool result (role=tool) back to a user
// turn containing a ToolResultBlock, as required by the Converse API.
func (o *BedrockOracle) convertToolResultMessage(msg eywa.OracleMessage) *types.Message {
	return &types.Message{
		Role: types.ConversationRoleUser,
		Content: []types.ContentBlock{
			&types.ContentBlockMemberToolResult{
				Value: types.ToolResultBlock{
					ToolUseId: aws.String(msg.ToolName),
					Content: []types.ToolResultContentBlock{
						&types.ToolResultContentBlockMemberText{Value: msg.Content},
					},
				},
			},
		},
	}
}

func (o *BedrockOracle) imageBlock(att eywa.LLMAttachment) types.ContentBlock {
	mimeType := att.MimeType
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	format := types.ImageFormatJpeg
	switch mimeType {
	case "image/png":
		format = types.ImageFormatPng
	case "image/gif":
		format = types.ImageFormatGif
	case "image/webp":
		format = types.ImageFormatWebp
	}

	return &types.ContentBlockMemberImage{
		Value: types.ImageBlock{
			Format: format,
			Source: &types.ImageSourceMemberBytes{
				Value: att.Data,
			},
		},
	}
}

// ── Tool conversion ───────────────────────────────────────────────────────────

func (o *BedrockOracle) convertTools(tools []eywa.OracleTool) (*types.ToolConfiguration, error) {
	bedTools := make([]types.Tool, 0, len(tools))

	for _, t := range tools {
		bedTools = append(bedTools, &types.ToolMemberToolSpec{
			Value: types.ToolSpecification{
				Name:        aws.String(t.Name),
				Description: aws.String(t.Description),
				InputSchema: &types.ToolInputSchemaMemberJson{
					Value: document.NewLazyDocument(t.Parameters),
				},
			},
		})
	}

	return &types.ToolConfiguration{Tools: bedTools}, nil
}

// ── Response parsing ──────────────────────────────────────────────────────────

func (o *BedrockOracle) parseResponse(output *bedrockruntime.ConverseOutput) (*eywa.OracleResponse, error) {
	msgOutput, ok := output.Output.(*types.ConverseOutputMemberMessage)
	if !ok {
		return nil, fmt.Errorf("bedrock: unexpected output type %T", output.Output)
	}

	var text string
	var toolCalls []eywa.OracleToolCall

	for _, block := range msgOutput.Value.Content {
		switch b := block.(type) {
		case *types.ContentBlockMemberText:
			text += b.Value
		case *types.ContentBlockMemberToolUse:
			tc, err := o.parseToolUse(b.Value)
			if err != nil {
				return nil, err
			}
			toolCalls = append(toolCalls, tc)
		}
	}

	return &eywa.OracleResponse{
		Content:    text,
		ToolCalls:  toolCalls,
		StopReason: o.normalizeStopReason(string(output.StopReason)),
		TokensUsed: eywa.OracleUsage{
			PromptTokens:     int(aws.ToInt32(output.Usage.InputTokens)),
			CompletionTokens: int(aws.ToInt32(output.Usage.OutputTokens)),
			TotalTokens:      int(aws.ToInt32(output.Usage.TotalTokens)),
		},
	}, nil
}

func (o *BedrockOracle) parseToolUse(tu types.ToolUseBlock) (eywa.OracleToolCall, error) {
	inputBytes, err := json.Marshal(tu.Input)
	if err != nil {
		return eywa.OracleToolCall{}, fmt.Errorf("bedrock: marshal tool input: %w", err)
	}

	var args map[string]any
	if err := json.Unmarshal(inputBytes, &args); err != nil {
		return eywa.OracleToolCall{}, fmt.Errorf("bedrock: unmarshal tool input: %w", err)
	}

	return eywa.OracleToolCall{
		ID:        aws.ToString(tu.ToolUseId),
		Name:      aws.ToString(tu.Name),
		Arguments: args,
	}, nil
}

func (o *BedrockOracle) normalizeStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return eywa.StopReasonComplete
	case "max_tokens":
		return eywa.StopReasonLength
	case "tool_use":
		return eywa.StopReasonToolCalls
	case "content_filtered":
		return eywa.StopReasonContentFilter
	default:
		return reason
	}
}
