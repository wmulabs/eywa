package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

const textStreamSSE = `event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}

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

func TestGenerateStream_TextResponse(t *testing.T) {
	server := sseServer(t, textStreamSSE, http.StatusOK)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	ch, err := oracle.GenerateStream(context.Background(), &eywa.OracleRequest{Model: "claude", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deltas, done, gotErr := collectStream(ch)
	if gotErr != nil {
		t.Fatalf("stream error: %v", gotErr)
	}
	if strings.Join(deltas, "") != "Hello world" {
		t.Errorf("expected deltas to assemble to 'Hello world', got %v", deltas)
	}
	if done == nil {
		t.Fatal("expected a Done event")
	}
	if done.StopReason != eywa.StopReasonComplete {
		t.Errorf("expected normalized stop reason %q, got %q", eywa.StopReasonComplete, done.StopReason)
	}
	if done.Usage.PromptTokens != 10 {
		t.Errorf("expected input tokens accounted (10), got %d", done.Usage.PromptTokens)
	}
}

func TestGenerateStream_HTTPError_EmitsError(t *testing.T) {
	server := sseServer(t, "", http.StatusInternalServerError)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL, MaxRetries: 1})
	ch, err := oracle.GenerateStream(context.Background(), &eywa.OracleRequest{Model: "claude", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("GenerateStream must not fail synchronously: %v", err)
	}
	if _, _, gotErr := collectStream(ch); gotErr == nil {
		t.Error("expected an error event for an HTTP 500 stream")
	}
}

func toolUseSSE(partialJSON string) string {
	return `event: message_start
data: {"type":"message_start","message":{"id":"m","type":"message","role":"assistant","model":"claude","content":[],"stop_reason":null,"usage":{"input_tokens":5,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"lookup","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"` + partialJSON + `"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":4}}

event: message_stop
data: {"type":"message_stop"}

`
}

func TestGenerateStream_ToolUse_AssemblesToolCallOnDone(t *testing.T) {
	server := sseServer(t, toolUseSSE(`{\"q\":\"hi\"}`), http.StatusOK)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	ch, _ := oracle.GenerateStream(context.Background(), &eywa.OracleRequest{Model: "claude", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}})

	deltas, done, gotErr := collectStream(ch)
	if gotErr != nil {
		t.Fatalf("stream error: %v", gotErr)
	}
	if len(deltas) != 0 {
		t.Errorf("tool-use produces no text deltas, got %v", deltas)
	}
	if done == nil || len(done.ToolCalls) != 1 || done.ToolCalls[0].Name != "lookup" {
		t.Errorf("expected one assembled tool call 'lookup' on Done, got %+v", done)
	}
}

func TestGenerateStream_MalformedToolJSON_EmitsError(t *testing.T) {
	// Not valid JSON at all -> the SDK's accumulate rejects it.
	server := sseServer(t, toolUseSSE(`not-json`), http.StatusOK)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	ch, _ := oracle.GenerateStream(context.Background(), &eywa.OracleRequest{Model: "claude", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}})

	if _, _, gotErr := collectStream(ch); gotErr == nil {
		t.Error("expected an error event when the streamed tool JSON is malformed")
	}
}

func TestGenerateStream_NonObjectToolInput_EmitsError(t *testing.T) {
	// Valid JSON but not an object -> accumulate succeeds, parseToolUseBlock fails to unmarshal.
	server := sseServer(t, toolUseSSE(`[1,2]`), http.StatusOK)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	ch, _ := oracle.GenerateStream(context.Background(), &eywa.OracleRequest{Model: "claude", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}})

	if _, _, gotErr := collectStream(ch); gotErr == nil {
		t.Error("expected an error event when the assembled tool input is not a JSON object")
	}
}
