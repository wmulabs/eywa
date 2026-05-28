package ports

import "context"

// TemplateParameter represents a single parameter value in a template component.
// WhatsApp templates use ordered parameters (1, 2, 3...) rather than named variables.
type TemplateParameter struct {
	Type  string `json:"type"`            // "text", "currency", "date_time", "image", "video", "document"
	Text  string `json:"text,omitempty"`  // For text type
	Image string `json:"image,omitempty"` // Image URL for image type
	Video string `json:"video,omitempty"` // Video URL for video type
}

// TemplateComponent represents a component in a WhatsApp template (header, body, button).
// Each component can have multiple ordered parameters.
type TemplateComponent struct {
	Type       string              `json:"type"`                 // "header", "body", "button"
	SubType    string              `json:"sub_type,omitempty"`   // For buttons: "quick_reply", "url"
	Index      int                 `json:"index,omitempty"`      // For buttons: 0-based index
	Parameters []TemplateParameter `json:"parameters,omitempty"` // Ordered list of parameters
}

// TemplateMessage represents a complete WhatsApp template message.
// Templates must be pre-approved by Meta/WhatsApp Business API.
type TemplateMessage struct {
	Name       string              `json:"name"`       // Template name (e.g., "welcome_message")
	Language   string              `json:"language"`   // Language code (e.g., "pt_BR", "en_US")
	Components []TemplateComponent `json:"components"` // Template components with parameters
}

// WhatsAppClient defines the interface for WhatsApp messaging providers
// Implementations: Dialog360Client, TwilioClient
type WhatsAppClient interface {
	// SendMessage sends a freeform WhatsApp message (within 24h conversation window)
	// phone: recipient phone number
	// message: text message to send
	// metadata: optional metadata (buttons, image_url, etc)
	SendMessage(ctx context.Context, phone, message string, metadata map[string]any) error

	// SendTemplateMessage sends a pre-approved WhatsApp template message
	// Used for starting conversations or messaging outside the 24-hour window.
	// Templates must be pre-approved by Meta/WhatsApp Business API.
	// phone: recipient phone number
	// template: template configuration with ordered parameters by component
	SendTemplateMessage(ctx context.Context, phone string, template *TemplateMessage) error

	// DownloadMedia downloads media file from WhatsApp provider
	// mediaID: provider-specific media identifier
	// Returns: media data bytes, MIME type, and error
	DownloadMedia(ctx context.Context, mediaID string) (data []byte, mimeType string, err error)
}
