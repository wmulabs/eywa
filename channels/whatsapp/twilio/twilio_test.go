package twilio

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
)

// --- NewTwilioClient defaults ---

func TestNewTwilioClient_Defaults(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{
		AccountSID: "AC123",
		AuthToken:  "tok",
		FromNumber: "+15551234567",
	})
	if c.pollInterval != defaultPollInterval {
		t.Errorf("expected default poll interval, got %v", c.pollInterval)
	}
	if c.deliveryTimeout != defaultDeliveryTimeout {
		t.Errorf("expected default delivery timeout, got %v", c.deliveryTimeout)
	}
	if c.templateSIDs == nil {
		t.Error("expected non-nil templateSIDs map")
	}
}

func TestNewTwilioClient_CustomIntervals(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{
		AccountSID:      "AC123",
		AuthToken:       "tok",
		FromNumber:      "+15551234567",
		PollInterval:    5 * time.Second,
		DeliveryTimeout: 60 * time.Second,
	})
	if c.pollInterval != 5*time.Second {
		t.Errorf("expected 5s poll interval, got %v", c.pollInterval)
	}
	if c.deliveryTimeout != 60*time.Second {
		t.Errorf("expected 60s delivery timeout, got %v", c.deliveryTimeout)
	}
}

func TestNewTwilioClient_FromNumberFormatted(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{FromNumber: "+15551234567"})
	if c.fromNumber != "whatsapp:+15551234567" {
		t.Errorf("expected whatsapp: prefix, got %q", c.fromNumber)
	}
}

func TestNewTwilioClient_FromNumberAlreadyFormatted(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{FromNumber: "whatsapp:+15551234567"})
	if c.fromNumber != "whatsapp:+15551234567" {
		t.Errorf("expected unchanged, got %q", c.fromNumber)
	}
}

func TestNewTwilioClient_WithTemplateSIDs(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{
		TemplateSIDs: map[string]string{"hello": "HXabc123"},
	})
	if c.templateSIDs["hello"] != "HXabc123" {
		t.Errorf("expected template SID 'HXabc123', got %q", c.templateSIDs["hello"])
	}
}

// --- resolveTemplateSID ---

func TestResolveTemplateSID_Found(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{
		TemplateSIDs: map[string]string{"hello_world": "HX123"},
	})
	sid, err := c.resolveTemplateSID("hello_world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "HX123" {
		t.Errorf("expected HX123, got %q", sid)
	}
}

func TestResolveTemplateSID_NotFound(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{
		TemplateSIDs: map[string]string{"other": "HX999"},
	})
	_, err := c.resolveTemplateSID("missing_template")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
}

// --- buildTwilioVariables ---

func TestBuildTwilioVariables_NoComponents(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	tmpl := &eywa.TemplateMessage{Name: "t", Language: "en"}
	vars := c.buildTwilioVariables(tmpl)
	if len(vars) != 0 {
		t.Errorf("expected empty vars, got %v", vars)
	}
}

func TestBuildTwilioVariables_AllParamTypes(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	tmpl := &eywa.TemplateMessage{
		Name:     "t",
		Language: "en",
		Components: []eywa.TemplateComponent{
			{
				Type: "body",
				Parameters: []eywa.TemplateParameter{
					{Type: "text", Text: "hello"},
					{Type: "image", Image: "https://img.com/img.png"},
					{Type: "video", Video: "https://vid.com/v.mp4"},
					{Type: "unknown", Text: ""},
				},
			},
		},
	}
	vars := c.buildTwilioVariables(tmpl)
	if vars["1"] != "hello" {
		t.Errorf("expected '1'='hello', got %q", vars["1"])
	}
	if vars["2"] != "https://img.com/img.png" {
		t.Errorf("expected '2'=image URL, got %q", vars["2"])
	}
	if vars["3"] != "https://vid.com/v.mp4" {
		t.Errorf("expected '3'=video URL, got %q", vars["3"])
	}
	if vars["4"] != "N/D" {
		t.Errorf("expected '4'='N/D' for empty unknown, got %q", vars["4"])
	}
}

// --- safeParam ---

