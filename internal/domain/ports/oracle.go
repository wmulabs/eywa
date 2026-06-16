package ports

import (
	"context"
	"encoding/json"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	// RoleTool is the role used for Action execution results returned to the Oracle.
	RoleTool = "tool"
)

// Normalized stop reason constants — each provider uses different raw strings;
// Oracle implementations must map their native values to these before returning.
const (
	// StopReasonComplete: OpenAI "stop", Anthropic "end_turn", Gemini "stop"
	StopReasonComplete = "stop"
	// StopReasonLength: OpenAI "length", Anthropic "max_tokens", Gemini "max_tokens"
	StopReasonLength = "length"
	// StopReasonToolCalls: OpenAI "tool_calls", Anthropic "tool_use"
	StopReasonToolCalls = "tool_calls"
	// StopReasonContentFilter: OpenAI "content_filter"
	StopReasonContentFilter = "content_filter"
)

type OracleRequest struct {
	Model         string
	SystemPrompt  string
	Messages      []OracleMessage
	Temperature   float64
	MaxTokens     int
	TopP          float64
	TopK          int // not supported by all providers
	StopSequences []string
	Tools         []OracleTool
	UseTools      bool
	Metadata      map[string]any
	Attachments   []LLMAttachment

	// ResponseFormat, when set, requests a response conforming to a JSON Schema. Providers use their
	// native structured-output mode where available; otherwise the caller falls back to instruct +
	// validate. nil = free-form generation (default).
	ResponseFormat *ResponseFormat
}

// ResponseFormat describes a JSON Schema the model's response must conform to.
type ResponseFormat struct {
	Name   string         // schema name (provider-facing label)
	Schema map[string]any // JSON Schema (object)
	Strict bool           // reject responses with fields not in the schema
}

type OracleMessage struct {
	Role      string
	Content   string
	ImageURLs []string
	AudioURLs []string

	// Set when Role == RoleAssistant and the Oracle is requesting Action execution.
	ToolCalls []OracleToolCall

	// Set when Role == RoleTool. Content holds the Action result.
	ToolCallID string
	ToolName   string
	// IsError signals an Action failure at the protocol level — providers that support it
	// (e.g. Anthropic) use this flag to help the model distinguish failures from successful results.
	IsError bool
}

// LLMAttachment is the Oracle-side representation of a media artifact.
// Providers must prefer Data over URL when both are present — Data avoids URL expiry
// and keeps the payload self-contained (better for privacy).
type LLMAttachment struct {
	Type     entities.ArtifactType
	URL      string
	Data     []byte
	MimeType string
	Caption  string
}

// OracleTool is the Action descriptor sent to the Oracle so it knows when and how to call an Action.
type OracleTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

type OracleToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	// ThoughtSignature is opaque provider data used by reasoning models (e.g. Gemini 2.5+/3.x)
	// to link internal reasoning steps to tool calls. Must be echoed back in subsequent requests.
	// Other providers leave this nil.
	ThoughtSignature []byte `json:"thought_signature,omitempty"`
	// IsCriticalOverride overrides the action's own IsCritical() for this specific call.
	// Set by the engine from the Spirit's AllowedAction config before execution.
	IsCriticalOverride *bool `json:"-"`
}

type OracleResponse struct {
	Content    string
	ToolCalls  []OracleToolCall
	StopReason string
	TokensUsed OracleUsage
	Metadata   map[string]any

	// Structured holds the validated JSON object when the request set ResponseFormat. Empty otherwise.
	Structured json.RawMessage
}

type OracleUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func (u *OracleUsage) Add(other OracleUsage) {
	u.PromptTokens += other.PromptTokens
	u.CompletionTokens += other.CompletionTokens
	u.TotalTokens += other.TotalTokens
}

type Oracle interface {
	GetName() string
	GetAvailableModels() []string
	GenerateResponse(ctx context.Context, req *OracleRequest) (*OracleResponse, error)
	IsAvailable() bool
	GetConfig() map[string]any

	// SupportsImages returns whether the model can process image attachments natively.
	// When false, the MediaProcessingStep runs Eye as a transcription fallback.
	SupportsImages(model string) bool

	// SupportsAudio returns whether the model can process audio attachments natively.
	// When false, the MediaProcessingStep runs Ear as a transcription fallback.
	SupportsAudio(model string) bool

	// SupportsDocuments returns whether the model can process document attachments natively.
	// When false, the MediaProcessingStep runs DocumentProcessor as a transcription fallback.
	SupportsDocuments(model string) bool
}

type OracleFactory interface {
	RegisterProvider(name string, provider Oracle) error
	GetProvider(name string) (Oracle, error)
	GetDefaultProvider() (Oracle, error)
	SetDefaultProvider(name string) error
	ListProviders() []string
	ListAvailableProviders() []string
	GetProviderForModel(model string) (Oracle, error)
}

// StreamEventType classifies an incremental event from a streaming Oracle generation.
type StreamEventType string

const (
	// StreamEventDelta carries a chunk of generated text.
	StreamEventDelta StreamEventType = "delta"
	// StreamEventDone is the final event; it carries usage and the stop reason.
	StreamEventDone StreamEventType = "done"
	// StreamEventError reports a mid-stream failure.
	StreamEventError StreamEventType = "error"
)

// StreamEvent is one incremental event from GenerateStream. Tool calls and the final content are
// assembled by the consumer from the Delta sequence plus the Done event.
type StreamEvent struct {
	Type       StreamEventType
	Delta      string
	ToolCalls  []OracleToolCall
	Usage      OracleUsage
	StopReason string
	Err        error
}

// StreamingOracle is the optional streaming capability of an Oracle. Providers implement it where the
// SDK supports it; the reasoning loop falls back to GenerateResponse for providers that do not.
type StreamingOracle interface {
	GenerateStream(ctx context.Context, req *OracleRequest) (<-chan StreamEvent, error)
}

// StructuredOracle is the optional native structured-output capability of an Oracle. Providers
// implement it where the SDK supports schema-constrained generation; callers fall back to instruct +
// validate for providers (or models) that report no native support.
type StructuredOracle interface {
	SupportsStructuredOutput(model string) bool
}
