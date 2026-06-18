// Package otelgenai sets OpenTelemetry GenAI semantic-convention attributes on spans. It is a pure
// utility (no ports, no infrastructure) so the domain can enrich its spans without importing adapters.
package otelgenai

import (
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// GenAI semantic-convention attribute keys (OpenTelemetry gen_ai.*) plus a few eywa.* turn-level
// keys. Centralised here so spans stay consistent and drop straight into Langfuse / Phoenix / Jaeger.
const (
	AttrSystem         = "gen_ai.system"
	AttrRequestModel   = "gen_ai.request.model"
	AttrTemperature    = "gen_ai.request.temperature"
	AttrMaxTokens      = "gen_ai.request.max_tokens"  //nolint:gosec // G101 false positive: OTel attribute key, not a credential
	AttrInputTokens    = "gen_ai.usage.input_tokens"  //nolint:gosec // G101 false positive: OTel attribute key, not a credential
	AttrOutputTokens   = "gen_ai.usage.output_tokens" //nolint:gosec // G101 false positive: OTel attribute key, not a credential
	AttrFinishReason   = "gen_ai.response.finish_reason"
	AttrToolName       = "gen_ai.tool.name"
	AttrOperationName  = "gen_ai.operation.name"
	AttrEywaMemoryKey  = "eywa.memory_key"
	AttrEywaSpirit     = "eywa.spirit"
	AttrEywaIterations = "eywa.iterations"
	AttrEywaStatus     = "eywa.status"
)

// captureContent gates recording prompt/response text on spans (sensitive: PII). Default off — only
// metadata and token counts are recorded. It is a process-wide switch, set once at startup.
var captureContent atomic.Bool

// SetCaptureContent enables or disables recording prompt/completion text as span events.
func SetCaptureContent(enabled bool) { captureContent.Store(enabled) }

// CaptureContentEnabled reports whether prompt/completion capture is on.
func CaptureContentEnabled() bool { return captureContent.Load() }

// LLMCall holds the GenAI attributes of a single model call.
type LLMCall struct {
	System       string // provider, e.g. "openai"
	Model        string
	Temperature  float64
	MaxTokens    int
	InputTokens  int
	OutputTokens int
	FinishReason string
}

// SetLLMCallAttributes annotates a per-call span with GenAI request/response attributes.
func SetLLMCallAttributes(span trace.Span, call LLMCall) {
	if span == nil || !span.IsRecording() {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String(AttrOperationName, "chat"),
		attribute.String(AttrSystem, call.System),
		attribute.String(AttrRequestModel, call.Model),
	}
	if call.Temperature > 0 {
		attrs = append(attrs, attribute.Float64(AttrTemperature, call.Temperature))
	}
	if call.MaxTokens > 0 {
		attrs = append(attrs, attribute.Int(AttrMaxTokens, call.MaxTokens))
	}
	if call.InputTokens > 0 {
		attrs = append(attrs, attribute.Int(AttrInputTokens, call.InputTokens))
	}
	if call.OutputTokens > 0 {
		attrs = append(attrs, attribute.Int(AttrOutputTokens, call.OutputTokens))
	}
	if call.FinishReason != "" {
		attrs = append(attrs, attribute.String(AttrFinishReason, call.FinishReason))
	}
	span.SetAttributes(attrs...)
}

// Turn holds the GenAI/eywa attributes of a full reasoning turn.
type Turn struct {
	MemoryKey    string
	Spirit       string
	Iterations   int
	InputTokens  int
	OutputTokens int
	Status       string
}

// SetTurnAttributes annotates the turn-level span with conversation and aggregate-usage attributes.
func SetTurnAttributes(span trace.Span, turn Turn) {
	if span == nil || !span.IsRecording() {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String(AttrEywaMemoryKey, turn.MemoryKey),
		attribute.String(AttrEywaSpirit, turn.Spirit),
		attribute.Int(AttrEywaIterations, turn.Iterations),
	}
	if turn.InputTokens > 0 {
		attrs = append(attrs, attribute.Int(AttrInputTokens, turn.InputTokens))
	}
	if turn.OutputTokens > 0 {
		attrs = append(attrs, attribute.Int(AttrOutputTokens, turn.OutputTokens))
	}
	if turn.Status != "" {
		attrs = append(attrs, attribute.String(AttrEywaStatus, turn.Status))
	}
	span.SetAttributes(attrs...)
}

// SetToolName tags a tool-execution span with the Action name.
func SetToolName(span trace.Span, name string) {
	if span == nil || !span.IsRecording() {
		return
	}
	span.SetAttributes(attribute.String(AttrToolName, name))
}

// AddPromptEvent records the prompt as a span event, only when content capture is enabled.
func AddPromptEvent(span trace.Span, content string) {
	addContentEvent(span, "gen_ai.content.prompt", content)
}

// AddCompletionEvent records the completion as a span event, only when content capture is enabled.
func AddCompletionEvent(span trace.Span, content string) {
	addContentEvent(span, "gen_ai.content.completion", content)
}

func addContentEvent(span trace.Span, name, content string) {
	if span == nil || !span.IsRecording() || !CaptureContentEnabled() || content == "" {
		return
	}
	span.AddEvent(name, trace.WithAttributes(attribute.String("content", content)))
}
