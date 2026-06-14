package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// notifierState builds a ProcessingState for NotificationStep tests.
func notifierState(spiritName, voiceName, mode string) *ProcessingState {
	return &ProcessingState{
		Event: &entities.Pulse{
			MemoryKey:   "user:1",
			UserMessage: "hello",
			Knowledge:   map[string]any{},
		},
		EventConfig: &entities.Link{VoiceName: voiceName},
		Spirit: &entities.Spirit{
			Name: spiritName,
			Type: entities.SpiritTypeNotifier,
			NotifierConfig: entities.NotifierConfig{
				Mode:     mode,
				Template: "Hello {{name}}",
			},
		},
	}
}

// --- NotificationStep ---

func TestNotificationStep_NilSpirit_NoOp(t *testing.T) {
	step := NewNotificationStep(&stubVoiceRegistry{}, &stubOracleFactory{}, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: "user:1"},
		EventConfig: &entities.Link{},
		Spirit:      nil,
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotificationStep_NonNotifierSpirit_NoOp(t *testing.T) {
	step := NewNotificationStep(&stubVoiceRegistry{}, &stubOracleFactory{}, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: "user:1"},
		EventConfig: &entities.Link{},
		Spirit:      &entities.Spirit{Name: "support", Type: entities.SpiritTypeConversational},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotificationStep_Skipped_NoOp(t *testing.T) {
	step := NewNotificationStep(&stubVoiceRegistry{}, &stubOracleFactory{}, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "template")
	state.Skipped = true
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotificationStep_EmptyVoiceName_NoOp(t *testing.T) {
	step := NewNotificationStep(&stubVoiceRegistry{}, &stubOracleFactory{}, time.Second, testLogger(t))
	state := notifierState("notifier", "", "template") // empty VoiceName
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotificationStep_VoiceNotFound_NoOp(t *testing.T) {
	reg := &stubVoiceRegistry{getErr: errors.New("not found")}
	step := NewNotificationStep(reg, &stubOracleFactory{}, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "template")
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotificationStep_TemplateMode_SendsResponse(t *testing.T) {
	voice := &stubVoice{}
	reg := &stubVoiceRegistry{voice: voice}
	step := NewNotificationStep(reg, &stubOracleFactory{}, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "template")
	state.Event.Knowledge["name"] = "Alice"

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !voice.sendCalled {
		t.Error("expected voice.SendResponse to be called")
	}
	if state.Response != "Hello Alice" {
		t.Errorf("expected 'Hello Alice', got %q", state.Response)
	}
	if !state.ResponseDelivered {
		t.Error("expected ResponseDelivered=true")
	}
}

func TestNotificationStep_TemplateMode_EmptyMessage_Skipped(t *testing.T) {
	voice := &stubVoice{}
	reg := &stubVoiceRegistry{voice: voice}
	step := NewNotificationStep(reg, &stubOracleFactory{}, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "template")
	state.Spirit.NotifierConfig.Template = "" // produces empty message

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if voice.sendCalled {
		t.Error("expected voice.SendResponse NOT to be called for empty message")
	}
	if !state.Skipped {
		t.Error("expected state.Skipped=true for empty message")
	}
}

func TestNotificationStep_LLMMode_SendsResponse(t *testing.T) {
	voice := &stubVoice{}
	reg := &stubVoiceRegistry{voice: voice}
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "Notification text"}}
	factory := &stubOracleFactory{oracle: oracle}
	step := NewNotificationStep(reg, factory, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "llm")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !voice.sendCalled {
		t.Error("expected voice.SendResponse to be called")
	}
	if state.Response != "Notification text" {
		t.Errorf("unexpected response: %q", state.Response)
	}
}

func TestNotificationStep_LLMMode_InjectsKnowledgeIntoPrompt(t *testing.T) {
	voice := &stubVoice{}
	reg := &stubVoiceRegistry{voice: voice}
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "ok"}}
	factory := &stubOracleFactory{oracle: oracle}
	step := NewNotificationStep(reg, factory, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "llm")
	state.Event.Knowledge["order_status"] = "delivered"

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if oracle.gotReq == nil {
		t.Fatal("expected oracle to receive a request")
	}
	if !strings.Contains(oracle.gotReq.SystemPrompt, "delivered") {
		t.Errorf("expected event Knowledge in system prompt, got: %q", oracle.gotReq.SystemPrompt)
	}
}

func TestNotificationStep_LLMMode_OracleProviderError_NoOp(t *testing.T) {
	voice := &stubVoice{}
	reg := &stubVoiceRegistry{voice: voice}
	factory := &stubOracleFactory{err: errors.New("no provider")}
	step := NewNotificationStep(reg, factory, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "llm")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("LLM errors must be non-fatal, got: %v", err)
	}
	if voice.sendCalled {
		t.Error("expected no send when LLM fails")
	}
}

