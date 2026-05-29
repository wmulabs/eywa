package dialog360

import (
	"context"
	"fmt"

	eywa "github.com/wmulabs/eywa"
)

// WhatsApp360DialogInbound converts 360Dialog webhook payloads to Pulses.
// Downloads attachment bytes via WhatsAppClient so att.Data is fully populated before processing.
// Groups messages by sender to produce one Pulse per sender (reduces LLM calls on multi-sender webhooks).
// Uses wamid as IdempotencyKey for deduplication.
type WhatsApp360DialogInbound struct {
	whatsappClient eywa.WhatsAppClient
}

func NewWhatsApp360DialogInbound(client eywa.WhatsAppClient) eywa.Receptor {
	return &WhatsApp360DialogInbound{whatsappClient: client}
}

func (c *WhatsApp360DialogInbound) GetName() string {
	return "whatsapp_360dialog"
}

func (c *WhatsApp360DialogInbound) Convert(ctx context.Context, eventType string, raw map[string]any) ([]*eywa.Pulse, error) {
	entryArray, ok := eywa.GetSlice(raw, "entry")
	if !ok || len(entryArray) == 0 {
		return nil, fmt.Errorf("missing or empty 'entry' array in webhook payload")
	}

	entryMap, err := eywa.GetFirstMapFromSlice(entryArray)
	if err != nil {
		return nil, fmt.Errorf("invalid entry format: %w", err)
	}

	changesArray, ok := eywa.GetSlice(entryMap, "changes")
	if !ok || len(changesArray) == 0 {
		return nil, fmt.Errorf("missing or empty 'changes' array in entry")
	}

	changeMap, err := eywa.GetFirstMapFromSlice(changesArray)
	if err != nil {
		return nil, fmt.Errorf("invalid change format: %w", err)
	}

	value, ok := eywa.GetMap(changeMap, "value")
	if !ok {
		return nil, fmt.Errorf("missing 'value' object in change")
	}

	messagesArray, ok := eywa.GetSlice(value, "messages")
	if !ok || len(messagesArray) == 0 {
		return nil, fmt.Errorf("missing or empty 'messages' array in value")
	}

	webhookMetadata := c.extractWebhookMetadata(value)

	messagesBySender := make(map[string][]map[string]any)
	for _, msgInterface := range messagesArray {
		msgMap, ok := msgInterface.(map[string]any)
		if !ok {
			continue
		}

		from := eywa.GetStringOrDefault(msgMap, "from", "")
		if from == "" {
			continue
		}

		messagesBySender[from] = append(messagesBySender[from], msgMap)
	}

	if len(messagesBySender) == 0 {
		return nil, fmt.Errorf("no valid messages found in webhook")
	}

	return c.createEventsForSenders(ctx, eventType, messagesBySender, raw, webhookMetadata)
}

type dialog360WebhookMetadata struct {
	displayPhoneNumber string
	phoneNumberID      string
	contactName        string
}

func (c *WhatsApp360DialogInbound) extractWebhookMetadata(value map[string]any) dialog360WebhookMetadata {
	var meta dialog360WebhookMetadata

	if metadata, ok := eywa.GetMap(value, "metadata"); ok {
		meta.displayPhoneNumber = eywa.GetStringOrDefault(metadata, "display_phone_number", "")
		meta.phoneNumberID = eywa.GetStringOrDefault(metadata, "phone_number_id", "")
	}

	if contactsArray, ok := eywa.GetSlice(value, "contacts"); ok && len(contactsArray) > 0 {
		if contactMap, ok := contactsArray[0].(map[string]any); ok {
			if profile, ok := eywa.GetMap(contactMap, "profile"); ok {
				meta.contactName = eywa.GetStringOrDefault(profile, "name", "")
			}
		}
	}

	return meta
}

func (c *WhatsApp360DialogInbound) createEventsForSenders(
	ctx context.Context,
	eventType string,
	messagesBySender map[string][]map[string]any,
	rawPayload map[string]any,
	meta dialog360WebhookMetadata,
) ([]*eywa.Pulse, error) {
	events := make([]*eywa.Pulse, 0, len(messagesBySender))

	for sender, senderMessages := range messagesBySender {
		event, err := c.buildEventForSender(ctx, eventType, sender, senderMessages, rawPayload, meta)
		if err != nil {
			continue
		}
		events = append(events, event)
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("failed to create any valid events from webhook")
	}

	return events, nil
}

func (c *WhatsApp360DialogInbound) buildEventForSender(
	ctx context.Context,
	eventType, sender string,
	senderMessages []map[string]any,
	rawPayload map[string]any,
	meta dialog360WebhookMetadata,
) (*eywa.Pulse, error) {
	messageData := c.processMessages(ctx, senderMessages)

	if messageData.firstWamid == "" {
		return nil, fmt.Errorf("no valid message ID (wamid) found for sender %s", sender)
	}

	return c.buildEvent(eventType, sender, messageData, rawPayload, meta), nil
}

