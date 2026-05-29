package whatsapp

import (
	"context"
	"fmt"
	"strings"

	eywa "github.com/wmulabs/eywa"
)

type SendWhatsAppTemplateTool struct {
	client     eywa.WhatsAppClient
	isCritical bool
}

func NewSendWhatsAppTemplateTool(client eywa.WhatsAppClient) eywa.Action {
	return &SendWhatsAppTemplateTool{
		client:     client,
		isCritical: true,
	}
}

func NewSendWhatsAppTemplateToolWithCriticality(client eywa.WhatsAppClient, isCritical bool) eywa.Action {
	return &SendWhatsAppTemplateTool{
		client:     client,
		isCritical: isCritical,
	}
}

func (t *SendWhatsAppTemplateTool) GetName() string {
	return "send_whatsapp_template"
}

func (t *SendWhatsAppTemplateTool) GetDescription() string {
	return "Send a pre-approved WhatsApp template message to start a conversation or message outside the 24-hour window. Templates must be approved by Meta/WhatsApp Business API. Use this for initial notifications to drivers or customers who haven't messaged you recently. Parameters are provided as ordered arrays per component (header, body, buttons)."
}

func (t *SendWhatsAppTemplateTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"phone": map[string]any{
				"type":        "string",
				"description": "Phone number with country code (e.g., +5211234567890)",
			},
			"template_name": map[string]any{
				"type":        "string",
				"description": "Name of the approved template (e.g., 'welcome_message', 'route_notification')",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Template language code (e.g., 'pt_BR', 'en_US', 'es_MX')",
				"default":     "pt_BR",
			},
			"header_params": map[string]any{
				"type":        "array",
				"description": "Ordered array of header parameters (e.g., ['João', 'image_url']). Leave empty if template has no header or header has no variables.",
				"items": map[string]any{
					"type": "string",
				},
			},
			"body_params": map[string]any{
				"type":        "array",
				"description": "Ordered array of body parameters (e.g., ['123456', '15:30', 'Rua ABC']). Parameters replace {{1}}, {{2}}, {{3}} in order.",
				"items": map[string]any{
					"type": "string",
				},
			},
			"button_params": map[string]any{
				"type":        "array",
				"description": "Ordered array of button parameters for dynamic buttons (e.g., ['payload_value']). Leave empty if no dynamic buttons.",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []string{"phone", "template_name", "language"},
	}
}

func (t *SendWhatsAppTemplateTool) Validate(args map[string]any) error {
	phone, ok := args["phone"].(string)
	if !ok || phone == "" {
		return eywa.NewBusinessError("missing or invalid 'phone' argument")
	}

	templateName, ok := args["template_name"].(string)
	if !ok || templateName == "" {
		return eywa.NewBusinessError("missing or invalid 'template_name' argument")
	}

	language, ok := args["language"].(string)
	if !ok || language == "" {
		return eywa.NewBusinessError("missing or invalid 'language' argument")
	}

	if headerParams, ok := args["header_params"]; ok && headerParams != nil {
		if _, ok := headerParams.([]any); !ok {
			return eywa.NewBusinessError("'header_params' must be an array")
		}
	}

	if bodyParams, ok := args["body_params"]; ok && bodyParams != nil {
		if _, ok := bodyParams.([]any); !ok {
			return eywa.NewBusinessError("'body_params' must be an array")
		}
	}

	if buttonParams, ok := args["button_params"]; ok && buttonParams != nil {
		if _, ok := buttonParams.([]any); !ok {
			return eywa.NewBusinessError("'button_params' must be an array")
		}
	}

	return nil
}

func (t *SendWhatsAppTemplateTool) IsCritical() bool {
	return t.isCritical
}

func (t *SendWhatsAppTemplateTool) GetCategory() eywa.ActionCategory {
	return eywa.ActionDelivery
}

func (t *SendWhatsAppTemplateTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	rawPhone := args["phone"].(string)
	templateName := args["template_name"].(string)
	language := args["language"].(string)

	phone, err := eywa.NormalizePhone(rawPhone, "")
	if err != nil {
		eywa.GetLogger().Warnw("phone normalization failed, using original", "raw_phone", rawPhone, "error", err)
		phone = rawPhone
	} else if phone != rawPhone {
		eywa.GetLogger().Infow("phone number normalized", "raw_phone", rawPhone, "normalized_phone", phone)
	}

	template := &eywa.TemplateMessage{
		Name:       templateName,
		Language:   language,
		Components: []eywa.TemplateComponent{},
	}

	if headerParams, ok := args["header_params"].([]any); ok && len(headerParams) > 0 {
		component := eywa.TemplateComponent{
			Type:       "header",
			Parameters: t.convertToParameters(headerParams),
		}
		template.Components = append(template.Components, component)
	}

	if bodyParams, ok := args["body_params"].([]any); ok && len(bodyParams) > 0 {
		component := eywa.TemplateComponent{
			Type:       "body",
			Parameters: t.convertToParameters(bodyParams),
		}
		template.Components = append(template.Components, component)
	}

	if buttonParams, ok := args["button_params"].([]any); ok && len(buttonParams) > 0 {
		for i, param := range buttonParams {
			component := eywa.TemplateComponent{
				Type:    "button",
				SubType: "quick_reply",
				Index:   i,
				Parameters: []eywa.TemplateParameter{
					{
						Type: "payload",
						Text: fmt.Sprintf("%v", param),
					},
				},
			}
			template.Components = append(template.Components, component)
		}
	}

	if err := t.client.SendTemplateMessage(ctx, phone, template); err != nil {
		return "", fmt.Errorf("failed to send WhatsApp template to %s: %w", eywa.ObfuscatePhone(phone), err)
	}

	return fmt.Sprintf("Template '%s' sent successfully to %s", templateName, phone), nil
}

// convertToParameters detects whether each param is an image URL, video URL, or plain text.
func (t *SendWhatsAppTemplateTool) convertToParameters(params []any) []eywa.TemplateParameter {
	result := make([]eywa.TemplateParameter, 0, len(params))

	for _, param := range params {
		strParam := fmt.Sprintf("%v", param)

		if isImageURL(strParam) {
			result = append(result, eywa.TemplateParameter{
				Type:  "image",
				Image: strParam,
			})
		} else if isVideoURL(strParam) {
			result = append(result, eywa.TemplateParameter{
				Type:  "video",
				Video: strParam,
			})
		} else {
			result = append(result, eywa.TemplateParameter{
				Type: "text",
				Text: strParam,
			})
		}
	}

	return result
}

func isImageURL(s string) bool {
	lower := strings.ToLower(s)
	return (strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")) &&
		(strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".gif"))
}

func isVideoURL(s string) bool {
	lower := strings.ToLower(s)
	return (strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")) &&
		(strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".mov") || strings.HasSuffix(lower, ".avi"))
}
