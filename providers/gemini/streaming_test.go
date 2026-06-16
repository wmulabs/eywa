package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

const textStreamSSE = `data: {"candidates":[{"content":{"parts":[{"text":"Hel"}],"role":"model"}}]}

data: {"candidates":[{"content":{"parts":[{"text":"lo"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2,"totalTokenCount":12}}

`

const toolCallStreamSSE = `data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"lookup","args":{"q":"hi"}}}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}

`

func sseServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
}

func collectStream(ch <-chan eywa.StreamEvent) (deltas []string, done *eywa.StreamEvent, gotErr error) {
	for ev := range ch {
		switch ev.Type {
		case eywa.StreamEventDelta:
			deltas = append(deltas, ev.Delta)
		case eywa.StreamEventDone:
			d := ev
			done = &d
		case eywa.StreamEventError:
			gotErr = ev.Err
		}
	}
	return
}

func streamOracle(t *testing.T, url string) *GeminiOracle {
	t.Helper()
	oracle, err := NewOracleWithConfig(Config{APIKey: "k", BaseURL: url})
	if err != nil {
		t.Fatalf("creating oracle: %v", err)
	}
	return oracle
}

func TestGenerateStream_ChatPath_AssemblesText(t *testing.T) {
	server := sseServer(t, textStreamSSE, http.StatusOK)
	defer server.Close()

	req := &eywa.OracleRequest{Model: "gemini", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	ch, err := streamOracle(t, server.URL).GenerateStream(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deltas, done, gotErr := collectStream(ch)
	if gotErr != nil {
		t.Fatalf("stream error: %v", gotErr)
	}
	if strings.Join(deltas, "") != "Hello" {
		t.Errorf("expected deltas to assemble to 'Hello', got %v", deltas)
	}
	if done == nil || done.StopReason != eywa.StopReasonComplete {
		t.Errorf("expected Done with normalized stop reason, got %+v", done)
	}
	if done.Usage.TotalTokens != 12 {
		t.Errorf("expected usage total 12, got %d", done.Usage.TotalTokens)
	}
}

func TestGenerateStream_SimplePath_NoHistory(t *testing.T) {
	server := sseServer(t, textStreamSSE, http.StatusOK)
	defer server.Close()

	req := &eywa.OracleRequest{Model: "gemini", SystemPrompt: "hi"}
	ch, err := streamOracle(t, server.URL).GenerateStream(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deltas, done, gotErr := collectStream(ch)
	if gotErr != nil {
		t.Fatalf("stream error: %v", gotErr)
	}
	if strings.Join(deltas, "") != "Hello" {
		t.Errorf("expected deltas to assemble to 'Hello', got %v", deltas)
	}
	if done == nil {
		t.Fatal("expected a Done event")
	}
}

func TestGenerateStream_ToolCall_AssemblesOnDone(t *testing.T) {
	server := sseServer(t, toolCallStreamSSE, http.StatusOK)
	defer server.Close()

	req := &eywa.OracleRequest{Model: "gemini", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	ch, err := streamOracle(t, server.URL).GenerateStream(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deltas, done, gotErr := collectStream(ch)
	if gotErr != nil {
		t.Fatalf("stream error: %v", gotErr)
	}
	if len(deltas) != 0 {
		t.Errorf("tool-call stream produces no text deltas, got %v", deltas)
	}
	if done == nil || len(done.ToolCalls) != 1 || done.ToolCalls[0].Name != "lookup" {
		t.Fatalf("expected one assembled tool call 'lookup' on Done, got %+v", done)
	}
	if done.StopReason != eywa.StopReasonToolCalls {
		t.Errorf("expected stop reason %q, got %q", eywa.StopReasonToolCalls, done.StopReason)
	}
}

func TestGenerateStream_HTTPError_EmitsError(t *testing.T) {
	server := sseServer(t, "", http.StatusBadRequest)
	defer server.Close()

	req := &eywa.OracleRequest{Model: "gemini", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	ch, err := streamOracle(t, server.URL).GenerateStream(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateStream must not fail synchronously: %v", err)
	}
	if _, _, gotErr := collectStream(ch); gotErr == nil {
		t.Error("expected an error event for an HTTP error stream")
	}
}

func TestGenerateStream_NoResponse_EmitsError(t *testing.T) {
	server := sseServer(t, "", http.StatusOK)
	defer server.Close()

	req := &eywa.OracleRequest{Model: "gemini", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	ch, err := streamOracle(t, server.URL).GenerateStream(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateStream must not fail synchronously: %v", err)
	}
	if _, _, gotErr := collectStream(ch); gotErr == nil {
		t.Error("expected an error event when the stream yields no response")
	}
}
