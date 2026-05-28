package twilio

import (
	"context"
	"fmt"
	"strings"

	eywa "github.com/wmulabs/eywa"
)

// WhatsAppTwilioInbound converts Twilio WhatsApp webhook payloads to Pulses.
// Downloads attachment bytes via WhatsAppClient (Twilio media URLs require Basic Auth).
// Uses MessageSid as IdempotencyKey for deduplication.
type WhatsAppTwilioInbound struct {
	whatsappClient eywa.WhatsAppClient
}

func NewWhatsAppTwilioInbound(client eywa.WhatsAppClient) eywa.Receptor {
	return &WhatsAppTwilioInbound{whatsappClient: client}
}

func (c *WhatsAppTwilioInbound) GetName() string {
	return "whatsapp_twilio"
}

func (c *WhatsAppTwilioInbound) Convert(ctx context.Context, eventType string, raw map[string]any) ([]*eywa.Pulse, error) {
	messageSid := eywa.GetStringOrDefault(raw, "MessageSid", "")
	if messageSid == "" {
		return nil, fmt.Errorf("missing required field 'MessageSid' in webhook payload")
	}

	from := eywa.GetStringOrDefault(raw, "From", "")
	if from == "" {
		return nil, fmt.Errorf("missing required field 'From' in webhook payload")
	}

	phoneNumber := c.extractPhoneNumber(from)
	if phoneNumber == "" {
		return nil, fmt.Errorf("invalid 'From' format, expected 'whatsapp:+1234567890', got '%s'", from)
	}

	meta := c.extractWebhookMetadata(raw)
	messageData := c.processMessageContent(ctx, raw)
	event := c.buildEvent(eventType, phoneNumber, messageData, raw, meta)

	return []*eywa.Pulse{event}, nil
}

type twilioMessageData struct {
	textParts   []string
	attachments []*eywa.Artifact
}

type twilioWebhookMetadata struct {
	accountSid          string
	messageSid          string
	smsSid              string
	to                  string
	profileName         string
	waID                string
	buttonPayload       string
	buttonText          string
	buttonType          string
	forwarded           string
	frequentlyForwarded string
}

func (c *WhatsAppTwilioInbound) extractWebhookMetadata(raw map[string]any) twilioWebhookMetadata {
	return twilioWebhookMetadata{
		accountSid:          eywa.GetStringOrDefault(raw, "AccountSid", ""),
		messageSid:          eywa.GetStringOrDefault(raw, "MessageSid", ""),
		smsSid:              eywa.GetStringOrDefault(raw, "SmsSid", ""),
		to:                  eywa.GetStringOrDefault(raw, "To", ""),
		profileName:         eywa.GetStringOrDefault(raw, "ProfileName", ""),
		waID:                eywa.GetStringOrDefault(raw, "WaId", ""),
		buttonPayload:       eywa.GetStringOrDefault(raw, "ButtonPayload", ""),
		buttonText:          eywa.GetStringOrDefault(raw, "ButtonText", ""),
		buttonType:          eywa.GetStringOrDefault(raw, "ButtonType", ""),
		forwarded:           eywa.GetStringOrDefault(raw, "Forwarded", ""),
		frequentlyForwarded: eywa.GetStringOrDefault(raw, "FrequentlyForwarded", ""),
	}
}

func (c *WhatsAppTwilioInbound) processMessageContent(ctx context.Context, raw map[string]any) twilioMessageData {
	var data twilioMessageData
	data.textParts = make([]string, 0)
	data.attachments = make([]*eywa.Artifact, 0)

	c.processTextMessage(raw, &data)
	c.processButtonInteraction(raw, &data)
	c.processMediaAttachments(ctx, raw, &data)
	c.processLocation(raw, &data)

	return data
}

func (c *WhatsAppTwilioInbound) processTextMessage(raw map[string]any, data *twilioMessageData) {
	body := eywa.GetStringOrDefault(raw, "Body", "")
	if body != "" {
		data.textParts = append(data.textParts, body)
	}
}

func (c *WhatsAppTwilioInbound) processButtonInteraction(raw map[string]any, data *twilioMessageData) {
	buttonText := eywa.GetStringOrDefault(raw, "ButtonText", "")
	if buttonText == "" {
		return
	}

	if len(data.textParts) > 0 && data.textParts[0] == buttonText {
		return
	}

	data.textParts = append(data.textParts, buttonText)
}

func (c *WhatsAppTwilioInbound) processMediaAttachments(ctx context.Context, raw map[string]any, data *twilioMessageData) {
	numMedia := eywa.GetIntOrDefault(raw, "NumMedia", 0)
	if numMedia == 0 {
		return
	}

	for i := 0; i < numMedia; i++ {
		att := c.extractMediaAttachment(ctx, raw, i)
		if att != nil {
			data.attachments = append(data.attachments, att)
		}
	}
}

