package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

func filterState(pulse *entities.Pulse, link *entities.Link) *ProcessingState {
	return &ProcessingState{Event: pulse, EventConfig: link}
}

func TestFilterStep_NoRules_Passes(t *testing.T) {
	step := NewFilterStep(5*time.Second, testLogger(t))
	pulse := &entities.Pulse{
		MemoryKey: "session:123",
		Source:    "bot",
	}
	link := entities.NewLink("test_event").Build()
	state := filterState(pulse, link)

	err := step.Execute(context.Background(), state)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestFilterStep_BlockList_Blocks(t *testing.T) {
	step := NewFilterStep(5*time.Second, testLogger(t))
	pulse := &entities.Pulse{
		MemoryKey: "session:123",
		Source:    "bot",
	}
	link := entities.NewLink("test_event").
		WithGuards(entities.Guard{
			Field:     "source",
			BlockList: []string{"bot", "internal"},
		}).
		Build()
	state := filterState(pulse, link)

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Error("expected error for blocked value, got nil")
	}
	if !IsFilterBlocked(err) {
		t.Errorf("expected IsFilterBlocked=true, got false: %v", err)
	}
}

func TestFilterStep_BlockList_AllowsOther(t *testing.T) {
	step := NewFilterStep(5*time.Second, testLogger(t))
	pulse := &entities.Pulse{
		MemoryKey: "session:123",
		Source:    "api",
	}
	link := entities.NewLink("test_event").
		WithGuards(entities.Guard{
			Field:     "source",
			BlockList: []string{"bot", "internal"},
		}).
		Build()
	state := filterState(pulse, link)

	err := step.Execute(context.Background(), state)
	if err != nil {
		t.Errorf("expected no error for non-blocked value, got %v", err)
	}
}

func TestFilterStep_AllowList_Blocks(t *testing.T) {
	step := NewFilterStep(5*time.Second, testLogger(t))
	pulse := &entities.Pulse{
		MemoryKey: "session:123",
		Source:    "unknown",
	}
	link := entities.NewLink("test_event").
		WithGuards(entities.Guard{
			Field:     "source",
			AllowList: []string{"api", "web"},
		}).
		Build()
	state := filterState(pulse, link)

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Error("expected error for value not in allowlist, got nil")
	}
	if !IsFilterNotAllowed(err) {
		t.Errorf("expected IsFilterNotAllowed=true, got false: %v", err)
	}
}

func TestFilterStep_AllowList_Allows(t *testing.T) {
	step := NewFilterStep(5*time.Second, testLogger(t))
	pulse := &entities.Pulse{
		MemoryKey: "session:123",
		Source:    "api",
	}
	link := entities.NewLink("test_event").
		WithGuards(entities.Guard{
			Field:     "source",
			AllowList: []string{"api", "web"},
		}).
		Build()
	state := filterState(pulse, link)

	err := step.Execute(context.Background(), state)
	if err != nil {
		t.Errorf("expected no error for allowed value, got %v", err)
	}
}

func TestFilterStep_MissingField_Skips(t *testing.T) {
	step := NewFilterStep(5*time.Second, testLogger(t))
	pulse := &entities.Pulse{
		MemoryKey: "session:123",
	}
	link := entities.NewLink("test_event").
		WithGuards(entities.Guard{
			Field:     "source",
			BlockList: []string{"bot"},
		}).
		Build()
	state := filterState(pulse, link)

	err := step.Execute(context.Background(), state)
	if err != nil {
		t.Errorf("expected no error when field is missing, got %v", err)
	}
}

func TestResolveEventField_Source(t *testing.T) {
	pulse := &entities.Pulse{Source: "api"}
	result := resolveEventField(pulse, "source")
	if result != "api" {
		t.Errorf("expected 'api', got '%s'", result)
	}
}

func TestResolveEventField_MemoryKey(t *testing.T) {
	pulse := &entities.Pulse{MemoryKey: "session:abc123"}
	result := resolveEventField(pulse, "memory_key")
	if result != "session:abc123" {
		t.Errorf("expected 'session:abc123', got '%s'", result)
	}
}

func TestResolveEventField_ContactPhone(t *testing.T) {
	pulse := &entities.Pulse{ContactPhone: "+5511999999999"}
	result := resolveEventField(pulse, "contact_phone")
	if result != "+5511999999999" {
		t.Errorf("expected '+5511999999999', got '%s'", result)
	}
}

func TestResolveEventField_SubType(t *testing.T) {
	pulse := &entities.Pulse{SubType: "image"}
	result := resolveEventField(pulse, "sub_type")
	if result != "image" {
		t.Errorf("expected 'image', got '%s'", result)
	}
}

func TestResolveEventField_EventType(t *testing.T) {
	pulse := &entities.Pulse{EventType: "whatsapp_message"}
	result := resolveEventField(pulse, "event_type")
	if result != "whatsapp_message" {
		t.Errorf("expected 'whatsapp_message', got '%s'", result)
	}
}

func TestResolveEventField_IdempotencyKey(t *testing.T) {
	pulse := &entities.Pulse{IdempotencyKey: "wamid:123"}
	result := resolveEventField(pulse, "idempotency_key")
	if result != "wamid:123" {
		t.Errorf("expected 'wamid:123', got '%s'", result)
	}
}

func TestResolveEventField_UserMessage(t *testing.T) {
	pulse := &entities.Pulse{UserMessage: "hello world"}
	result := resolveEventField(pulse, "user_message")
	if result != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", result)
	}
}

func TestResolveEventField_SubjectKey(t *testing.T) {
	pulse := &entities.Pulse{SubjectKey: "shipment:456"}
	result := resolveEventField(pulse, "subject_key")
	if result != "shipment:456" {
		t.Errorf("expected 'shipment:456', got '%s'", result)
	}
}

func TestResolveEventField_KnowledgeDot(t *testing.T) {
	pulse := &entities.Pulse{
		Knowledge: map[string]any{
			"tier": "premium",
		},
	}
	result := resolveEventField(pulse, "knowledge.tier")
	if result != "premium" {
		t.Errorf("expected 'premium', got '%s'", result)
	}
}

func TestResolveEventField_MetadataDot(t *testing.T) {
	pulse := &entities.Pulse{
		Metadata: map[string]any{
			"channel": "whatsapp",
		},
	}
	result := resolveEventField(pulse, "metadata.channel")
	if result != "whatsapp" {
		t.Errorf("expected 'whatsapp', got '%s'", result)
	}
}

func TestResolveEventField_PayloadDot(t *testing.T) {
	pulse := &entities.Pulse{
		Payload: map[string]any{
			"wamid": "wam123",
		},
	}
	result := resolveEventField(pulse, "payload.wamid")
	if result != "wam123" {
		t.Errorf("expected 'wam123', got '%s'", result)
	}
}

func TestResolveEventField_NestedMap(t *testing.T) {
	pulse := &entities.Pulse{
		Knowledge: map[string]any{
			"address": map[string]any{
				"city": "Sao Paulo",
			},
		},
	}
	result := resolveEventField(pulse, "knowledge.address.city")
	if result != "Sao Paulo" {
		t.Errorf("expected 'Sao Paulo', got '%s'", result)
	}
}

func TestResolveEventField_UnknownTopLevel(t *testing.T) {
	pulse := &entities.Pulse{
		MemoryKey: "session:123",
	}
	result := resolveEventField(pulse, "unknown_field")
	if result != "" {
		t.Errorf("expected empty string for unknown field, got '%s'", result)
	}
}
