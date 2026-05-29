package dialog360

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// stubWhatsAppClient implements eywa.WhatsAppClient for tests.
type stubWhatsAppClient struct {
	data     []byte
	mimeType string
	err      error
}

func (s *stubWhatsAppClient) SendMessage(_ context.Context, _, _ string, _ map[string]any) error {
	return nil
}
func (s *stubWhatsAppClient) SendTemplateMessage(_ context.Context, _ string, _ *eywa.TemplateMessage) error {
	return nil
}
func (s *stubWhatsAppClient) DownloadMedia(_ context.Context, _ string) ([]byte, string, error) {
	return s.data, s.mimeType, s.err
}

func buildWebhook(msgs []map[string]any) map[string]any {
	return map[string]any{
		"entry": []any{
			map[string]any{
				"changes": []any{
					map[string]any{
						"value": map[string]any{
							"messages": toSlice(msgs),
							"metadata": map[string]any{
								"display_phone_number": "+5511999990000",
								"phone_number_id":      "pid-123",
							},
							"contacts": []any{
								map[string]any{
									"profile": map[string]any{
										"name": "Test User",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func toSlice(maps []map[string]any) []any {
	out := make([]any, len(maps))
	for i, m := range maps {
		out[i] = m
	}
	return out
}

func textMsg(from, text, id string) map[string]any {
	return map[string]any{
		"id":        id,
		"from":      from,
		"timestamp": "1234567890",
		"type":      "text",
		"text":      map[string]any{"body": text},
	}
}

// --- GetName ---

func TestWhatsApp360DialogInbound_GetName(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	if r.GetName() != "whatsapp_360dialog" {
		t.Errorf("unexpected name: %s", r.GetName())
	}
}

// --- Convert: missing/invalid fields ---

func TestConvert_MissingEntry(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	_, err := r.Convert(context.Background(), "message", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing entry")
	}
}

func TestConvert_EmptyEntry(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := map[string]any{
		"entry": []any{},
	}
	_, err := r.Convert(context.Background(), "message", payload)
	if err == nil {
		t.Fatal("expected error for empty entry")
	}
}

func TestConvert_InvalidEntryFormat(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := map[string]any{
		"entry": []any{"not-a-map"},
	}
	_, err := r.Convert(context.Background(), "message", payload)
	if err == nil {
		t.Fatal("expected error for invalid entry format")
	}
}

func TestConvert_MissingChanges(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := map[string]any{
		"entry": []any{
			map[string]any{"id": "entry-1"},
		},
	}
	_, err := r.Convert(context.Background(), "message", payload)
	if err == nil {
		t.Fatal("expected error for missing changes")
	}
}

func TestConvert_InvalidChangeFormat(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := map[string]any{
		"entry": []any{
			map[string]any{
				"changes": []any{"not-a-map"},
			},
		},
	}
	_, err := r.Convert(context.Background(), "message", payload)
	if err == nil {
		t.Fatal("expected error for invalid change format")
	}
}

func TestConvert_MissingValue(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := map[string]any{
		"entry": []any{
			map[string]any{
				"changes": []any{
					map[string]any{"field": "messages"},
				},
			},
		},
	}
	_, err := r.Convert(context.Background(), "message", payload)
	if err == nil {
		t.Fatal("expected error for missing value")
	}
}

func TestConvert_MissingMessages(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := map[string]any{
		"entry": []any{
			map[string]any{
				"changes": []any{
					map[string]any{
						"value": map[string]any{"metadata": map[string]any{}},
					},
				},
			},
		},
	}
	_, err := r.Convert(context.Background(), "message", payload)
	if err == nil {
		t.Fatal("expected error for missing messages")
	}
}

func TestConvert_NoValidSenders(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	// Message with no 'from' field
	payload := buildWebhook([]map[string]any{
		{"id": "wamid-1", "type": "text", "text": map[string]any{"body": "hi"}},
	})
	_, err := r.Convert(context.Background(), "message", payload)
	if err == nil {
		t.Fatal("expected error for messages with no sender")
	}
}

// --- Convert: success with various message types ---

func TestConvert_TextMessage(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		textMsg("+5511999999999", "Hello world", "wamid-001"),
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) != 1 {
		t.Fatalf("expected 1 pulse, got %d", len(pulses))
	}
	if pulses[0].UserMessage != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", pulses[0].UserMessage)
	}
}

func TestConvert_MultipleSenders(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		textMsg("+5511111111111", "Hi", "wamid-001"),
		textMsg("+5522222222222", "Hello", "wamid-002"),
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) != 2 {
		t.Fatalf("expected 2 pulses, got %d", len(pulses))
	}
}

func TestConvert_MultipleMsgsFromSameSender(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		textMsg("+5511999999999", "First", "wamid-001"),
		textMsg("+5511999999999", "Second", "wamid-002"),
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) != 1 {
		t.Fatalf("expected 1 pulse (grouped), got %d", len(pulses))
	}
}

func TestConvert_ImageMessage(t *testing.T) {
	client := &stubWhatsAppClient{data: []byte("IMG"), mimeType: "image/jpeg"}
	r := NewWhatsApp360DialogInbound(client)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-img", "from": "+5511999999999", "timestamp": "123",
			"type": "image",
			"image": map[string]any{
				"id": "media-123", "mime_type": "image/jpeg", "caption": "Look at this",
			},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) == 0 {
		t.Error("expected attachments for image message")
	}
}

func TestConvert_ImageMessage_DownloadError(t *testing.T) {
	client := &stubWhatsAppClient{err: errDownload}
	r := NewWhatsApp360DialogInbound(client)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-img", "from": "+5511999999999", "timestamp": "123",
			"type":  "image",
			"image": map[string]any{"id": "media-123", "mime_type": "image/jpeg"},
		},
	})
	// Download error is non-fatal — pulse still created
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse even when download fails")
	}
}

func TestConvert_AudioMessage(t *testing.T) {
	client := &stubWhatsAppClient{data: []byte("AUDIO"), mimeType: "audio/ogg"}
	r := NewWhatsApp360DialogInbound(client)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-audio", "from": "+5511999999999", "timestamp": "123",
			"type":  "audio",
			"audio": map[string]any{"id": "audio-123", "mime_type": "audio/ogg"},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) == 0 {
		t.Error("expected attachment for audio message")
	}
}