func (c *WhatsAppTwilioInbound) extractMediaAttachment(ctx context.Context, raw map[string]any, index int) *eywa.Artifact {
	mediaURL := eywa.GetStringOrDefault(raw, fmt.Sprintf("MediaUrl%d", index), "")
	if mediaURL == "" {
		return nil
	}

	mimeType := eywa.GetStringOrDefault(raw, fmt.Sprintf("MediaContentType%d", index), "")
	attachmentType := classifyMimeType(mimeType)

	att := &eywa.Artifact{
		Type:     attachmentType,
		MediaID:  mediaURL,
		MimeType: mimeType,
	}

	// Twilio media URLs require Basic Auth — handled by TwilioClient.DownloadMedia.
	if c.whatsappClient != nil {
		data, responseMimeType, err := c.whatsappClient.DownloadMedia(ctx, mediaURL)
		if err != nil {
			eywa.GetLogger().Warnw("twilio: failed to download attachment",
				"media_url", mediaURL, "type", att.Type, "mime_type", att.MimeType, "error", err)
		} else {
			att.Data = data
			if responseMimeType != "" && att.MimeType == "" {
				att.MimeType = responseMimeType
			}
			eywa.GetLogger().Debugw("twilio: attachment downloaded",
				"media_url", mediaURL, "type", att.Type, "bytes", len(data))
		}
	}

	return att
}

func (c *WhatsAppTwilioInbound) processLocation(raw map[string]any, data *twilioMessageData) {
	lat := eywa.GetFloat64OrDefault(raw, "Latitude", 0)
	lon := eywa.GetFloat64OrDefault(raw, "Longitude", 0)

	if lat == 0 && lon == 0 {
		return
	}

	address := eywa.GetStringOrDefault(raw, "Address", "")
	label := eywa.GetStringOrDefault(raw, "Label", "")
	data.textParts = append(data.textParts, formatLocationText(label, address, lat, lon))
}

func formatLocationText(label, address string, lat, lon float64) string {
	if label != "" && address != "" {
		return fmt.Sprintf("[Location: %s, %s (%.6f, %.6f)]", label, address, lat, lon)
	}
	if address != "" {
		return fmt.Sprintf("[Location: %s (%.6f, %.6f)]", address, lat, lon)
	}
	if label != "" {
		return fmt.Sprintf("[Location: %s (%.6f, %.6f)]", label, lat, lon)
	}
	return fmt.Sprintf("[Location: (%.6f, %.6f)]", lat, lon)
}

func classifyMimeType(mimeType string) eywa.ArtifactType {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return eywa.ArtifactTypeImage
	case strings.HasPrefix(mimeType, "audio/"):
		return eywa.ArtifactTypeAudio
	case strings.HasPrefix(mimeType, "video/"):
		return eywa.ArtifactTypeVideo
	default:
		return eywa.ArtifactTypeDocument
	}
}

func (c *WhatsAppTwilioInbound) extractPhoneNumber(from string) string {
	phone := strings.TrimPrefix(from, "whatsapp:")

	normalized, err := eywa.NormalizePhone(phone, "")
	if err != nil {
		return phone
	}

	return normalized
}

func (c *WhatsAppTwilioInbound) buildEvent(
	eventType, phoneNumber string,
	data twilioMessageData,
	rawPayload map[string]any,
	meta twilioWebhookMetadata,
) *eywa.Pulse {
	combinedText := eywa.CombineTextPartsMultiLine(data.textParts)
	originalPhone := strings.TrimPrefix(eywa.GetStringOrDefault(rawPayload, "From", ""), "whatsapp:")

	builder := eywa.NewPulse(eywa.MemoryKey{
		Channel: "whatsapp",
		User:    phoneNumber,
	}).
		WithUserMessage(combinedText).
		WithSource("whatsapp_twilio").
		WithContactPhone(phoneNumber).
		WithEventType(eventType).
		WithPayload(rawPayload).
		WithIdempotencyKey(meta.messageSid).
		AddMetadata("channel", "whatsapp").
		AddMetadata("provider", "twilio").
		AddMetadata("sender_phone", phoneNumber).
		AddMetadata("sender_phone_raw", originalPhone).
		AddMetadata("sender_name", meta.profileName).
		AddMetadata("sender_wa_id", meta.waID).
		AddMetadata("business_phone", c.extractPhoneNumber(meta.to)).
		AddMetadata("business_twilio_number", meta.to).
		AddMetadata("account_sid", meta.accountSid).
		AddMetadata("message_sid", meta.messageSid).
		AddMetadata("sms_sid", meta.smsSid)

	if meta.buttonPayload != "" {
		builder = builder.
			AddMetadata("button_payload", meta.buttonPayload).
			AddMetadata("button_text", meta.buttonText)
		if meta.buttonType != "" {
			builder = builder.AddMetadata("button_type", meta.buttonType)
		}
	}

	if meta.forwarded != "" {
		builder = builder.AddMetadata("forwarded", meta.forwarded)
	}
	if meta.frequentlyForwarded != "" {
		builder = builder.AddMetadata("frequently_forwarded", meta.frequentlyForwarded)
	}

	if len(data.attachments) > 0 {
		builder = builder.WithAttachments(data.attachments)

		downloaded := 0
		for _, att := range data.attachments {
			if len(att.Data) > 0 {
				downloaded++
			}
		}
		builder = builder.
			AddMetadata("attachment_count", len(data.attachments)).
			AddMetadata("attachment_downloaded_count", downloaded)
	}

	return builder.Build()
}
