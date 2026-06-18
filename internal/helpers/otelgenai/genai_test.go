package otelgenai

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// fakeSpan records SetAttributes/AddEvent calls without pulling in the OTel SDK. Only the methods the
// helpers use are overridden; the embedded interface is never otherwise invoked.
type fakeSpan struct {
	trace.Span
	recording bool
	attrs     map[string]attribute.Value
	events    []string
}

func newFakeSpan(recording bool) *fakeSpan {
	return &fakeSpan{recording: recording, attrs: map[string]attribute.Value{}}
}

func (s *fakeSpan) IsRecording() bool { return s.recording }

func (s *fakeSpan) SetAttributes(kv ...attribute.KeyValue) {
	for _, a := range kv {
		s.attrs[string(a.Key)] = a.Value
	}
}

func (s *fakeSpan) AddEvent(name string, _ ...trace.EventOption) {
	s.events = append(s.events, name)
}

func TestSetLLMCallAttributes(t *testing.T) {
	span := newFakeSpan(true)
	SetLLMCallAttributes(span, LLMCall{
		System:       "openai",
		Model:        "gpt-4o",
		Temperature:  0.7,
		MaxTokens:    256,
		InputTokens:  10,
		OutputTokens: 5,
		FinishReason: "stop",
	})

	if got := span.attrs[AttrSystem].AsString(); got != "openai" {
		t.Errorf("system = %q", got)
	}
	if got := span.attrs[AttrRequestModel].AsString(); got != "gpt-4o" {
		t.Errorf("model = %q", got)
	}
	if got := span.attrs[AttrTemperature].AsFloat64(); got != 0.7 {
		t.Errorf("temperature = %v", got)
	}
	if got := span.attrs[AttrInputTokens].AsInt64(); got != 10 {
		t.Errorf("input tokens = %d", got)
	}
	if got := span.attrs[AttrFinishReason].AsString(); got != "stop" {
		t.Errorf("finish reason = %q", got)
	}
}

func TestSetLLMCallAttributes_OmitsZeroValues(t *testing.T) {
	span := newFakeSpan(true)
	SetLLMCallAttributes(span, LLMCall{System: "ollama", Model: "llama3.1"})

	for _, k := range []string{AttrTemperature, AttrMaxTokens, AttrInputTokens, AttrOutputTokens, AttrFinishReason} {
		if _, ok := span.attrs[k]; ok {
			t.Errorf("expected %s omitted for zero value", k)
		}
	}
}

func TestSetLLMCallAttributes_NotRecording(t *testing.T) {
	span := newFakeSpan(false)
	SetLLMCallAttributes(span, LLMCall{System: "openai", Model: "gpt-4o"})
	if len(span.attrs) != 0 {
		t.Error("expected no attributes on a non-recording span")
	}
}

func TestSetTurnAttributes(t *testing.T) {
	span := newFakeSpan(true)
	SetTurnAttributes(span, Turn{MemoryKey: "user:1", Spirit: "helper", Iterations: 3, InputTokens: 20, OutputTokens: 8, Status: "success"})

	if got := span.attrs[AttrEywaMemoryKey].AsString(); got != "user:1" {
		t.Errorf("memory_key = %q", got)
	}
	if got := span.attrs[AttrEywaIterations].AsInt64(); got != 3 {
		t.Errorf("iterations = %d", got)
	}
	if got := span.attrs[AttrEywaStatus].AsString(); got != "success" {
		t.Errorf("status = %q", got)
	}
}

func TestSetTurnAttributes_NotRecording(t *testing.T) {
	span := newFakeSpan(false)
	SetTurnAttributes(span, Turn{MemoryKey: "user:1", Spirit: "helper", Iterations: 1})
	if len(span.attrs) != 0 {
		t.Error("expected no attributes on a non-recording span")
	}
}

func TestSetToolName(t *testing.T) {
	span := newFakeSpan(true)
	SetToolName(span, "lookup")
	if got := span.attrs[AttrToolName].AsString(); got != "lookup" {
		t.Errorf("tool name = %q", got)
	}

	off := newFakeSpan(false)
	SetToolName(off, "lookup")
	if len(off.attrs) != 0 {
		t.Error("expected no tool attribute on a non-recording span")
	}
}

func TestContentCaptureGate(t *testing.T) {
	t.Cleanup(func() { SetCaptureContent(false) })

	span := newFakeSpan(true)
	SetCaptureContent(false)
	AddPromptEvent(span, "secret prompt")
	if len(span.events) != 0 {
		t.Error("expected no events when content capture is off")
	}

	SetCaptureContent(true)
	if !CaptureContentEnabled() {
		t.Error("expected capture enabled")
	}
	AddPromptEvent(span, "hello")
	AddCompletionEvent(span, "world")
	AddCompletionEvent(span, "") // empty content is skipped
	if len(span.events) != 2 {
		t.Errorf("expected 2 content events, got %d", len(span.events))
	}
}

func TestContentEvents_NilSpanSafe(t *testing.T) {
	SetCaptureContent(true)
	t.Cleanup(func() { SetCaptureContent(false) })
	// Must not panic.
	AddPromptEvent(nil, "x")
	AddCompletionEvent(nil, "x")
}
