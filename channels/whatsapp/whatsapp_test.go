package whatsapp

import (
	"context"
	"errors"
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// stubWAClient implements eywa.WhatsAppClient for tests.
type stubWAClient struct {
	sendMessageErr  error
	sendTemplateErr error
}

func (s *stubWAClient) SendMessage(_ context.Context, _, _ string, _ map[string]any) error {
	return s.sendMessageErr
}
func (s *stubWAClient) SendTemplateMessage(_ context.Context, _ string, _ *eywa.TemplateMessage) error {
	return s.sendTemplateErr
}
func (s *stubWAClient) DownloadMedia(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", nil
}

// --- SendWhatsAppMessageTool ---

func TestSendMessageTool_GetName(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	if tool.GetName() != "send_whatsapp_message" {
		t.Errorf("unexpected name: %s", tool.GetName())
	}
}

func TestSendMessageTool_GetDescription(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	if tool.GetDescription() == "" {
		t.Error("expected non-empty description")
	}
}

func TestSendMessageTool_GetParameters(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	params := tool.GetParameters()
	if params == nil {
		t.Fatal("expected non-nil parameters")
	}
}

func TestSendMessageTool_IsCritical_Default(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	if !tool.IsCritical() {
		t.Error("expected default IsCritical=true")
	}
}

func TestSendMessageTool_IsCritical_NonCritical(t *testing.T) {
	tool := NewSendWhatsAppMessageToolWithCriticality(&stubWAClient{}, false)
	if tool.IsCritical() {
		t.Error("expected IsCritical=false")
	}
}

func TestSendMessageTool_GetCategory(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	if tool.GetCategory() != eywa.ActionDelivery {
		t.Errorf("expected ActionDelivery, got %v", tool.GetCategory())
	}
}

// --- Validate ---

func TestSendMessageTool_Validate_Valid(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	err := tool.Validate(map[string]any{
		"phone":   "+5511999999999",
		"message": "Hello",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessageTool_Validate_MissingPhone(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	if err := tool.Validate(map[string]any{"message": "hi"}); err == nil {
		t.Fatal("expected error for missing phone")
	}
}

func TestSendMessageTool_Validate_MissingMessage(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	if err := tool.Validate(map[string]any{"phone": "+55119"}); err == nil {
		t.Fatal("expected error for missing message")
	}
}

func TestSendMessageTool_Validate_TooManyButtons(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	err := tool.Validate(map[string]any{
		"phone":   "+5511999999999",
		"message": "Choose:",
		"buttons": []any{
			map[string]any{"title": "A"},
			map[string]any{"title": "B"},
			map[string]any{"title": "C"},
			map[string]any{"title": "D"},
		},
	})
	if err == nil {
		t.Fatal("expected error for >3 buttons")
	}
}

func TestSendMessageTool_Validate_ButtonNotObject(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	err := tool.Validate(map[string]any{
		"phone":   "+5511999999999",
		"message": "Choose:",
		"buttons": []any{"not-a-map"},
	})
	if err == nil {
		t.Fatal("expected error for non-object button")
	}
}

func TestSendMessageTool_Validate_ButtonMissingTitle(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	err := tool.Validate(map[string]any{
		"phone":   "+5511999999999",
		"message": "Choose:",
		"buttons": []any{
			map[string]any{"id": "btn1"}, // no title
		},
	})
	if err == nil {
		t.Fatal("expected error for button missing title")
	}
}

func TestSendMessageTool_Validate_InvalidImageURL(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	err := tool.Validate(map[string]any{
		"phone":     "+5511999999999",
		"message":   "Look:",
		"image_url": "ftp://invalid.com/img.png",
	})
	if err == nil {
		t.Fatal("expected error for non-http image_url")
	}
}

func TestSendMessageTool_Validate_ValidButtons(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	err := tool.Validate(map[string]any{
		"phone":   "+5511999999999",
		"message": "Choose:",
		"buttons": []any{
			map[string]any{"title": "Yes"},
			map[string]any{"title": "No"},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Execute ---

func TestSendMessageTool_Execute_Success(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	result, err := tool.Execute(context.Background(), map[string]any{
		"phone":   "+5511999999999",
		"message": "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestSendMessageTool_Execute_WithButtons(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"phone":   "+5511999999999",
		"message": "Choose:",
		"buttons": []any{
			map[string]any{"title": "Yes"},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessageTool_Execute_WithImageURL(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"phone":     "+5511999999999",
		"message":   "Look:",
		"image_url": "https://example.com/img.png",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessageTool_Execute_ClientError(t *testing.T) {
	tool := NewSendWhatsAppMessageTool(&stubWAClient{sendMessageErr: errors.New("send failed")})
	_, err := tool.Execute(context.Background(), map[string]any{
		"phone":   "+5511999999999",
		"message": "Hello",
	})
	if err == nil {
		t.Fatal("expected error from client")
	}
}

func TestSendMessageTool_Execute_InvalidPhone_UsesOriginal(t *testing.T) {
	// "notaphone" fails NormalizePhone → should still call SendMessage with original
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"phone":   "notaphone",
		"message": "Hello",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessageTool_Execute_PhoneNormalized(t *testing.T) {
	// Formatted number normalizes to E164 (different from raw) → triggers info log branch
	tool := NewSendWhatsAppMessageTool(&stubWAClient{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"phone":   "+55 11 99999-9999",
		"message": "Hello",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- SendWhatsAppTemplateTool ---

func TestSendTemplateTool_GetName(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if tool.GetName() != "send_whatsapp_template" {
		t.Errorf("unexpected name: %s", tool.GetName())
	}
}

func TestSendTemplateTool_GetDescription(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if tool.GetDescription() == "" {
		t.Error("expected non-empty description")
	}
}

func TestSendTemplateTool_GetParameters(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if tool.GetParameters() == nil {
		t.Error("expected non-nil parameters")
	}
}

func TestSendTemplateTool_IsCritical_Default(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if !tool.IsCritical() {
		t.Error("expected IsCritical=true by default")
	}
}

func TestSendTemplateTool_IsCritical_NonCritical(t *testing.T) {
	tool := NewSendWhatsAppTemplateToolWithCriticality(&stubWAClient{}, false)
	if tool.IsCritical() {
		t.Error("expected IsCritical=false")
	}
}

func TestSendTemplateTool_GetCategory(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if tool.GetCategory() != eywa.ActionDelivery {
		t.Errorf("expected ActionDelivery, got %v", tool.GetCategory())
	}
}

// --- Template Validate ---

func TestSendTemplateTool_Validate_Valid(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	err := tool.Validate(map[string]any{
		"phone":         "+5511999999999",
		"template_name": "welcome",
		"language":      "pt_BR",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendTemplateTool_Validate_MissingPhone(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if err := tool.Validate(map[string]any{
		"template_name": "welcome",
		"language":      "pt_BR",
	}); err == nil {
		t.Fatal("expected error for missing phone")
	}
}

func TestSendTemplateTool_Validate_MissingTemplateName(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if err := tool.Validate(map[string]any{
		"phone":    "+5511999999999",
		"language": "pt_BR",
	}); err == nil {
		t.Fatal("expected error for missing template_name")
	}
}

func TestSendTemplateTool_Validate_MissingLanguage(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if err := tool.Validate(map[string]any{
		"phone":         "+5511999999999",
		"template_name": "welcome",
	}); err == nil {
		t.Fatal("expected error for missing language")
	}
}

func TestSendTemplateTool_Validate_HeaderParamsNotArray(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if err := tool.Validate(map[string]any{
		"phone":         "+5511999999999",
		"template_name": "welcome",
		"language":      "pt_BR",
		"header_params": "not-an-array",
	}); err == nil {
		t.Fatal("expected error for header_params not array")
	}
}

func TestSendTemplateTool_Validate_BodyParamsNotArray(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if err := tool.Validate(map[string]any{
		"phone":         "+5511999999999",
		"template_name": "welcome",
		"language":      "pt_BR",
		"body_params":   "not-an-array",
	}); err == nil {
		t.Fatal("expected error for body_params not array")
	}
}

func TestSendTemplateTool_Validate_ButtonParamsNotArray(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	if err := tool.Validate(map[string]any{
		"phone":         "+5511999999999",
		"template_name": "welcome",
		"language":      "pt_BR",
		"button_params": 42,
	}); err == nil {
		t.Fatal("expected error for button_params not array")
	}
}

// --- Template Execute ---

func TestSendTemplateTool_Execute_Success_NoParams(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	result, err := tool.Execute(context.Background(), map[string]any{
		"phone":         "+5511999999999",
		"template_name": "hello_world",
		"language":      "en_US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hello_world") {
		t.Errorf("expected template name in result, got %q", result)
	}
}

func TestSendTemplateTool_Execute_WithHeaderBodyButtonParams(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"phone":         "+5511999999999",
		"template_name": "order_update",
		"language":      "pt_BR",
		"header_params": []any{"https://example.com/img.png"},                     // image URL
		"body_params":   []any{"João", "https://example.com/v.mp4", "text-param"}, // video, text
		"button_params": []any{"payload1"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendTemplateTool_Execute_ClientError(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{sendTemplateErr: errors.New("send failed")})
	_, err := tool.Execute(context.Background(), map[string]any{
		"phone":         "+5511999999999",
		"template_name": "hello",
		"language":      "en",
	})
	if err == nil {
		t.Fatal("expected error from client")
	}
}

func TestSendTemplateTool_Execute_InvalidPhone_UsesOriginal(t *testing.T) {
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"phone":         "notaphone",
		"template_name": "hello",
		"language":      "en",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendTemplateTool_Execute_PhoneNormalized(t *testing.T) {
	// Formatted number normalizes to E164 (different from raw)
	tool := NewSendWhatsAppTemplateTool(&stubWAClient{})
	_, err := tool.Execute(context.Background(), map[string]any{
		"phone":         "+55 11 99999-9999",
		"template_name": "hello",
		"language":      "en",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- isImageURL / isVideoURL ---

func TestIsImageURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://example.com/img.jpg", true},
		{"https://example.com/img.jpeg", true},
		{"https://example.com/img.png", true},
		{"https://example.com/img.gif", true},
		{"http://example.com/img.png", true},
		{"https://example.com/vid.mp4", false},
		{"https://example.com/file.pdf", false},
		{"not-a-url", false},
	}
	for _, tc := range cases {
		got := isImageURL(tc.url)
		if got != tc.want {
			t.Errorf("isImageURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestIsVideoURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://example.com/v.mp4", true},
		{"https://example.com/v.mov", true},
		{"https://example.com/v.avi", true},
		{"http://example.com/v.mp4", true},
		{"https://example.com/img.png", false},
		{"not-a-url", false},
	}
	for _, tc := range cases {
		got := isVideoURL(tc.url)
		if got != tc.want {
			t.Errorf("isVideoURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

// --- WhatsAppResponseChannel ---

func TestResponseChannel_GetName(t *testing.T) {
	ch := NewWhatsAppResponseChannel(&stubWAClient{})
	if ch.GetName() != "whatsapp" {
		t.Errorf("unexpected name: %s", ch.GetName())
	}
}

func TestResponseChannel_ShouldAutoRespond(t *testing.T) {
	ch := NewWhatsAppResponseChannel(&stubWAClient{})
	if !ch.ShouldAutoRespond() {
		t.Error("expected ShouldAutoRespond=true")
	}
}

func TestResponseChannel_GetChannelMetadata(t *testing.T) {
	ch := NewWhatsAppResponseChannel(&stubWAClient{})
	pulse := &eywa.Pulse{ContactPhone: "+5511999999999", Source: "whatsapp"}
	meta := ch.GetChannelMetadata(pulse)
	if meta["channel"] != "whatsapp" {
		t.Errorf("expected channel=whatsapp, got %v", meta["channel"])
	}
	if meta["phone_number"] != "+5511999999999" {
		t.Errorf("unexpected phone_number: %v", meta["phone_number"])
	}
}

func TestResponseChannel_SendResponse_Success(t *testing.T) {
	ch := NewWhatsAppResponseChannel(&stubWAClient{})
	pulse := &eywa.Pulse{ContactPhone: "+5511999999999"}
	if err := ch.SendResponse(context.Background(), pulse, "Hello!"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResponseChannel_SendResponse_EmptyResponse(t *testing.T) {
	ch := NewWhatsAppResponseChannel(&stubWAClient{})
	pulse := &eywa.Pulse{ContactPhone: "+5511999999999"}
	if err := ch.SendResponse(context.Background(), pulse, ""); err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestResponseChannel_SendResponse_NoPhone(t *testing.T) {
	ch := NewWhatsAppResponseChannel(&stubWAClient{})
	pulse := &eywa.Pulse{}
	if err := ch.SendResponse(context.Background(), pulse, "Hello"); err == nil {
		t.Fatal("expected error for missing phone")
	}
}

func TestResponseChannel_SendResponse_InvalidPhone(t *testing.T) {
	ch := NewWhatsAppResponseChannel(&stubWAClient{})
	pulse := &eywa.Pulse{ContactPhone: "notaphone"}
	if err := ch.SendResponse(context.Background(), pulse, "Hello"); err == nil {
		t.Fatal("expected error for invalid phone")
	}
}

func TestResponseChannel_SendResponse_ClientError(t *testing.T) {
	ch := NewWhatsAppResponseChannel(&stubWAClient{sendMessageErr: errors.New("send failed")})
	pulse := &eywa.Pulse{ContactPhone: "+5511999999999"}
	if err := ch.SendResponse(context.Background(), pulse, "Hello"); err == nil {
		t.Fatal("expected error from client")
	}
}
