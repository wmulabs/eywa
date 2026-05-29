package voices

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// HTTPVoice is a no-op Voice for HTTP webhook flows.
// ShouldAutoRespond returns false because the response travels in the HTTP body — no async send needed.
type HTTPVoice struct{}

func NewHTTPVoice() ports.Voice {
	return &HTTPVoice{}
}

func (c *HTTPVoice) GetName() string {
	return "http"
}

func (c *HTTPVoice) ShouldAutoRespond() bool {
	return false
}

func (c *HTTPVoice) SendResponse(_ context.Context, _ *entities.Pulse, _ string) error {
	return nil
}

func (c *HTTPVoice) GetChannelMetadata(event *entities.Pulse) map[string]any {
	return map[string]any{
		"channel": "http",
		"source":  event.Source,
	}
}