func TestConvert_VideoMessage_WithCaption(t *testing.T) {
	client := &stubWhatsAppClient{data: []byte("VID"), mimeType: "video/mp4"}
	r := NewWhatsApp360DialogInbound(client)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-vid", "from": "+5511999999999", "timestamp": "123",
			"type":  "video",
			"video": map[string]any{"id": "vid-123", "mime_type": "video/mp4", "caption": "My video"},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) == 0 {
		t.Error("expected attachment for video message")
	}
}

func TestConvert_DocumentMessage_WithCaption(t *testing.T) {
	client := &stubWhatsAppClient{data: []byte("DOC"), mimeType: "application/pdf"}
	r := NewWhatsApp360DialogInbound(client)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-doc", "from": "+5511999999999", "timestamp": "123",
			"type": "document",
			"document": map[string]any{
				"id": "doc-123", "mime_type": "application/pdf",
				"filename": "file.pdf", "caption": "My doc",
			},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) == 0 {
		t.Error("expected attachment for document message")
	}
}

func TestConvert_LocationMessage(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-loc", "from": "+5511999999999", "timestamp": "123",
			"type": "location",
			"location": map[string]any{
				"latitude": -23.5505, "longitude": -46.6333,
				"name": "São Paulo", "address": "Brazil",
			},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 || pulses[0].UserMessage == "" {
		t.Error("expected location text in user message")
	}
}

func TestConvert_ReactionMessage_WithEmoji(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-react", "from": "+5511999999999", "timestamp": "123",
			"type":     "reaction",
			"reaction": map[string]any{"emoji": "👍", "message_id": "wamid-001"},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse for reaction message")
	}
}

func TestConvert_ReactionMessage_Removed(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-react", "from": "+5511999999999", "timestamp": "123",
			"type":     "reaction",
			"reaction": map[string]any{"emoji": "", "message_id": "wamid-001"},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse for removed reaction")
	}
}

