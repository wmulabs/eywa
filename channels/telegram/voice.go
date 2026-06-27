package telegram

import (
	"context"
	"fmt"

	eywa "github.com/wmulabs/eywa"
)

// Voice sends the Spirit's reply back to the originating Telegram chat.
type Voice struct {
	client *Client
}

func NewVoice(client *Client) eywa.Voice { return &Voice{client: client} }

func (v *Voice) GetName() string { return "telegram" }

func (v *Voice) ShouldAutoRespond() bool { return true }

func (v *Voice) SendResponse(ctx context.Context, event *eywa.Pulse, response string) error {
	if response == "" {
		return fmt.Errorf("telegram: cannot send empty response")
	}
	chatID, ok := chatIDFromEvent(event)
	if !ok {
		return fmt.Errorf("telegram: no telegram_chat_id in event metadata")
	}
	if err := v.client.SendMessage(ctx, chatID, response); err != nil {
		return fmt.Errorf("telegram: send response: %w", err)
	}
	return nil
}

func (v *Voice) GetChannelMetadata(event *eywa.Pulse) map[string]any {
	return map[string]any{"channel": "telegram", "source": event.Source}
}

// chatIDFromEvent reads the chat id stored at ingest. It tolerates float64/int after a JSON round-trip
// (e.g. when the Pulse was persisted and reloaded).
func chatIDFromEvent(event *eywa.Pulse) (int64, bool) {
	if event == nil || event.Metadata == nil {
		return 0, false
	}
	switch v := event.Metadata["telegram_chat_id"].(type) {
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}
