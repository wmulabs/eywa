package telegram

import (
	"context"
	"fmt"
	"strconv"

	eywa "github.com/wmulabs/eywa"
)

// Inbound converts a Telegram Bot API webhook Update into a Pulse. v1 handles text messages only;
// edits, channel posts, and non-text content are skipped. Bot-authored messages are ignored so the
// agent never answers itself.
type Inbound struct{}

func NewInbound() eywa.Receptor { return &Inbound{} }

func (i *Inbound) GetName() string { return "telegram" }

func (i *Inbound) Convert(_ context.Context, eventType string, raw map[string]any) ([]*eywa.Pulse, error) {
	msg, ok := eywa.GetMap(raw, "message")
	if !ok {
		return nil, fmt.Errorf("telegram: no 'message' in update")
	}

	if from, ok := eywa.GetMap(msg, "from"); ok {
		if isBot, _ := from["is_bot"].(bool); isBot {
			return nil, fmt.Errorf("telegram: ignoring bot-authored message")
		}
	}

	chat, ok := eywa.GetMap(msg, "chat")
	if !ok {
		return nil, fmt.Errorf("telegram: no 'chat' in message")
	}
	chatID := int64(eywa.GetFloat64OrDefault(chat, "id", 0))
	if chatID == 0 {
		return nil, fmt.Errorf("telegram: missing chat id")
	}

	text := eywa.GetStringOrDefault(msg, "text", "")
	if text == "" {
		return nil, fmt.Errorf("telegram: unsupported non-text message")
	}

	updateID := int64(eywa.GetFloat64OrDefault(raw, "update_id", 0))

	builder := eywa.NewPulse(eywa.MemoryKey{Channel: "telegram", User: strconv.FormatInt(chatID, 10)}).
		WithUserMessage(text).
		WithSource("telegram").
		WithEventType(eventType).
		WithPayload(raw).
		WithIdempotencyKey(fmt.Sprintf("telegram:%d", updateID)).
		AddMetadata("channel", "telegram").
		AddMetadata("telegram_chat_id", chatID).
		AddMetadata("telegram_update_id", updateID)

	if from, ok := eywa.GetMap(msg, "from"); ok {
		name := eywa.GetStringOrDefault(from, "username", eywa.GetStringOrDefault(from, "first_name", ""))
		builder = builder.AddMetadata("sender_name", name)
	}

	return []*eywa.Pulse{builder.Build()}, nil
}
