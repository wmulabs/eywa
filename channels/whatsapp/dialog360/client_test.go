package dialog360

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// mockServer returns a test server that always responds with the given status and body.
func mockServer(t *testing.T, status int, body string) (*httptest.Server, *Dialog360Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body)) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	client := NewDialog360Client(Dialog360Config{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})
	return srv, client
}

// captureServer returns a server that records the last request body and responds with 200.
func captureServer(t *testing.T) (*httptest.Server, *[]byte, *Dialog360Client) {
	t.Helper()
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`)) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	client := NewDialog360Client(Dialog360Config{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})
	return srv, &captured, client
}

// --- NewDialog360Client ---

func TestNewDialog360Client_Defaults(t *testing.T) {
	c := NewDialog360Client(Dialog360Config{APIKey: "key"})
	if c.baseURL != defaultBaseURL {
		t.Errorf("expected default base URL, got %s", c.baseURL)
	}
	if c.httpClient == nil {
		t.Error("expected non-nil HTTP client")
	}
}

func TestNewDialog360Client_CustomHTTPClient(t *testing.T) {
	custom := &http.Client{}
	c := NewDialog360Client(Dialog360Config{APIKey: "key", HTTPClient: custom})
	if c.httpClient != custom {
		t.Error("expected custom HTTP client to be used")
	}
}

// --- SendMessage ---

func TestSendMessage_Success(t *testing.T) {
	_, client := mockServer(t, http.StatusOK, `{}`)
	err := client.SendMessage(context.Background(), "+5511999999999", "Hello!", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessage_APIError_WithMessage(t *testing.T) {
	body := `{"error":{"message":"invalid recipient"}}`
	_, client := mockServer(t, http.StatusBadRequest, body)
	err := client.SendMessage(context.Background(), "+5511999999999", "Hello!", nil)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestSendMessage_APIError_WithoutMessage(t *testing.T) {
	_, client := mockServer(t, http.StatusInternalServerError, "internal error")
	err := client.SendMessage(context.Background(), "+5511999999999", "Hello!", nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSendMessage_HTTPClientError(t *testing.T) {
	client := NewDialog360Client(Dialog360Config{
		APIKey:  "key",
		BaseURL: "http://127.0.0.1:1", // unreachable
	})
	err := client.SendMessage(context.Background(), "+5511999999999", "Hello!", nil)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// --- SendMessage with different payload types ---

func TestSendMessage_WithButtons(t *testing.T) {
	_, client := mockServer(t, http.StatusOK, `{}`)
	meta := map[string]any{
		"buttons": []any{
			map[string]any{"title": "Yes", "id": "yes"},
			map[string]any{"title": "No"},
		},
	}
	if err := client.SendMessage(context.Background(), "+5511999999999", "Choose:", meta); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessage_WithButtons_NoID(t *testing.T) {
	_, client := mockServer(t, http.StatusOK, `{}`)
	meta := map[string]any{
		"buttons": []any{
			map[string]any{"title": "Option A"},
		},
	}
	if err := client.SendMessage(context.Background(), "+5511999999999", "Pick one:", meta); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessage_WithImageURL(t *testing.T) {
	_, client := mockServer(t, http.StatusOK, `{}`)
	meta := map[string]any{"image_url": "https://example.com/img.png"}
	if err := client.SendMessage(context.Background(), "+5511999999999", "Look:", meta); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessage_WithImageURL_NoCaption(t *testing.T) {
	_, client := mockServer(t, http.StatusOK, `{}`)
	meta := map[string]any{"image_url": "https://example.com/img.png"}
	if err := client.SendMessage(context.Background(), "+5511999999999", "", meta); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessage_NilMetadata(t *testing.T) {
	_, client := mockServer(t, http.StatusOK, `{}`)
	if err := client.SendMessage(context.Background(), "+5511999999999", "Hi", nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- SendTemplateMessage ---

func TestSendTemplateMessage_Success_NoComponents(t *testing.T) {
	_, client := mockServer(t, http.StatusOK, `{}`)
	tmpl := &eywa.TemplateMessage{Name: "hello_world", Language: "en"}
	if err := client.SendTemplateMessage(context.Background(), "+5511999999999", tmpl); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendTemplateMessage_WithComponents(t *testing.T) {
	_, client := mockServer(t, http.StatusOK, `{}`)
	tmpl := &eywa.TemplateMessage{
		Name:     "order_update",
		Language: "pt_BR",
		Components: []eywa.TemplateComponent{
			{
				Type: "body",
				Parameters: []eywa.TemplateParameter{
					{Type: "text", Text: "12345"},
					{Type: "image", Image: "https://img.com/img.png"},
					{Type: "video", Video: "https://vid.com/vid.mp4"},
					{Type: "unknown", Text: ""},
				},
			},
			{
				Type:    "button",
				SubType: "quick_reply",
				Index:   0,
				Parameters: []eywa.TemplateParameter{
					{Type: "text", Text: "Yes"},
				},
			},
		},
	}
	if err := client.SendTemplateMessage(context.Background(), "+5511999999999", tmpl); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendTemplateMessage_HTTPError(t *testing.T) {
	_, client := mockServer(t, http.StatusUnauthorized, `{"error":{"message":"unauthorized"}}`)
	tmpl := &eywa.TemplateMessage{Name: "hello_world", Language: "en"}
	if err := client.SendTemplateMessage(context.Background(), "+5511999999999", tmpl); err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestSendTemplateMessage_HTTPClientError(t *testing.T) {
	client := NewDialog360Client(Dialog360Config{
		APIKey:  "key",
		BaseURL: "http://127.0.0.1:1",
	})
	tmpl := &eywa.TemplateMessage{Name: "hello_world", Language: "en"}
	if err := client.SendTemplateMessage(context.Background(), "+5511999999999", tmpl); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// --- DownloadMedia ---

func TestDownloadMedia_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/media/media-id-123" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
				"url":       "https://lookaside.fbsbx.com/media/file.jpg",
				"mime_type": "image/jpeg",
			})
			return
		}
		// download request (after URL replacement)
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("JPEG_DATA")) //nolint:errcheck
	}))
	defer srv.Close()

	client := NewDialog360Client(Dialog360Config{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})

	data, mime, err := client.DownloadMedia(context.Background(), "media-id-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "JPEG_DATA" {
		t.Errorf("expected JPEG_DATA, got %s", string(data))
	}
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
}

func TestDownloadMedia_MediaInfoError(t *testing.T) {
	_, client := mockServer(t, http.StatusNotFound, `{"error":{"message":"not found"}}`)
	_, _, err := client.DownloadMedia(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("expected error for 404 media info")
	}
}

func TestDownloadMedia_HTTPClientError(t *testing.T) {
	client := NewDialog360Client(Dialog360Config{
		APIKey:  "key",
		BaseURL: "http://127.0.0.1:1",
	})
	_, _, err := client.DownloadMedia(context.Background(), "media-id-123")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestDownloadMedia_InvalidJSONResponse(t *testing.T) {
	_, client := mockServer(t, http.StatusOK, "not-json")
	_, _, err := client.DownloadMedia(context.Background(), "media-id")
	if err == nil {
		t.Fatal("expected error for invalid JSON media info")
	}
}

func TestDownloadMedia_DownloadRequestError(t *testing.T) {
	// Media info succeeds but download URL points to unreachable host.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"url":       "https://lookaside.fbsbx.com/media/file.jpg",
			"mime_type": "image/jpeg",
		})
	}))
	defer srv.Close()
	// Replace base URL with srv.URL so media info succeeds,
	// but the download URL gets srv.URL/media/file.jpg → returns 404.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/media/media-fail" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
				"url":       "https://lookaside.fbsbx.com/media/file.jpg",
				"mime_type": "image/jpeg",
			})
			return
		}
		// Download request → 403
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"message":"forbidden"}}`)) //nolint:errcheck
	}))
	defer srv2.Close()

	client := NewDialog360Client(Dialog360Config{
		APIKey:     "test-key",
		BaseURL:    srv2.URL,
		HTTPClient: srv2.Client(),
	})
	_, _, err := client.DownloadMedia(context.Background(), "media-fail")
	if err == nil {
		t.Fatal("expected error when download request returns non-2xx")
	}
}
