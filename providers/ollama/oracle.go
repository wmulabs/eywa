// Package ollama is a native Oracle for a local Ollama server, talking to its /api/chat endpoint.
// It needs no API key and defaults to http://localhost:11434, so agents run against local models
// (Llama, Qwen, Mistral, …) out of the box.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	eywa "github.com/wmulabs/eywa"
)

const (
	ProviderName     = "ollama"
	defaultHost      = "http://localhost:11434"
	defaultTimeout   = 120 * time.Second
	maxResponseBytes = 10 << 20 // 10 MiB cap on a single response body
)

// Config configures the Ollama Oracle. Host is operator-provided (not user input); only http/https
// schemes are accepted.
type Config struct {
	Name      string        // provider name registered on the Weave; defaults to "ollama"
	Host      string        // server address; defaults to http://localhost:11434
	Timeout   time.Duration // HTTP timeout; defaults to 120s
	KeepAlive string        // how long the model stays loaded after a request (e.g. "5m"); optional
}

type OllamaOracle struct {
	config Config
	client *http.Client
}

func NewOracle(config Config) *OllamaOracle {
	if config.Host == "" {
		config.Host = defaultHost
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	if config.Name == "" {
		config.Name = ProviderName
	}
	return &OllamaOracle{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

var _ eywa.Oracle = (*OllamaOracle)(nil)

func (p *OllamaOracle) GetName() string              { return p.config.Name }
func (p *OllamaOracle) GetAvailableModels() []string { return nil }
func (p *OllamaOracle) IsAvailable() bool            { return p.config.Host != "" }

func (p *OllamaOracle) GetConfig() map[string]any {
	return map[string]any{
		"name":       p.config.Name,
		"host":       p.config.Host,
		"keep_alive": p.config.KeepAlive,
	}
}

// Local models are served text-only here; media falls back to the transcription pipeline. Native
// vision for vision-capable models (llava, llama3.2-vision) is a follow-up.
func (p *OllamaOracle) SupportsImages(_ string) bool    { return false }
func (p *OllamaOracle) SupportsAudio(_ string) bool     { return false }
func (p *OllamaOracle) SupportsDocuments(_ string) bool { return false }

func (p *OllamaOracle) GenerateResponse(ctx context.Context, req *eywa.OracleRequest) (*eywa.OracleResponse, error) {
	payload, err := json.Marshal(p.buildChatRequest(req))
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Host+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ollama: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("ollama: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: api error: status %d: %s", resp.StatusCode, string(data))
	}

	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}

	return p.parseResponse(&cr), nil
}

func (p *OllamaOracle) buildChatRequest(req *eywa.OracleRequest) chatRequest {
	cr := chatRequest{
		Model:     req.Model,
		Messages:  p.convertMessages(req),
		Stream:    false,
		KeepAlive: p.config.KeepAlive,
		Options:   buildOptions(req),
	}
	if req.UseTools && len(req.Tools) > 0 {
		cr.Tools = convertTools(req.Tools)
	}
	return cr
}

func (p *OllamaOracle) convertMessages(req *eywa.OracleRequest) []chatMessage {
	msgs := make([]chatMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		msgs = append(msgs, chatMessage{Role: eywa.RoleSystem, Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		msg := chatMessage{Role: m.Role, Content: m.Content}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, chatToolCall{Function: chatToolCallFunc{Name: tc.Name, Arguments: tc.Arguments}})
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

func buildOptions(req *eywa.OracleRequest) map[string]any {
	opts := map[string]any{}
	if req.Temperature > 0 {
		opts["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		opts["num_predict"] = req.MaxTokens
	}
	if req.TopP > 0 {
		opts["top_p"] = req.TopP
	}
	if req.TopK > 0 {
		opts["top_k"] = req.TopK
	}
	if len(req.StopSequences) > 0 {
		opts["stop"] = req.StopSequences
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

func convertTools(tools []eywa.OracleTool) []chatTool {
	out := make([]chatTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, chatTool{
			Type: "function",
			Function: chatToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

func (p *OllamaOracle) parseResponse(cr *chatResponse) *eywa.OracleResponse {
	var toolCalls []eywa.OracleToolCall
	for _, tc := range cr.Message.ToolCalls {
		toolCalls = append(toolCalls, eywa.OracleToolCall{
			ID:        tc.Function.Name, // Ollama omits IDs; the name correlates the result
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	stopReason := normalizeStopReason(cr.DoneReason)
	if len(toolCalls) > 0 {
		stopReason = eywa.StopReasonToolCalls
	}

	return &eywa.OracleResponse{
		Content:    cr.Message.Content,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		TokensUsed: eywa.OracleUsage{
			PromptTokens:     cr.PromptEvalCount,
			CompletionTokens: cr.EvalCount,
			TotalTokens:      cr.PromptEvalCount + cr.EvalCount,
		},
	}
}

func normalizeStopReason(doneReason string) string {
	switch doneReason {
	case "length":
		return eywa.StopReasonLength
	case "stop", "":
		return eywa.StopReasonComplete
	default:
		return eywa.StopReasonComplete
	}
}

type chatRequest struct {
	Model     string         `json:"model"`
	Messages  []chatMessage  `json:"messages"`
	Stream    bool           `json:"stream"`
	Tools     []chatTool     `json:"tools,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
	KeepAlive string         `json:"keep_alive,omitempty"`
}

type chatMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []chatToolCall `json:"tool_calls,omitempty"`
}

type chatToolCall struct {
	Function chatToolCallFunc `json:"function"`
}

type chatToolCallFunc struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatToolFunc `json:"function"`
}

type chatToolFunc struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type chatResponse struct {
	Model           string              `json:"model"`
	Message         chatResponseMessage `json:"message"`
	DoneReason      string              `json:"done_reason"`
	PromptEvalCount int                 `json:"prompt_eval_count"`
	EvalCount       int                 `json:"eval_count"`
}

type chatResponseMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []chatToolCall `json:"tool_calls"`
}