func TestNotificationStep_LLMMode_OracleCallError_NoOp(t *testing.T) {
	voice := &stubVoice{}
	reg := &stubVoiceRegistry{voice: voice}
	oracle := &stubOracle{err: errors.New("llm timeout")}
	factory := &stubOracleFactory{oracle: oracle}
	step := NewNotificationStep(reg, factory, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "llm")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("LLM errors must be non-fatal, got: %v", err)
	}
	if voice.sendCalled {
		t.Error("expected no send when LLM oracle call fails")
	}
}

func TestNotificationStep_UnknownMode_NoOp(t *testing.T) {
	voice := &stubVoice{}
	reg := &stubVoiceRegistry{voice: voice}
	step := NewNotificationStep(reg, &stubOracleFactory{}, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "unknown-mode")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if voice.sendCalled {
		t.Error("expected no send for unknown mode")
	}
}

func TestNotificationStep_SendResponseFails_NoOp(t *testing.T) {
	voice := &stubVoice{sendErr: errors.New("delivery failed")}
	reg := &stubVoiceRegistry{voice: voice}
	step := NewNotificationStep(reg, &stubOracleFactory{}, time.Second, testLogger(t))
	state := notifierState("notifier", "whatsapp", "template")
	state.Event.Knowledge["name"] = "Bob"

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("send failure must be non-fatal, got: %v", err)
	}
	if state.ResponseDelivered {
		t.Error("expected ResponseDelivered=false on send failure")
	}
}

func TestNotificationStep_NameAndTimeout(t *testing.T) {
	step := NewNotificationStep(nil, nil, 3*time.Second, testLogger(t))
	if step.Name() != "Notification" {
		t.Errorf("expected Notification, got %s", step.Name())
	}
	if step.Timeout() != 3*time.Second {
		t.Errorf("expected 3s, got %v", step.Timeout())
	}
}

// --- renderTemplate ---

func TestRenderTemplate_SubstitutesKnowledge(t *testing.T) {
	knowledge := map[string]any{
		"name":  "Alice",
		"order": "ORD-123",
	}
	result := renderTemplate("Hello {{name}}, your order {{order}} is ready", knowledge)
	if result != "Hello Alice, your order ORD-123 is ready" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRenderTemplate_NoPlaceholders(t *testing.T) {
	result := renderTemplate("Hello World", map[string]any{})
	if result != "Hello World" {
		t.Errorf("unexpected result: %q", result)
	}
}

// --- toString ---

func TestToString_Nil(t *testing.T) {
	if got := toString(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestToString_String(t *testing.T) {
	if got := toString("hello"); got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestToString_Float64_Integer(t *testing.T) {
	if got := toString(float64(42)); got != "42" {
		t.Errorf("expected 42, got %q", got)
	}
}

func TestToString_Float64_NonInteger(t *testing.T) {
	if got := toString(float64(3.14)); got != "3.14" {
		t.Errorf("expected 3.14, got %q", got)
	}
}

func TestToString_OtherType(t *testing.T) {
	if got := toString(true); got != "true" {
		t.Errorf("expected true, got %q", got)
	}
}
