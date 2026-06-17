package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

const textStreamNDJSON = `{"model":"llama3.1","message":{"role":"assistant","content":"Hel"},"done":false}

{"model":"llama3.1","message":{"role":"assistant","content":"lo"},"done":false}
{"model":"llama3.1","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}
`

const toolStreamNDJSON = `{"model":"llama3.1","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"lookup","arguments":{"q":"hi"}}}]},"done":false}
{"model":"llama3.1","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":3,"eval_count":2}
`

func ndjsonServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
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

func streamReq() *eywa.OracleRequest {
	return &eywa.OracleRequest{Model: "llama3.1", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
}

func TestGenerateStream_Text(t *testing.T) {
	server := ndjsonServer(t, textStreamNDJSON, http.StatusOK)
	defer server.Close()

	ch, err := NewOracle(Config{Host: server.URL}).GenerateStream(context.Background(), streamReq())
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
		t.Fatalf("expected Done with complete stop reason, got %+v", done)
	}
	if done.Usage.TotalTokens != 15 {
		t.Errorf("expected usage total 15, got %d", done.Usage.TotalTokens)
	}
}

func TestGenerateStream_ToolCall(t *testing.T) {
	server := ndjsonServer(t, toolStreamNDJSON, http.StatusOK)
	defer server.Close()

	ch, _ := NewOracle(Config{Host: server.URL}).GenerateStream(context.Background(), streamReq())
	deltas, done, gotErr := collectStream(ch)
	if gotErr != nil {
		t.Fatalf("stream error: %v", gotErr)
	}
	if len(deltas) != 0 {
		t.Errorf("tool-call stream emits no text deltas, got %v", deltas)
	}
	if done == nil || len(done.ToolCalls) != 1 || done.ToolCalls[0].Name != "lookup" {
		t.Fatalf("expected one assembled tool call 'lookup', got %+v", done)
	}
	if done.StopReason != eywa.StopReasonToolCalls {
		t.Errorf("expected tool-calls stop reason, got %q", done.StopReason)
	}
}

func TestGenerateStream_HTTPError(t *testing.T) {
	server := ndjsonServer(t, "", http.StatusInternalServerError)
	defer server.Close()

	ch, err := NewOracle(Config{Host: server.URL}).GenerateStream(context.Background(), streamReq())
	if err != nil {
		t.Fatalf("GenerateStream must not fail synchronously on HTTP status: %v", err)
	}
	if _, _, gotErr := collectStream(ch); gotErr == nil {
		t.Error("expected an error event for HTTP 500")
	}
}

func TestGenerateStream_DecodeError(t *testing.T) {
	server := ndjsonServer(t, "not json\n", http.StatusOK)
	defer server.Close()

	ch, _ := NewOracle(Config{Host: server.URL}).GenerateStream(context.Background(), streamReq())
	if _, _, gotErr := collectStream(ch); gotErr == nil {
		t.Error("expected a decode error for a malformed chunk")
	}
}

func TestGenerateStream_NoCompletion(t *testing.T) {
	// Chunks but never done:true → must surface an error, not hang.
	body := `{"model":"x","message":{"role":"assistant","content":"hi"},"done":false}` + "\n"
	server := ndjsonServer(t, body, http.StatusOK)
	defer server.Close()

	ch, _ := NewOracle(Config{Host: server.URL}).GenerateStream(context.Background(), streamReq())
	deltas, done, gotErr := collectStream(ch)
	if done != nil {
		t.Error("expected no Done event when the stream never completes")
	}
	if gotErr == nil {
		t.Error("expected an error when the stream ends without completion")
	}
	if strings.Join(deltas, "") != "hi" {
		t.Errorf("expected partial delta before the error, got %v", deltas)
	}
}

func TestGenerateStream_RequestError(t *testing.T) {
	ch, err := NewOracle(Config{Host: "http://127.0.0.1:0"}).GenerateStream(context.Background(), streamReq())
	if err == nil {
		if ch != nil {
			collectStream(ch)
		}
		t.Error("expected a synchronous request error against an unreachable host")
	}
}

func TestGenerateStream_BadHost(t *testing.T) {
	if _, err := NewOracle(Config{Host: "http://\x7f"}).GenerateStream(context.Background(), streamReq()); err == nil {
		t.Error("expected a build-request error for an invalid host")
	}
}
