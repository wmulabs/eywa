package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func messageCallback(ev map[string]any) map[string]any {
	return map[string]any{
		"type":     "event_callback",
		"team_id":  "T1",
		"event_id": "Ev1",
		"event":    ev,
	}
}

func TestInbound_Convert_Message(t *testing.T) {
	raw := messageCallback(map[string]any{
		"type": "message", "channel": "C1", "user": "U1", "text": "hello", "ts": "123.45",
	})
	pulses, err := NewInbound(nil).Convert(context.Background(), "user_message", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pulses[0]
	if p.UserMessage != "hello" || p.Source != "slack" {
		t.Errorf("unexpected pulse: msg=%q src=%q", p.UserMessage, p.Source)
	}
	if p.Metadata["slack_channel"] != "C1" || p.Metadata["slack_user"] != "U1" {
		t.Errorf("metadata: %+v", p.Metadata)
	}
	if p.IdempotencyKey != "slack:Ev1" {
		t.Errorf("idempotency = %q", p.IdempotencyKey)
	}
}

func TestInbound_Convert_URLVerification_NoPulse(t *testing.T) {
	pulses, err := NewInbound(nil).Convert(context.Background(), "e", map[string]any{
		"type": "url_verification", "challenge": "x",
	})
	if err != nil || pulses != nil {
		t.Errorf("url_verification must yield no pulse and no error, got %v / %v", pulses, err)
	}
}

func TestInbound_Convert_Rejections(t *testing.T) {
	cases := map[string]map[string]any{
		"unknown envelope": {"type": "something_else"},
		"no event":         {"type": "event_callback"},
		"non-message":      messageCallback(map[string]any{"type": "reaction_added", "channel": "C1"}),
		"bot message":      messageCallback(map[string]any{"type": "message", "channel": "C1", "bot_id": "B1", "text": "hi"}),
		"missing channel":  messageCallback(map[string]any{"type": "message", "text": "hi"}),
		"empty":            messageCallback(map[string]any{"type": "message", "channel": "C1"}),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewInbound(nil).Convert(context.Background(), "e", raw); err == nil {
				t.Errorf("expected rejection for %s", name)
			}
		})
	}
}

func TestInbound_Convert_Files(t *testing.T) {
	raw := messageCallback(map[string]any{
		"type": "message", "channel": "C1",
		"files": []any{
			map[string]any{"id": "F1", "mimetype": "image/png", "name": "a.png", "url_private_download": "https://files.slack.com/a"},
			map[string]any{"id": "F2", "mimetype": "application/pdf", "name": "b.pdf"},
		},
	})
	pulses, err := NewInbound(nil).Convert(context.Background(), "e", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	atts := pulses[0].Attachments
	if len(atts) != 2 || atts[0].Type != "image" || atts[1].Type != "document" {
		t.Fatalf("unexpected attachments: %+v", atts)
	}
	if atts[1].FileName != "b.pdf" {
		t.Errorf("filename not mapped: %+v", atts[1])
	}
}

func TestInbound_Convert_DownloadsFileBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing bearer auth")
		}
		_, _ = w.Write([]byte("PNGBYTES"))
	}))
	defer srv.Close()

	client := NewClient("tok")
	client.skipHostCheck = true
	raw := messageCallback(map[string]any{
		"type": "message", "channel": "C1",
		"files": []any{map[string]any{"id": "F1", "mimetype": "image/png", "url_private_download": srv.URL + "/f"}},
	})
	pulses, err := NewInbound(client).Convert(context.Background(), "e", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(pulses[0].Attachments[0].Data) != "PNGBYTES" {
		t.Errorf("file bytes = %q", string(pulses[0].Attachments[0].Data))
	}
}

func TestInbound_Convert_SkipsNonMapFile(t *testing.T) {
	raw := messageCallback(map[string]any{
		"type": "message", "channel": "C1", "text": "hi",
		"files": []any{"not-a-map", map[string]any{"id": "F1", "mimetype": "image/png"}},
	})
	pulses, err := NewInbound(nil).Convert(context.Background(), "e", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) != 1 {
		t.Errorf("non-map file entry must be skipped, got %d attachments", len(pulses[0].Attachments))
	}
}

func TestArtifactType(t *testing.T) {
	cases := map[string]string{
		"image/png":       "image",
		"audio/mpeg":      "audio",
		"video/mp4":       "video",
		"application/pdf": "document",
		"":                "document",
	}
	for mime, want := range cases {
		if got := string(artifactType(mime)); got != want {
			t.Errorf("artifactType(%q) = %q, want %q", mime, got, want)
		}
	}
}

func TestInbound_GetName(t *testing.T) {
	if NewInbound(nil).GetName() != "slack" {
		t.Error("name must be slack")
	}
}
