package whatsapp

import (
	"context"
	"fmt"
	"strings"

	eywa "github.com/wmulabs/eywa"
)

type SendWhatsAppMessageTool struct {
	client     eywa.WhatsAppClient
	isCritical bool
}

func NewSendWhatsAppMessageTool(client eywa.WhatsAppClient) eywa.Action {
	return &SendWhatsAppMessageTool{
		client:     client,
		isCritical: true,
	}
}

func NewSendWhatsAppMessageToolWithCriticality(client eywa.WhatsAppClient, isCritical bool) eywa.Action {
	return &SendWhatsAppMessageTool{
		client:     client,
		isCritical: isCritical,
	}
}

func (t *SendWhatsAppMessageTool) GetName() string {
	return "send_whatsapp_message"
}

func (t *SendWhatsAppMessageTool) GetDescription() string {
	return "Send a WhatsApp message to a phone number. Supports text messages, interactive buttons (up to 3), and image attachments. Use this when you need to communicate with a customer via WhatsApp."
}

func (t *SendWhatsAppMessageTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"phone": map[string]any{
				"type":        "string",
				"description": "Phone number with country code (e.g., +5511999999999 or 5511999999999)",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "Message content to send",
			},
			"buttons": map[string]any{
				"type":        "array",
				"description": "Optional interactive buttons",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "Button identifier",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "Button text",
						},
					},
				},
			},
			"image_url": map[string]any{
				"type":        "string",
				"description": "Optional image URL to include in the message",
			},
		},
		"required": []string{"phone", "message"},
	}
}

func (t *SendWhatsAppMessageTool) Validate(args map[string]any) error {
	phone, ok := args["phone"].(string)
	if !ok || phone == "" {
		return eywa.NewBusinessError("missing or invalid 'phone' argument")
	}

	message, ok := args["message"].(string)
	if !ok || message == "" {
		return eywa.NewBusinessError("missing or invalid 'message' argument")
	}

	if buttons, ok := args["buttons"].([]any); ok {
		if len(buttons) > 3 {
			return eywa.NewBusinessError("maximum 3 buttons allowed")
		}
		for i, btn := range buttons {
			btnMap, ok := btn.(map[string]any)
			if !ok {
				return eywa.NewBusinessError(fmt.Sprintf("button %d is not an object", i))
			}
			if title, hasTitle := btnMap["title"].(string); !hasTitle || title == "" {
				return eywa.NewBusinessError(fmt.Sprintf("button %d missing or empty 'title'", i))
			}
		}
	}

	if imageURL, ok := args["image_url"].(string); ok && imageURL != "" {
		if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
			return eywa.NewBusinessError("image_url must be a valid HTTP(S) URL")
		}
	}

	return nil
}

func (t *SendWhatsAppMessageTool) IsCritical() bool {
	return t.isCritical
}

func (t *SendWhatsAppMessageTool) GetCategory() eywa.ActionCategory {
	return eywa.ActionDelivery
}

func (t *SendWhatsAppMessageTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	rawPhone := args["phone"].(string)
	message := args["message"].(string)

	phone, err := eywa.NormalizePhone(rawPhone, "")
	if err != nil {
		eywa.GetLogger().Warnw("phone normalization failed, using original", "raw_phone", rawPhone, "error", err)
		phone = rawPhone
	} else if phone != rawPhone {
		eywa.GetLogger().Infow("phone number normalized", "raw_phone", rawPhone, "normalized_phone", phone)
	}

	metadata := make(map[string]any)
	if buttons, ok := args["buttons"].([]any); ok {
		metadata["buttons"] = buttons
	}
	if imageURL, ok := args["image_url"].(string); ok {
		metadata["image_url"] = imageURL
	}

	if err := t.client.SendMessage(ctx, phone, message, metadata); err != nil {
		return "", fmt.Errorf("failed to send WhatsApp message to %s: %w", eywa.ObfuscatePhone(phone), err)
	}

	return fmt.Sprintf("Message sent successfully to %s", phone), nil
}
