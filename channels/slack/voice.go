package slack

import (
	"context"
	"fmt"

	eywa "github.com/wmulabs/eywa"
)

// Voice sends the Spirit's reply back to the originating Slack channel via chat.postMessage.
type Voice struct {
	client *Client
}

func NewVoice(client *Client) eywa.Voice { return &Voice{client: client} }

func (v *Voice) GetName() string { return "slack" }

func (v *Voice) ShouldAutoRespond() bool { return true }

func (v *Voice) SendResponse(ctx context.Context, event *eywa.Pulse, response string) error {
	if response == "" {
		return fmt.Errorf("slack: cannot send empty response")
	}
	channel, ok := slackChannel(event)
	if !ok {
		return fmt.Errorf("slack: no slack_channel in event metadata")
	}
	if err := v.client.PostMessage(ctx, channel, response); err != nil {
		return fmt.Errorf("slack: send response: %w", err)
	}
	return nil
}

func (v *Voice) GetChannelMetadata(event *eywa.Pulse) map[string]any {
	return map[string]any{"channel": "slack", "source": event.Source}
}

func slackChannel(event *eywa.Pulse) (string, bool) {
	if event == nil || event.Metadata == nil {
		return "", false
	}
	ch, ok := event.Metadata["slack_channel"].(string)
	if !ok || ch == "" {
		return "", false
	}
	return ch, true
}
