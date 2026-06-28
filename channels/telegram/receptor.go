package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	eywa "github.com/wmulabs/eywa"
)

// Inbound converts a Telegram Bot API webhook Update into a Pulse. Text and media (photo, voice, audio,
// video, document) are supported; captions are folded into the user message. Bot-authored messages are
// ignored so the agent never answers itself. A message with neither text nor media is rejected.
type Inbound struct {
	client *Client
}

// NewInbound builds the Telegram Receptor. The client is used to download media bytes (getFile +
// download); pass nil to skip downloads (attachments are still recorded, without their bytes).
func NewInbound(client *Client) eywa.Receptor { return &Inbound{client: client} }

func (i *Inbound) GetName() string { return "telegram" }

func (i *Inbound) Convert(ctx context.Context, eventType string, raw map[string]any) ([]*eywa.Pulse, error) {
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
	attachments := i.extractAttachments(ctx, msg)

	if text == "" {
		text = eywa.GetStringOrDefault(msg, "caption", "")
	}
	if text == "" && len(attachments) == 0 {
		return nil, fmt.Errorf("telegram: unsupported message (no text or media)")
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

	if len(attachments) > 0 {
		builder = builder.WithAttachments(attachments).AddMetadata("attachment_count", len(attachments))
	}

	return []*eywa.Pulse{builder.Build()}, nil
}

// extractAttachments maps Telegram media objects to Artifacts and downloads their bytes when a client is
// configured. MIME types come from the message (Telegram does not return them on getFile); photos are
// always JPEG. Download failures are non-fatal — the Artifact is kept without Data.
func (i *Inbound) extractAttachments(ctx context.Context, msg map[string]any) []*eywa.Artifact {
	caption := eywa.GetStringOrDefault(msg, "caption", "")
	var atts []*eywa.Artifact

	// photo is an array of sizes (smallest → largest); take the largest.
	if photos, ok := eywa.GetSlice(msg, "photo"); ok && len(photos) > 0 {
		if largest, ok := photos[len(photos)-1].(map[string]any); ok {
			atts = append(atts, &eywa.Artifact{
				Type:     eywa.ArtifactTypeImage,
				MediaID:  eywa.GetStringOrDefault(largest, "file_id", ""),
				MimeType: "image/jpeg",
				Caption:  caption,
			})
		}
	}

	if o, ok := eywa.GetMap(msg, "voice"); ok {
		atts = append(atts, mediaArtifact(eywa.ArtifactTypeAudio, o, ""))
	}
	if o, ok := eywa.GetMap(msg, "audio"); ok {
		atts = append(atts, mediaArtifact(eywa.ArtifactTypeAudio, o, ""))
	}
	if o, ok := eywa.GetMap(msg, "video"); ok {
		atts = append(atts, mediaArtifact(eywa.ArtifactTypeVideo, o, caption))
	}
	if o, ok := eywa.GetMap(msg, "document"); ok {
		att := mediaArtifact(eywa.ArtifactTypeDocument, o, caption)
		att.FileName = eywa.GetStringOrDefault(o, "file_name", "")
		atts = append(atts, att)
	}

	for _, att := range atts {
		if att.MediaID == "" || i.client == nil {
			continue
		}
		data, err := i.client.DownloadFile(ctx, att.MediaID)
		if err != nil {
			eywa.GetLogger().Warnw("telegram: failed to download attachment",
				"file_id", att.MediaID, "type", att.Type, "error", err)
			continue
		}
		att.Data = data
	}

	return atts
}

func mediaArtifact(t eywa.ArtifactType, obj map[string]any, caption string) *eywa.Artifact {
	return &eywa.Artifact{
		Type:     t,
		MediaID:  eywa.GetStringOrDefault(obj, "file_id", ""),
		MimeType: eywa.GetStringOrDefault(obj, "mime_type", ""),
		Caption:  strings.TrimSpace(caption),
	}
}
