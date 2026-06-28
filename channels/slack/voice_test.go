package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

func eventWithChannel(ch string) *eywa.Pulse {
	b := eywa.NewPulse(eywa.MemoryKey{Channel: "slack", User: "C1"}).WithSource("slack")
	if ch != "" {
		b = b.AddMetadata("slack_channel", ch)
	}
	return b.Build()
}

func TestVoice_SendResponse_Success(t *testing.T) {
	var gotChannel, gotText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing bearer")
		}
		body, _ := io.ReadAll(r.Body)
		var p map[string]any
		_ = json.Unmarshal(body, &p)
		gotChannel, _ = p["channel"].(string)
		gotText, _ = p["text"].(string)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient("tok")
	client.baseURL = srv.URL
	if err := NewVoice(client).SendResponse(context.Background(), eventWithChannel("C9"), "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotChannel != "C9" || gotText != "hello" {
		t.Errorf("server got channel=%q text=%q", gotChannel, gotText)
	}
}

func TestVoice_SendResponse_Errors(t *testing.T) {
	v := NewVoice(NewClient("tok"))
	if err := v.SendResponse(context.Background(), eventWithChannel("C1"), ""); err == nil {
		t.Error("expected error on empty response")
	}
	if err := v.SendResponse(context.Background(), eventWithChannel(""), "hi"); err == nil {
		t.Error("expected error when channel missing")
	}
}

func TestVoice_SendResponse_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
	}))
	defer srv.Close()
	client := NewClient("tok")
	client.baseURL = srv.URL
	if err := NewVoice(client).SendResponse(context.Background(), eventWithChannel("C1"), "hi"); err == nil {
		t.Error("expected error when Slack returns ok:false")
	}
}

func TestVoice_NameAndMetadata(t *testing.T) {
	v := NewVoice(NewClient("t"))
	if v.GetName() != "slack" || !v.ShouldAutoRespond() {
		t.Error("name/auto-respond")
	}
	if v.GetChannelMetadata(eventWithChannel("C1"))["channel"] != "slack" {
		t.Error("metadata channel")
	}
}
