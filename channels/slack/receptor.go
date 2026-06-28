package slack

import (
	"context"
	"fmt"
	"strings"

	eywa "github.com/wmulabs/eywa"
)

// Inbound converts a Slack Events API event_callback into a Pulse. Message events become Pulses keyed by
// channel; files are attached (and downloaded when a client is configured). url_verification envelopes
// produce no Pulse — the route echoes the challenge (see URLVerificationChallenge). Bot-authored messages
// are ignored so the agent never answers itself.
type Inbound struct {
	client *Client
}

func NewInbound(client *Client) eywa.Receptor { return &Inbound{client: client} }

func (i *Inbound) GetName() string { return "slack" }

func (i *Inbound) Convert(ctx context.Context, eventType string, raw map[string]any) ([]*eywa.Pulse, error) {
	switch eywa.GetStringOrDefault(raw, "type", "") {
	case "url_verification":
		return nil, nil // handshake handled at the HTTP route, not here
	case "event_callback":
		// proceed
	default:
		return nil, fmt.Errorf("slack: unsupported envelope type")
	}

	ev, ok := eywa.GetMap(raw, "event")
	if !ok {
		return nil, fmt.Errorf("slack: no 'event' in callback")
	}
	if eywa.GetStringOrDefault(ev, "type", "") != "message" {
		return nil, fmt.Errorf("slack: non-message event")
	}
	if eywa.GetStringOrDefault(ev, "bot_id", "") != "" || eywa.GetStringOrDefault(ev, "subtype", "") == "bot_message" {
		return nil, fmt.Errorf("slack: ignoring bot-authored message")
	}

	channel := eywa.GetStringOrDefault(ev, "channel", "")
	if channel == "" {
		return nil, fmt.Errorf("slack: missing channel")
	}

	text := eywa.GetStringOrDefault(ev, "text", "")
	attachments := i.extractFiles(ctx, ev)
	if text == "" && len(attachments) == 0 {
		return nil, fmt.Errorf("slack: empty message (no text or files)")
	}

	builder := eywa.NewPulse(eywa.MemoryKey{Channel: "slack", User: channel}).
		WithUserMessage(text).
		WithSource("slack").
		WithEventType(eventType).
		WithPayload(raw).
		WithIdempotencyKey("slack:"+eywa.GetStringOrDefault(raw, "event_id", "")).
		AddMetadata("channel", "slack").
		AddMetadata("slack_channel", channel).
		AddMetadata("slack_user", eywa.GetStringOrDefault(ev, "user", "")).
		AddMetadata("slack_team", eywa.GetStringOrDefault(raw, "team_id", ""))

	if len(attachments) > 0 {
		builder = builder.WithAttachments(attachments).AddMetadata("attachment_count", len(attachments))
	}

	return []*eywa.Pulse{builder.Build()}, nil
}

func (i *Inbound) extractFiles(ctx context.Context, ev map[string]any) []*eywa.Artifact {
	files, ok := eywa.GetSlice(ev, "files")
	if !ok || len(files) == 0 {
		return nil
	}

	var atts []*eywa.Artifact
	for _, f := range files {
		fm, ok := f.(map[string]any)
		if !ok {
			continue
		}
		mime := eywa.GetStringOrDefault(fm, "mimetype", "")
		att := &eywa.Artifact{
			Type:     artifactType(mime),
			MediaID:  eywa.GetStringOrDefault(fm, "id", ""),
			MimeType: mime,
			FileName: eywa.GetStringOrDefault(fm, "name", ""),
		}

		fileURL := eywa.GetStringOrDefault(fm, "url_private_download", eywa.GetStringOrDefault(fm, "url_private", ""))
		if fileURL != "" && i.client != nil {
			data, err := i.client.DownloadFile(ctx, fileURL)
			if err != nil {
				eywa.GetLogger().Warnw("slack: failed to download file",
					"file_id", att.MediaID, "type", att.Type, "error", err)
			} else {
				att.Data = data
			}
		}
		atts = append(atts, att)
	}
	return atts
}

func artifactType(mime string) eywa.ArtifactType {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return eywa.ArtifactTypeImage
	case strings.HasPrefix(mime, "audio/"):
		return eywa.ArtifactTypeAudio
	case strings.HasPrefix(mime, "video/"):
		return eywa.ArtifactTypeVideo
	default:
		return eywa.ArtifactTypeDocument
	}
}