type dialog360MessageData struct {
	textParts      []string
	allWamids      []string
	firstWamid     string
	firstTimestamp string
	attachments    []*eywa.Artifact
}

func (c *WhatsApp360DialogInbound) processMessages(ctx context.Context, messages []map[string]any) dialog360MessageData {
	var data dialog360MessageData
	data.textParts = make([]string, 0)
	data.allWamids = make([]string, 0)
	data.attachments = make([]*eywa.Artifact, 0)

	for _, msg := range messages {
		msgType := eywa.GetStringOrDefault(msg, "type", "text")
		wamid := eywa.GetStringOrDefault(msg, "id", "")
		timestamp := eywa.GetStringOrDefault(msg, "timestamp", "")

		if wamid != "" {
			data.allWamids = append(data.allWamids, wamid)
			if data.firstWamid == "" {
				data.firstWamid = wamid
				data.firstTimestamp = timestamp
			}
		}

		c.processMessageByType(ctx, msg, msgType, &data)
	}

	return data
}

func (c *WhatsApp360DialogInbound) processMessageByType(ctx context.Context, msg map[string]any, msgType string, data *dialog360MessageData) {
	switch msgType {
	case "text":
		c.processTextMessage(msg, data)
	case "image":
		c.processImageMessage(ctx, msg, data)
	case "audio":
		c.processAudioMessage(ctx, msg, data)
	case "video":
		c.processVideoMessage(ctx, msg, data)
	case "document":
		c.processDocumentMessage(ctx, msg, data)
	case "location":
		c.processLocationMessage(msg, data)
	case "contacts":
		data.textParts = append(data.textParts, "[Contact shared]")
	case "reaction":
		c.processReactionMessage(msg, data)
	case "interactive":
		c.processInteractiveMessage(msg, data)
	default:
		data.textParts = append(data.textParts, fmt.Sprintf("[Unsupported message type: %s]", msgType))
	}
}

func (c *WhatsApp360DialogInbound) processTextMessage(msg map[string]any, data *dialog360MessageData) {
	if textObj, ok := eywa.GetMap(msg, "text"); ok {
		body := eywa.GetStringOrDefault(textObj, "body", "")
		if body != "" {
			data.textParts = append(data.textParts, body)
		}
	}
}

func (c *WhatsApp360DialogInbound) processImageMessage(ctx context.Context, msg map[string]any, data *dialog360MessageData) {
	imageObj, ok := eywa.GetMap(msg, "image")
	if !ok {
		return
	}

	mediaID := eywa.GetStringOrDefault(imageObj, "id", "")
	mimeType := eywa.GetStringOrDefault(imageObj, "mime_type", "")
	caption := eywa.GetStringOrDefault(imageObj, "caption", "")

	att := &eywa.Artifact{
		Type:     eywa.ArtifactTypeImage,
		MediaID:  mediaID,
		MimeType: mimeType,
		Caption:  caption,
	}

	c.downloadAttachment(ctx, att)
	data.attachments = append(data.attachments, att)

	if caption != "" {
		data.textParts = append(data.textParts, caption)
	}
}

func (c *WhatsApp360DialogInbound) processAudioMessage(ctx context.Context, msg map[string]any, data *dialog360MessageData) {
	audioObj, ok := eywa.GetMap(msg, "audio")
	if !ok {
		return
	}

	att := &eywa.Artifact{
		Type:     eywa.ArtifactTypeAudio,
		MediaID:  eywa.GetStringOrDefault(audioObj, "id", ""),
		MimeType: eywa.GetStringOrDefault(audioObj, "mime_type", ""),
	}

	c.downloadAttachment(ctx, att)
	data.attachments = append(data.attachments, att)
}

func (c *WhatsApp360DialogInbound) processVideoMessage(ctx context.Context, msg map[string]any, data *dialog360MessageData) {
	videoObj, ok := eywa.GetMap(msg, "video")
	if !ok {
		return
	}

	caption := eywa.GetStringOrDefault(videoObj, "caption", "")

	att := &eywa.Artifact{
		Type:     eywa.ArtifactTypeVideo,
		MediaID:  eywa.GetStringOrDefault(videoObj, "id", ""),
		MimeType: eywa.GetStringOrDefault(videoObj, "mime_type", ""),
		Caption:  caption,
	}

	c.downloadAttachment(ctx, att)
	data.attachments = append(data.attachments, att)

	if caption != "" {
		data.textParts = append(data.textParts, caption)
	}
}

