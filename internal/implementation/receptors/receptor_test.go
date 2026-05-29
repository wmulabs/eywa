package receptors

import (
	"context"
	"testing"
)

func TestAPIDefaultReceptor_GetName(t *testing.T) {
	r := NewAPIDefaultReceptor()
	if r.GetName() != "api_default" {
		t.Errorf("expected api_default, got %s", r.GetName())
	}
}

func TestAPIDefaultReceptor_Convert_MissingMessage(t *testing.T) {
	r := NewAPIDefaultReceptor()
	_, err := r.Convert(context.Background(), "chat", map[string]any{
		"memory_key": "user:1",
	})
	if err == nil {
		t.Fatal("expected error for missing message")
	}
}

func TestAPIDefaultReceptor_Convert_EmptyMessage(t *testing.T) {
	r := NewAPIDefaultReceptor()
	_, err := r.Convert(context.Background(), "chat", map[string]any{
		"memory_key": "user:1",
		"message":    "",
	})
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}

func TestAPIDefaultReceptor_Convert_NonStringMessage(t *testing.T) {
	r := NewAPIDefaultReceptor()
	_, err := r.Convert(context.Background(), "chat", map[string]any{
		"memory_key": "user:1",
		"message":    42,
	})
	if err == nil {
		t.Fatal("expected error for non-string message")
	}
}

func TestAPIDefaultReceptor_Convert_MinimalFields(t *testing.T) {
	r := NewAPIDefaultReceptor()
	pulses, err := r.Convert(context.Background(), "chat", map[string]any{
		"memory_key": "user:1",
		"message":    "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) != 1 {
		t.Fatalf("expected 1 pulse, got %d", len(pulses))
	}
	p := pulses[0]
	if p.UserMessage != "hello" {
		t.Errorf("expected message=hello, got %q", p.UserMessage)
	}
	if p.Source != "api" {
		t.Errorf("expected source=api, got %q", p.Source)
	}
}

func TestAPIDefaultReceptor_Convert_AllOptionalFields(t *testing.T) {
	r := NewAPIDefaultReceptor()
	pulses, err := r.Convert(context.Background(), "order", map[string]any{
		"memory_key":      "user:42",
		"message":         "track my order",
		"source":          "whatsapp",
		"contact_phone":   "+5511999999999",
		"idempotency_key": "idem-abc",
		"context":         map[string]any{"order_id": "ORD-1"},
		"metadata":        map[string]any{"channel": "wa"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) != 1 {
		t.Fatalf("expected 1 pulse, got %d", len(pulses))
	}
	p := pulses[0]
	if p.UserMessage != "track my order" {
		t.Errorf("unexpected message: %q", p.UserMessage)
	}
	if p.Source != "whatsapp" {
		t.Errorf("unexpected source: %q", p.Source)
	}
	if p.ContactPhone != "+5511999999999" {
		t.Errorf("unexpected contact_phone: %q", p.ContactPhone)
	}
	if p.IdempotencyKey != "idem-abc" {
		t.Errorf("unexpected idempotency_key: %q", p.IdempotencyKey)
	}
	if p.Knowledge["order_id"] != "ORD-1" {
		t.Errorf("expected order_id in knowledge")
	}
	if p.Metadata["channel"] != "wa" {
		t.Errorf("expected channel in metadata")
	}
}

func TestAPIDefaultReceptor_Convert_MissingMemoryKey_UsesRandom(t *testing.T) {
	r := NewAPIDefaultReceptor()
	pulses, err := r.Convert(context.Background(), "chat", map[string]any{
		"message": "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) != 1 {
		t.Fatalf("expected 1 pulse, got %d", len(pulses))
	}
	if pulses[0].MemoryKey == "" {
		t.Error("expected non-empty memory key")
	}
}

func TestAPIDefaultReceptor_Convert_EventType(t *testing.T) {
	r := NewAPIDefaultReceptor()
	pulses, err := r.Convert(context.Background(), "order_status", map[string]any{
		"memory_key": "user:1",
		"message":    "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pulses[0].EventType != "order_status" {
		t.Errorf("expected event_type=order_status, got %q", pulses[0].EventType)
	}
}
