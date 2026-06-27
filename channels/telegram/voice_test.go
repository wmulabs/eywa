package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

func eventWithChatID(v any) *eywa.Pulse {
	return eywa.NewPulse(eywa.MemoryKey{Channel: "telegram", User: "1"}).
		WithSource("telegram").
		AddMetadata("telegram_chat_id", v).
		Build()
}

func TestVoice_SendResponse_Success(t *testing.T) {
	var gotChatID float64
	var gotText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/botTOKEN/sendMessage") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var p map[string]any
		_ = json.Unmarshal(body, &p)
		gotChatID, _ = p["chat_id"].(float64)
		gotText, _ = p["text"].(string)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient("TOKEN")
	client.baseURL = srv.URL
	v := NewVoice(client)

	if err := v.SendResponse(context.Background(), eventWithChatID(int64(555)), "hi there"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if int64(gotChatID) != 555 || gotText != "hi there" {
		t.Errorf("server got chat_id=%v text=%q", gotChatID, gotText)
	}
}

func TestVoice_SendResponse_Errors(t *testing.T) {
	v := NewVoice(NewClient("TOKEN"))
	if err := v.SendResponse(context.Background(), eventWithChatID(int64(1)), ""); err == nil {
		t.Error("expected error on empty response")
	}
	noChat := eywa.NewPulse(eywa.MemoryKey{Channel: "telegram", User: "1"}).Build()
	if err := v.SendResponse(context.Background(), noChat, "hi"); err == nil {
		t.Error("expected error when chat id missing")
	}
}

func TestVoice_SendResponse_ClientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer srv.Close()

	client := NewClient("TOKEN")
	client.baseURL = srv.URL
	if err := NewVoice(client).SendResponse(context.Background(), eventWithChatID(int64(1)), "hi"); err == nil {
		t.Error("expected error when the Bot API rejects the send")
	}
}

func TestChatIDFromEvent_NilSafe(t *testing.T) {
	if _, ok := chatIDFromEvent(nil); ok {
		t.Error("nil event must not yield a chat id")
	}
	noMeta := eywa.NewPulse(eywa.MemoryKey{Channel: "telegram", User: "1"}).Build()
	if _, ok := chatIDFromEvent(noMeta); ok {
		t.Error("event without metadata must not yield a chat id")
	}
}

func TestChatIDFromEvent_Types(t *testing.T) {
	for _, v := range []any{int64(7), float64(7), int(7)} {
		got, ok := chatIDFromEvent(eventWithChatID(v))
		if !ok || got != 7 {
			t.Errorf("for %T: got %d ok=%v", v, got, ok)
		}
	}
	if _, ok := chatIDFromEvent(eventWithChatID("nope")); ok {
		t.Error("string chat id must not parse")
	}
}

func TestVoice_NameAndMetadata(t *testing.T) {
	v := NewVoice(NewClient("t"))
	if v.GetName() != "telegram" || !v.ShouldAutoRespond() {
		t.Error("name/auto-respond")
	}
	md := v.GetChannelMetadata(eventWithChatID(int64(1)))
	if md["channel"] != "telegram" {
		t.Errorf("metadata channel = %v", md["channel"])
	}
}