func (c *WhatsApp360DialogInbound) processDocumentMessage(ctx context.Context, msg map[string]any, data *dialog360MessageData) {
	docObj, ok := eywa.GetMap(msg, "document")
	if !ok {
		return
	}

	caption := eywa.GetStringOrDefault(docObj, "caption", "")

	att := &eywa.Artifact{
		Type:     eywa.ArtifactTypeDocument,
		MediaID:  eywa.GetStringOrDefault(docObj, "id", ""),
		MimeType: eywa.GetStringOrDefault(docObj, "mime_type", ""),
		FileName: eywa.GetStringOrDefault(docObj, "filename", ""),
		Caption:  caption,
	}

	c.downloadAttachment(ctx, att)
	data.attachments = append(data.attachments, att)

	if caption != "" {
		data.textParts = append(data.textParts, caption)
	}
}

func (c *WhatsApp360DialogInbound) processLocationMessage(msg map[string]any, data *dialog360MessageData) {
	if locObj, ok := eywa.GetMap(msg, "location"); ok {
		lat := eywa.GetFloat64OrDefault(locObj, "latitude", 0)
		lon := eywa.GetFloat64OrDefault(locObj, "longitude", 0)
		name := eywa.GetStringOrDefault(locObj, "name", "")
		address := eywa.GetStringOrDefault(locObj, "address", "")
		locationText := fmt.Sprintf("[Location: %s, %s (%.6f, %.6f)]", name, address, lat, lon)
		data.textParts = append(data.textParts, locationText)
	}
}

func (c *WhatsApp360DialogInbound) processReactionMessage(msg map[string]any, data *dialog360MessageData) {
	if reactionObj, ok := eywa.GetMap(msg, "reaction"); ok {
		emoji := eywa.GetStringOrDefault(reactionObj, "emoji", "")
		if emoji != "" {
			data.textParts = append(data.textParts, fmt.Sprintf("[Reacted with %s to previous message]", emoji))
		} else {
			data.textParts = append(data.textParts, "[Removed reaction from previous message]")
		}
	}
}

func (c *WhatsApp360DialogInbound) processInteractiveMessage(msg map[string]any, data *dialog360MessageData) {
	if interactiveObj, ok := eywa.GetMap(msg, "interactive"); ok {
		interactiveType := eywa.GetStringOrDefault(interactiveObj, "type", "")
		switch interactiveType {
		case "button_reply":
			if buttonReply, ok := eywa.GetMap(interactiveObj, "button_reply"); ok {
				if title := eywa.GetStringOrDefault(buttonReply, "title", ""); title != "" {
					data.textParts = append(data.textParts, title)
				}
			}
		case "list_reply":
			if listReply, ok := eywa.GetMap(interactiveObj, "list_reply"); ok {
				if title := eywa.GetStringOrDefault(listReply, "title", ""); title != "" {
					data.textParts = append(data.textParts, title)
				}
			}
		}
	}
}

// downloadAttachment: on failure logs warning and leaves att.Data empty — non-fatal.
func (c *WhatsApp360DialogInbound) downloadAttachment(ctx context.Context, att *eywa.Artifact) {
	if att.MediaID == "" || c.whatsappClient == nil {
		return
	}

	data, mimeType, err := c.whatsappClient.DownloadMedia(ctx, att.MediaID)
	if err != nil {
		eywa.GetLogger().Warnw("360dialog: failed to download attachment",
			"media_id", att.MediaID, "type", att.Type, "mime_type", att.MimeType, "error", err)
		return
	}

	att.Data = data
	if mimeType != "" && att.MimeType == "" {
		att.MimeType = mimeType
	}
	eywa.GetLogger().Debugw("360dialog: attachment downloaded",
		"media_id", att.MediaID, "type", att.Type, "bytes", len(data))
}

func (c *WhatsApp360DialogInbound) buildEvent(
	eventType, sender string,
	data dialog360MessageData,
	rawPayload map[string]any,
	meta dialog360WebhookMetadata,
) *eywa.Pulse {
	combinedText := eywa.CombineTextPartsMultiLine(data.textParts)

	normalizedPhone, err := eywa.NormalizePhone(sender, "")
	if err != nil {
		normalizedPhone = sender
	}

	builder := eywa.NewPulse(eywa.MemoryKey{
		Channel: "whatsapp",
		User:    normalizedPhone,
	}).
		WithUserMessage(combinedText).
		WithSource("whatsapp_360dialog").
		WithContactPhone(normalizedPhone).
		WithEventType(eventType).
		WithPayload(rawPayload).
		WithIdempotencyKey(data.firstWamid).
		AddMetadata("channel", "whatsapp").
		AddMetadata("provider", "360dialog").
		AddMetadata("sender_phone", normalizedPhone).
		AddMetadata("sender_phone_raw", sender).
		AddMetadata("sender_name", meta.contactName).
		AddMetadata("business_phone", meta.displayPhoneNumber).
		AddMetadata("business_phone_id", meta.phoneNumberID).
		AddMetadata("message_id", data.firstWamid).
		AddMetadata("message_timestamp", data.firstTimestamp).
		AddMetadata("grouped_messages_count", len(data.allWamids))

	if len(data.allWamids) > 1 {
		builder = builder.AddMetadata("grouped_message_ids", data.allWamids)
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
