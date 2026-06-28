package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func textUpdate() map[string]any {
	return map[string]any{
		"update_id": float64(42),
		"message": map[string]any{
			"message_id": float64(7),
			"from":       map[string]any{"id": float64(99), "is_bot": false, "username": "alice"},
			"chat":       map[string]any{"id": float64(12345), "type": "private"},
			"text":       "hello bot",
		},
	}
}

func TestInbound_Convert_TextMessage(t *testing.T) {
	pulses, err := NewInbound(nil).Convert(context.Background(), "user_message", textUpdate())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) != 1 {
		t.Fatalf("expected 1 pulse, got %d", len(pulses))
	}
	p := pulses[0]
	if p.UserMessage != "hello bot" {
		t.Errorf("UserMessage = %q", p.UserMessage)
	}
	if p.Source != "telegram" {
		t.Errorf("Source = %q", p.Source)
	}
	if got, _ := p.Metadata["telegram_chat_id"].(int64); got != 12345 {
		t.Errorf("telegram_chat_id = %v", p.Metadata["telegram_chat_id"])
	}
	if p.IdempotencyKey != "telegram:42" {
		t.Errorf("IdempotencyKey = %q", p.IdempotencyKey)
	}
	if p.Metadata["sender_name"] != "alice" {
		t.Errorf("sender_name = %v", p.Metadata["sender_name"])
	}
}

func TestInbound_Convert_Rejections(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
	}{
		{"no message", map[string]any{"update_id": float64(1)}},
		{"bot author", map[string]any{"message": map[string]any{
			"from": map[string]any{"is_bot": true},
			"chat": map[string]any{"id": float64(1)},
			"text": "hi",
		}}},
		{"no chat", map[string]any{"message": map[string]any{"text": "hi"}}},
		{"no text", map[string]any{"message": map[string]any{"chat": map[string]any{"id": float64(1)}}}},
		{"zero chat id", map[string]any{"message": map[string]any{"chat": map[string]any{"id": float64(0)}, "text": "hi"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewInbound(nil).Convert(context.Background(), "user_message", c.raw); err == nil {
				t.Errorf("expected rejection for %s", c.name)
			}
		})
	}
}

func TestInbound_GetName(t *testing.T) {
	if NewInbound(nil).GetName() != "telegram" {
		t.Error("name must be telegram")
	}
}

func TestInbound_Convert_Photo(t *testing.T) {
	raw := map[string]any{
		"update_id": float64(5),
		"message": map[string]any{
			"chat":    map[string]any{"id": float64(1)},
			"caption": "look at this",
			"photo": []any{
				map[string]any{"file_id": "small"},
				map[string]any{"file_id": "large"},
			},
		},
	}
	pulses, err := NewInbound(nil).Convert(context.Background(), "user_message", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pulses[0]
	if p.UserMessage != "look at this" {
		t.Errorf("caption should become the message, got %q", p.UserMessage)
	}
	if len(p.Attachments) != 1 || p.Attachments[0].Type != "image" {
		t.Fatalf("expected 1 image attachment, got %+v", p.Attachments)
	}
	if p.Attachments[0].MediaID != "large" {
		t.Errorf("must pick the largest photo size, got %q", p.Attachments[0].MediaID)
	}
}

func TestInbound_Convert_Voice_NoCaption(t *testing.T) {
	raw := map[string]any{
		"message": map[string]any{
			"chat":  map[string]any{"id": float64(1)},
			"voice": map[string]any{"file_id": "v1", "mime_type": "audio/ogg"},
		},
	}
	pulses, err := NewInbound(nil).Convert(context.Background(), "user_message", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	atts := pulses[0].Attachments
	if len(atts) != 1 || atts[0].Type != "audio" || atts[0].MimeType != "audio/ogg" {
		t.Errorf("expected one audio attachment, got %+v", atts)
	}
}

func TestInbound_Convert_AudioVideoDocument(t *testing.T) {
	raw := map[string]any{
		"message": map[string]any{
			"chat":     map[string]any{"id": float64(1)},
			"audio":    map[string]any{"file_id": "a", "mime_type": "audio/mpeg"},
			"video":    map[string]any{"file_id": "v", "mime_type": "video/mp4"},
			"document": map[string]any{"file_id": "d", "mime_type": "application/pdf", "file_name": "report.pdf"},
		},
	}
	pulses, err := NewInbound(nil).Convert(context.Background(), "user_message", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	atts := pulses[0].Attachments
	if len(atts) != 3 {
		t.Fatalf("expected 3 attachments, got %d", len(atts))
	}
	var doc *struct{ name, mime string }
	for _, a := range atts {
		if a.Type == "document" {
			doc = &struct{ name, mime string }{a.FileName, a.MimeType}
		}
	}
	if doc == nil || doc.name != "report.pdf" || doc.mime != "application/pdf" {
		t.Errorf("document attachment not mapped: %+v", atts)
	}
}

func TestInbound_Convert_DownloadFailure_NonFatal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound) // getFile fails
	}))
	defer srv.Close()
	client := NewClient("tok")
	client.baseURL = srv.URL

	raw := map[string]any{
		"message": map[string]any{
			"chat":  map[string]any{"id": float64(1)},
			"photo": []any{map[string]any{"file_id": "x"}},
		},
	}
	pulses, err := NewInbound(client).Convert(context.Background(), "user_message", raw)
	if err != nil {
		t.Fatalf("download failure must be non-fatal, got: %v", err)
	}
	if len(pulses[0].Attachments) != 1 || len(pulses[0].Attachments[0].Data) != 0 {
		t.Error("attachment should be kept without Data on download failure")
	}
}

func TestInbound_Convert_DownloadsMediaBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/getFile"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_path":"photos/x.jpg"}}`))
		case strings.Contains(r.URL.Path, "/file/bottok/photos/x.jpg"):
			_, _ = w.Write([]byte("JPEGBYTES"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient("tok")
	client.baseURL = srv.URL
	raw := map[string]any{
		"message": map[string]any{
			"chat":  map[string]any{"id": float64(1)},
			"photo": []any{map[string]any{"file_id": "large"}},
		},
	}
	pulses, err := NewInbound(client).Convert(context.Background(), "user_message", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(pulses[0].Attachments[0].Data); got != "JPEGBYTES" {
		t.Errorf("attachment bytes = %q, want JPEGBYTES", got)
	}
}