func TestSafeParam_NonEmpty(t *testing.T) {
	if got := safeParam("value"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}

func TestSafeParam_Empty(t *testing.T) {
	if got := safeParam(""); got != "N/D" {
		t.Errorf("expected 'N/D', got %q", got)
	}
}

func TestSafeParam_Whitespace(t *testing.T) {
	if got := safeParam("   "); got != "N/D" {
		t.Errorf("expected 'N/D' for whitespace, got %q", got)
	}
}

// --- classifyDeliveryFailure ---

func TestClassifyDeliveryFailure_PermanentCode_ReturnsBusinessError(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	permanentCodes := []int{21211, 21408, 21610, 21614, 63003, 63016}
	for _, code := range permanentCodes {
		err := c.classifyDeliveryFailure("failed", "invalid number", code)
		if err == nil {
			t.Fatalf("expected error for code %d", code)
		}
		// Should be a business error (not infrastructure)
		if !eywa.IsBusinessError(err) {
			t.Errorf("code %d: expected BusinessError, got %T: %v", code, err, err)
		}
	}
}

func TestClassifyDeliveryFailure_TransientCode_ReturnsInfraError(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	err := c.classifyDeliveryFailure("failed", "service down", 99999)
	if err == nil {
		t.Fatal("expected error")
	}
	if !eywa.IsInfrastructureError(err) {
		t.Errorf("expected InfrastructureError, got %T: %v", err, err)
	}
}

func TestClassifyDeliveryFailure_EmptyMessage_UsesStatusCode(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	err := c.classifyDeliveryFailure("undelivered", "", 99999)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- buildMessageParams ---

func TestBuildMessageParams_PlainText(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{FromNumber: "+15551234567"})
	params := c.buildMessageParams("+5511999999999", "Hello", map[string]any{})
	if params == nil {
		t.Fatal("expected non-nil params")
	}
}

func TestBuildMessageParams_WithImageURL(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{FromNumber: "+15551234567"})
	params := c.buildMessageParams("+5511999999999", "Look:", map[string]any{
		"image_url": "https://example.com/img.png",
	})
	if params == nil {
		t.Fatal("expected non-nil params")
	}
}

func TestBuildMessageParams_WithImageURL_NoMessage(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{FromNumber: "+15551234567"})
	params := c.buildMessageParams("+5511999999999", "", map[string]any{
		"image_url": "https://example.com/img.png",
	})
	if params == nil {
		t.Fatal("expected non-nil params")
	}
}

func TestBuildMessageParams_WithButtons(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{FromNumber: "+15551234567"})
	params := c.buildMessageParams("+5511999999999", "Choose:", map[string]any{
		"buttons": []any{
			map[string]any{"title": "Yes"},
			map[string]any{"title": "No"},
		},
	})
	if params == nil {
		t.Fatal("expected non-nil params")
	}
}

// --- classifyTwilioError ---

func TestClassifyTwilioError_KnownBusinessCodes(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	businessCodes := []int{21211, 21408, 21610, 21614}
	for _, code := range businessCodes {
		errJSON := fmt.Sprintf(`{"code":%d,"message":"msg"}`, code)
		err := c.classifyTwilioError(fmt.Errorf("%s", errJSON))
		if !eywa.IsBusinessError(err) {
			t.Errorf("code %d: expected BusinessError, got %T", code, err)
		}
	}
}

func TestClassifyTwilioError_KnownInfraCodes(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	infraCodes := []int{21910, 20003, 20429, 20500, 20503}
	for _, code := range infraCodes {
		errJSON := fmt.Sprintf(`{"code":%d,"message":"msg"}`, code)
		err := c.classifyTwilioError(fmt.Errorf("%s", errJSON))
		if !eywa.IsInfrastructureError(err) {
			t.Errorf("code %d: expected InfrastructureError, got %T", code, err)
		}
	}
}