func TestConvert_InteractiveMessage_ButtonReply(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-btn", "from": "+5511999999999", "timestamp": "123",
			"type": "interactive",
			"interactive": map[string]any{
				"type":         "button_reply",
				"button_reply": map[string]any{"id": "yes", "title": "Yes"},
			},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pulses[0].UserMessage != "Yes" {
		t.Errorf("expected 'Yes', got %q", pulses[0].UserMessage)
	}
}

func TestConvert_InteractiveMessage_ListReply(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-list", "from": "+5511999999999", "timestamp": "123",
			"type": "interactive",
			"interactive": map[string]any{
				"type":       "list_reply",
				"list_reply": map[string]any{"id": "opt1", "title": "Option 1"},
			},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pulses[0].UserMessage != "Option 1" {
		t.Errorf("expected 'Option 1', got %q", pulses[0].UserMessage)
	}
}

func TestConvert_ContactsMessage(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-contact", "from": "+5511999999999", "timestamp": "123",
			"type": "contacts",
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse for contacts message")
	}
}

func TestConvert_UnknownMessageType(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-unk", "from": "+5511999999999", "timestamp": "123",
			"type": "sticker",
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse for unknown message type (with fallback text)")
	}
}

func TestConvert_DownloadAttachment_NilClient(t *testing.T) {
	// nil whatsappClient → downloadAttachment no-ops
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-img", "from": "+5511999999999", "timestamp": "123",
			"type":  "image",
			"image": map[string]any{"id": "media-123", "mime_type": "image/jpeg"},
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse even with nil client")
	}
}

func TestConvert_DownloadAttachment_MimeMerge(t *testing.T) {
	// Download returns mimeType, attachment has no mimeType → should be merged
	client := &stubWhatsAppClient{data: []byte("DATA"), mimeType: "image/webp"}
	r := NewWhatsApp360DialogInbound(client)
	payload := buildWebhook([]map[string]any{
		{
			"id": "wamid-img", "from": "+5511999999999", "timestamp": "123",
			"type":  "image",
			"image": map[string]any{"id": "media-123"}, // no mime_type
		},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) == 0 {
		t.Fatal("expected attachment")
	}
	if pulses[0].Attachments[0].MimeType != "image/webp" {
		t.Errorf("expected merged mime type 'image/webp', got %q", pulses[0].Attachments[0].MimeType)
	}
}

func TestConvert_AllEventsBuildFail_ReturnsError(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	// Message has 'from' but no 'id' (wamid empty) → buildEventForSender fails
	payload := buildWebhook([]map[string]any{
		{"from": "+5511999999999", "timestamp": "123", "type": "text",
			"text": map[string]any{"body": "hi"}},
		// no "id" field → wamid will be empty
	})
	_, err := r.Convert(context.Background(), "message", payload)
	if err == nil {
		t.Fatal("expected error when all events fail to build")
	}
}

// --- Defensive !ok branches: message type without matching key ---

func TestConvert_ImageType_NoImageKey(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{"id": "wamid-x", "from": "+5511999999999", "timestamp": "123", "type": "image"},
		// no "image" key → processImageMessage returns early
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse even when image key missing")
	}
	if len(pulses[0].Attachments) != 0 {
		t.Error("expected no attachments when image key missing")
	}
}

func TestConvert_AudioType_NoAudioKey(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{"id": "wamid-x", "from": "+5511999999999", "timestamp": "123", "type": "audio"},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse even when audio key missing")
	}
}

func TestConvert_VideoType_NoVideoKey(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{"id": "wamid-x", "from": "+5511999999999", "timestamp": "123", "type": "video"},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse even when video key missing")
	}
}

func TestConvert_DocumentType_NoDocumentKey(t *testing.T) {
	r := NewWhatsApp360DialogInbound(nil)
	payload := buildWebhook([]map[string]any{
		{"id": "wamid-x", "from": "+5511999999999", "timestamp": "123", "type": "document"},
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse even when document key missing")
	}
}

// sentinel error for tests
var errDownload = &stubError{"download failed"}

type stubError struct{ msg string }

func (e *stubError) Error() string { return e.msg }
