package telegram

import (
	"context"
	"testing"
)

func textUpdate() map[string]any {
	return map[string]any{
		"update_id": float64(42),
		"message": map[string]any{
			"message_id": float64(7),
			"from":       map[string]any{"id": float64(99), "is_bot": false, "username": "alice"},
			"chat":       map[string]any{"id": float64(12345), "type": "private"},
			"text":       "hello bot",
		},
	}
}

func TestInbound_Convert_TextMessage(t *testing.T) {
	pulses, err := NewInbound().Convert(context.Background(), "user_message", textUpdate())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) != 1 {
		t.Fatalf("expected 1 pulse, got %d", len(pulses))
	}
	p := pulses[0]
	if p.UserMessage != "hello bot" {
		t.Errorf("UserMessage = %q", p.UserMessage)
	}
	if p.Source != "telegram" {
		t.Errorf("Source = %q", p.Source)
	}
	if got, _ := p.Metadata["telegram_chat_id"].(int64); got != 12345 {
		t.Errorf("telegram_chat_id = %v", p.Metadata["telegram_chat_id"])
	}
	if p.IdempotencyKey != "telegram:42" {
		t.Errorf("IdempotencyKey = %q", p.IdempotencyKey)
	}
	if p.Metadata["sender_name"] != "alice" {
		t.Errorf("sender_name = %v", p.Metadata["sender_name"])
	}
}

func TestInbound_Convert_Rejections(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
	}{
		{"no message", map[string]any{"update_id": float64(1)}},
		{"bot author", map[string]any{"message": map[string]any{
			"from": map[string]any{"is_bot": true},
			"chat": map[string]any{"id": float64(1)},
			"text": "hi",
		}}},
		{"no chat", map[string]any{"message": map[string]any{"text": "hi"}}},
		{"no text", map[string]any{"message": map[string]any{"chat": map[string]any{"id": float64(1)}}}},
		{"zero chat id", map[string]any{"message": map[string]any{"chat": map[string]any{"id": float64(0)}, "text": "hi"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewInbound().Convert(context.Background(), "user_message", c.raw); err == nil {
				t.Errorf("expected rejection for %s", c.name)
			}
		})
	}
}

func TestInbound_GetName(t *testing.T) {
	if NewInbound().GetName() != "telegram" {
		t.Error("name must be telegram")
	}
}