func TestClassifyTwilioError_UnknownCode(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	err := c.classifyTwilioError(fmt.Errorf(`{"code":99999,"message":"unknown"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClassifyTwilioError_InvalidJSON(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	err := c.classifyTwilioError(fmt.Errorf("not json"))
	if !eywa.IsInfrastructureError(err) {
		t.Errorf("expected InfrastructureError for non-JSON, got %T", err)
	}
}

// --- resolveMediaURL ---

func TestResolveMediaURL_HTTPS(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123"})
	url, err := c.resolveMediaURL("https://example.com/media.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/media.jpg" {
		t.Errorf("expected original URL, got %q", url)
	}
}

func TestResolveMediaURL_SidFormat(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123"})
	url, err := c.resolveMediaURL("MM123/ME456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty URL for Sid/Sid format")
	}
}

func TestResolveMediaURL_InvalidFormat(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123"})
	_, err := c.resolveMediaURL("nodash")
	if err == nil {
		t.Fatal("expected error for invalid media ID format")
	}
}

// --- formatWhatsAppNumber ---

func TestFormatWhatsAppNumber_NoPrefix(t *testing.T) {
	if got := formatWhatsAppNumber("+15551234567"); got != "whatsapp:+15551234567" {
		t.Errorf("expected whatsapp: prefix, got %q", got)
	}
}

func TestFormatWhatsAppNumber_AlreadyPrefixed(t *testing.T) {
	if got := formatWhatsAppNumber("whatsapp:+15551234567"); got != "whatsapp:+15551234567" {
		t.Errorf("expected unchanged, got %q", got)
	}
}

// --- formatMessageWithButtons ---

func TestFormatMessageWithButtons_WithButtons(t *testing.T) {
	result := formatMessageWithButtons("Choose:", []any{
		map[string]any{"title": "Yes"},
		map[string]any{"title": "No"},
	})
	if result == "" || result == "Choose:" {
		t.Errorf("expected formatted message with buttons, got %q", result)
	}
}

func TestFormatMessageWithButtons_ButtonNotMap(t *testing.T) {
	// Graceful: non-map button entry → title is empty string
	result := formatMessageWithButtons("Choose:", []any{"not-a-map"})
	if result == "" {
		t.Error("expected non-empty result even for non-map button")
	}
}

// --- DownloadMedia (via httptest with https URL mock) ---

// newTLSDownloadClient creates a TLS test server and a TwilioClient whose httpClient
// trusts the server's self-signed certificate. resolveMediaURL passes-through URLs
// that already start with "https://", so passing srv.URL (https://...) works directly.
func newTLSDownloadClient(t *testing.T, handler http.HandlerFunc) (*TwilioClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "tok"})
	c.httpClient = srv.Client() // trusts server's self-signed cert
	return c, srv
}

func TestDownloadMedia_DirectURL_Success(t *testing.T) {
	c, srv := newTLSDownloadClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("IMG_DATA")) //nolint:errcheck
	})
	data, mime, err := c.DownloadMedia(context.Background(), srv.URL+"/file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "IMG_DATA" {
		t.Errorf("expected IMG_DATA, got %s", data)
	}
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
}

func TestDownloadMedia_DirectURL_NoContentType_DefaultsMime(t *testing.T) {
	c, srv := newTLSDownloadClient(t, func(w http.ResponseWriter, _ *http.Request) {
		// Setting header map to empty slice prevents Go from auto-sniffing Content-Type,
		// and the header won't be sent on the wire → client reads "" → defaults to octet-stream.
		w.Header()["Content-Type"] = []string{}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("DATA")) //nolint:errcheck
	})
	_, mime, err := c.DownloadMedia(context.Background(), srv.URL+"/file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mime != "application/octet-stream" {
		t.Errorf("expected application/octet-stream, got %s", mime)
	}
}

func TestDownloadMedia_HTTPErrorStatus_ReturnsError(t *testing.T) {
	c, srv := newTLSDownloadClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized")) //nolint:errcheck
	})
	_, _, err := c.DownloadMedia(context.Background(), srv.URL+"/file")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestDownloadMedia_NetworkError_ReturnsError(t *testing.T) {
	// No httpClient set → uses default &http.Client{Timeout: 30s} (covers nil branch).
	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "tok"})
	// https:// prefix → resolveMediaURL passes through, then network fails on port 1.
	_, _, err := c.DownloadMedia(context.Background(), "https://127.0.0.1:1/file")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestDownloadMedia_InvalidMediaID(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123"})
	_, _, err := c.DownloadMedia(context.Background(), "invalid-no-slash")
	if err == nil {
		t.Fatal("expected error for invalid media ID format")
	}
}

// --- Receptor ---

func newTwilioReceptor() eywa.Receptor {
	return NewWhatsAppTwilioInbound(nil)
}

// stubTwilioWAClient implements eywa.WhatsAppClient for receptor tests.
type stubTwilioWAClient struct {
	data     []byte
	mimeType string
	err      error
}

func (s *stubTwilioWAClient) SendMessage(_ context.Context, _, _ string, _ map[string]any) error {
	return nil
}
func (s *stubTwilioWAClient) SendTemplateMessage(_ context.Context, _ string, _ *eywa.TemplateMessage) error {
	return nil
}
func (s *stubTwilioWAClient) DownloadMedia(_ context.Context, _ string) ([]byte, string, error) {
	return s.data, s.mimeType, s.err
}

func TestTwilioReceptor_GetName(t *testing.T) {
	r := newTwilioReceptor()
	if r.GetName() != "whatsapp_twilio" {
		t.Errorf("unexpected name: %s", r.GetName())
	}
}

func TestTwilioReceptor_Convert_MissingMessageSid(t *testing.T) {
	r := newTwilioReceptor()
	_, err := r.Convert(context.Background(), "message", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing MessageSid")
	}
}

func TestTwilioReceptor_Convert_MissingFrom(t *testing.T) {
	r := newTwilioReceptor()
	_, err := r.Convert(context.Background(), "message", map[string]any{
		"MessageSid": "SM123",
	})
	if err == nil {
		t.Fatal("expected error for missing From")
	}
}

func TestTwilioReceptor_Convert_InvalidFromFormat(t *testing.T) {
	r := newTwilioReceptor()
	_, err := r.Convert(context.Background(), "message", map[string]any{
		"MessageSid": "SM123",
		"From":       "notaphone",
	})
	// extractPhoneNumber returns original for invalid phone → non-empty → no error
	_ = err
}

func TestTwilioReceptor_Convert_EmptyPhoneAfterStrip(t *testing.T) {
	// "whatsapp:" → trim prefix → "" → NormalizePhone("") fails → return "" → phoneNumber=="" error
	r := newTwilioReceptor()
	_, err := r.Convert(context.Background(), "message", map[string]any{
		"MessageSid": "SM123",
		"From":       "whatsapp:",
	})
	if err == nil {
		t.Fatal("expected error for empty phone after stripping whatsapp: prefix")
	}
}

func twilioPayload(from, body string, extra map[string]any) map[string]any {
	p := map[string]any{
		"MessageSid":  "SM" + body[:min(8, len(body))],
		"From":        "whatsapp:" + from,
		"To":          "whatsapp:+15551234567",
		"AccountSid":  "AC123",
		"ProfileName": "Test User",
		"WaId":        "5511999999999",
		"Body":        body,
	}
	for k, v := range extra {
		p[k] = v
	}
	return p
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestTwilioReceptor_Convert_TextMessage(t *testing.T) {
	r := newTwilioReceptor()
	payload := twilioPayload("+5511999999999", "Hello world", nil)
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

func TestTwilioReceptor_Convert_ButtonInteraction(t *testing.T) {
	r := newTwilioReceptor()
	payload := twilioPayload("+5511999999999", "Yes", map[string]any{
		"ButtonText":    "Yes",
		"ButtonPayload": "yes_action",
		"ButtonType":    "quick_reply",
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Fatal("expected pulse for button interaction")
	}
}

func TestTwilioReceptor_Convert_ButtonInteraction_DifferentText(t *testing.T) {
	// ButtonText different from Body → both added
	r := newTwilioReceptor()
	payload := twilioPayload("+5511999999999", "I confirm", map[string]any{
		"ButtonText":    "Yes, confirm",
		"ButtonPayload": "confirm",
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Fatal("expected pulse")
	}
}

func TestTwilioReceptor_Convert_MediaAttachment_WithClient(t *testing.T) {
	client := &stubTwilioWAClient{data: []byte("IMG"), mimeType: "image/jpeg"}
	r := NewWhatsAppTwilioInbound(client)
	payload := twilioPayload("+5511999999999", "", map[string]any{
		"NumMedia":          1,
		"MediaUrl0":         "https://api.twilio.com/media/img.jpg",
		"MediaContentType0": "image/jpeg",
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) == 0 {
		t.Error("expected attachment for media message")
	}
}

func TestTwilioReceptor_Convert_MediaAttachment_DownloadError(t *testing.T) {
	client := &stubTwilioWAClient{err: fmt.Errorf("download failed")}
	r := NewWhatsAppTwilioInbound(client)
	payload := twilioPayload("+5511999999999", "", map[string]any{
		"NumMedia":          "1",
		"MediaUrl0":         "https://api.twilio.com/media/img.jpg",
		"MediaContentType0": "image/jpeg",
	})
	// Download error is non-fatal
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Error("expected pulse even when download fails")
	}
}

func TestTwilioReceptor_Convert_MediaAttachment_NilClient(t *testing.T) {
	r := NewWhatsAppTwilioInbound(nil)
	payload := twilioPayload("+5511999999999", "", map[string]any{
		"NumMedia":          "1",
		"MediaUrl0":         "https://api.twilio.com/media/img.jpg",
		"MediaContentType0": "audio/ogg",
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) == 0 {
		t.Error("expected attachment even with nil client")
	}
}

func TestTwilioReceptor_Convert_MediaAttachment_MimeMerge(t *testing.T) {
	client := &stubTwilioWAClient{data: []byte("DATA"), mimeType: "image/webp"}
	r := NewWhatsAppTwilioInbound(client)
	payload := twilioPayload("+5511999999999", "", map[string]any{
		"NumMedia":          "1",
		"MediaUrl0":         "https://api.twilio.com/media/img.webp",
		"MediaContentType0": "", // no mime type in payload
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) == 0 {
		t.Fatal("expected attachment")
	}
	if pulses[0].Attachments[0].MimeType != "image/webp" {
		t.Errorf("expected merged mime type, got %q", pulses[0].Attachments[0].MimeType)
	}
}

func TestTwilioReceptor_Convert_MediaAttachment_EmptyURL(t *testing.T) {
	r := newTwilioReceptor()
	payload := twilioPayload("+5511999999999", "text", map[string]any{
		"NumMedia":  1,
		"MediaUrl0": "", // empty URL → no attachment
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses[0].Attachments) != 0 {
		t.Error("expected no attachment for empty URL")
	}
}

func TestTwilioReceptor_Convert_LocationMessage(t *testing.T) {
	r := newTwilioReceptor()
	payload := twilioPayload("+5511999999999", "", map[string]any{
		"Latitude":  float64(-23.5505),
		"Longitude": float64(-46.6333),
		"Address":   "São Paulo, SP",
		"Label":     "Office",
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pulses[0].UserMessage == "" {
		t.Error("expected location text in user message")
	}
}

func TestTwilioReceptor_Convert_LocationMessage_LabelOnly(t *testing.T) {
	r := newTwilioReceptor()
	payload := twilioPayload("+5511999999999", "", map[string]any{
		"Latitude":  float64(-23.5505),
		"Longitude": float64(-46.6333),
		"Label":     "Office",
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pulses[0].UserMessage == "" {
		t.Error("expected location text")
	}
}

func TestTwilioReceptor_Convert_LocationMessage_AddressOnly(t *testing.T) {
	r := newTwilioReceptor()
	payload := twilioPayload("+5511999999999", "", map[string]any{
		"Latitude":  float64(-23.5505),
		"Longitude": float64(-46.6333),
		"Address":   "São Paulo",
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pulses[0].UserMessage == "" {
		t.Error("expected location text")
	}
}

func TestTwilioReceptor_Convert_LocationMessage_CoordOnly(t *testing.T) {
	r := newTwilioReceptor()
	payload := twilioPayload("+5511999999999", "", map[string]any{
		"Latitude":  float64(-23.5505),
		"Longitude": float64(-46.6333),
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pulses[0].UserMessage == "" {
		t.Error("expected location text with just coordinates")
	}
}

func TestTwilioReceptor_Convert_Metadata_Forwarded(t *testing.T) {
	r := newTwilioReceptor()
	payload := twilioPayload("+5511999999999", "Hi", map[string]any{
		"Forwarded":           "true",
		"FrequentlyForwarded": "false",
	})
	pulses, err := r.Convert(context.Background(), "message", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pulses) == 0 {
		t.Fatal("expected pulse")
	}
}

// --- classifyMimeType ---

func TestClassifyMimeType(t *testing.T) {
	cases := []struct {
		mime string
		want eywa.ArtifactType
	}{
		{"image/jpeg", eywa.ArtifactTypeImage},
		{"audio/ogg", eywa.ArtifactTypeAudio},
		{"video/mp4", eywa.ArtifactTypeVideo},
		{"application/pdf", eywa.ArtifactTypeDocument},
		{"", eywa.ArtifactTypeDocument},
	}
	for _, tc := range cases {
		got := classifyMimeType(tc.mime)
		if got != tc.want {
			t.Errorf("classifyMimeType(%q) = %q, want %q", tc.mime, got, tc.want)
		}
	}
}

// --- formatLocationText ---

func TestFormatLocationText_AllFields(t *testing.T) {
	result := formatLocationText("Office", "123 Main St", -23.5, -46.6)
	if result == "" {
		t.Error("expected non-empty location text")
	}
}

func TestFormatLocationText_LabelOnly(t *testing.T) {
	result := formatLocationText("Office", "", -23.5, -46.6)
	if result == "" {
		t.Error("expected location text with label")
	}
}

func TestFormatLocationText_AddressOnly(t *testing.T) {
	result := formatLocationText("", "123 Main St", -23.5, -46.6)
	if result == "" {
		t.Error("expected location text with address")
	}
}

func TestFormatLocationText_CoordOnly(t *testing.T) {
	result := formatLocationText("", "", -23.5, -46.6)
	if result == "" {
		t.Error("expected location text with coordinates only")
	}
}
