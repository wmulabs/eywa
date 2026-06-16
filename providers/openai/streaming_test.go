package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

const textStreamSSE = `data: {"id":"1","object":"chat.completion.chunk","created":1,"model":"gpt","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"},"finish_reason":null}]}

data: {"id":"1","object":"chat.completion.chunk","created":1,"model":"gpt","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":null}]}

data: {"id":"1","object":"chat.completion.chunk","created":1,"model":"gpt","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"1","object":"chat.completion.chunk","created":1,"model":"gpt","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

data: [DONE]

`

const emptyChoicesSSE = `data: {"id":"1","object":"chat.completion.chunk","created":1,"model":"gpt","choices":[]}

data: [DONE]

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

func streamOracle(url string) *OpenAIOracle {
	return NewOracleWithConfig(Config{APIKey: "k", BaseURL: url})
}

func streamReq() *eywa.OracleRequest {
	return &eywa.OracleRequest{Model: "gpt", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
}

func TestGenerateStream_TextResponse(t *testing.T) {
	server := sseServer(t, textStreamSSE, http.StatusOK)
	defer server.Close()

	ch, err := streamOracle(server.URL).GenerateStream(context.Background(), streamReq())
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

func TestGenerateStream_HTTPError_EmitsError(t *testing.T) {
	server := sseServer(t, "", http.StatusInternalServerError)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL, MaxRetries: 1})
	ch, err := oracle.GenerateStream(context.Background(), streamReq())
	if err != nil {
		t.Fatalf("GenerateStream must not fail synchronously: %v", err)
	}
	if _, _, gotErr := collectStream(ch); gotErr == nil {
		t.Error("expected an error event for an HTTP 500 stream")
	}
}

func TestGenerateStream_NoChoices_EmitsError(t *testing.T) {
	server := sseServer(t, emptyChoicesSSE, http.StatusOK)
	defer server.Close()

	ch, _ := streamOracle(server.URL).GenerateStream(context.Background(), streamReq())
	if _, _, gotErr := collectStream(ch); gotErr == nil {
		t.Error("expected an error event when the assembled completion has no choices")
	}
}
