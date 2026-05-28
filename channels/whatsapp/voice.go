package whatsapp

import (
	"context"
	"fmt"

	eywa "github.com/wmulabs/eywa"
)

// WhatsAppResponseChannel sends the Spirit's reply via WhatsApp when no send_whatsapp_message
// Action was explicitly called during processing.
type WhatsAppResponseChannel struct {
	client eywa.WhatsAppClient
}

func NewWhatsAppResponseChannel(client eywa.WhatsAppClient) eywa.Voice {
	return &WhatsAppResponseChannel{
		client: client,
	}
}

func (c *WhatsAppResponseChannel) GetName() string {
	return "whatsapp"
}

func (c *WhatsAppResponseChannel) ShouldAutoRespond() bool {
	return true
}

func (c *WhatsAppResponseChannel) SendResponse(ctx context.Context, event *eywa.Pulse, response string) error {
	if response == "" {
		return fmt.Errorf("cannot send empty response")
	}

	phone := event.ContactPhone
	if phone == "" {
		return fmt.Errorf("no phone number found in event.ContactPhone")
	}

	normalizedPhone, err := eywa.NormalizePhone(phone, "")
	if err != nil {
		return fmt.Errorf("invalid phone number %s: %w", phone, err)
	}

	if err := c.client.SendMessage(ctx, normalizedPhone, response, nil); err != nil {
		return fmt.Errorf("failed to send WhatsApp response: %w", err)
	}

	return nil
}

func (c *WhatsAppResponseChannel) GetChannelMetadata(event *eywa.Pulse) map[string]any {
	return map[string]any{
		"channel":      "whatsapp",
		"phone_number": event.ContactPhone,
		"source":       event.Source,
	}
}
