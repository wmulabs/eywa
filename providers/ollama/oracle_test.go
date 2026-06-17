package ollama

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
)

const textReply = `{"model":"llama3.1","message":{"role":"assistant","content":"Hello there"},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}`

const toolReply = `{"model":"llama3.1","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"lookup","arguments":{"q":"hi"}}}]},"done":true,"done_reason":"stop","prompt_eval_count":3,"eval_count":2}`

func chatServer(t *testing.T, reply string, status int, captured *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected POST /api/chat, got %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		*captured = string(b)
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
}

func TestGenerateResponse_Text(t *testing.T) {
	var body string
	server := chatServer(t, textReply, http.StatusOK, &body)
	defer server.Close()

	oracle := NewOracle(Config{Host: server.URL})
	req := &eywa.OracleRequest{
		Model:        "llama3.1",
		SystemPrompt: "be brief",
		Messages:     []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}},
		Temperature:  0.5,
		MaxTokens:    64,
	}

	resp, err := oracle.GenerateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello there" {
		t.Errorf("expected content, got %q", resp.Content)
	}
	if resp.StopReason != eywa.StopReasonComplete {
		t.Errorf("expected complete stop reason, got %q", resp.StopReason)
	}
	if resp.TokensUsed.PromptTokens != 10 || resp.TokensUsed.CompletionTokens != 5 || resp.TokensUsed.TotalTokens != 15 {
		t.Errorf("unexpected usage: %+v", resp.TokensUsed)
	}
	if !strings.Contains(body, `"be brief"`) || !strings.Contains(body, `"num_predict":64`) {
		t.Errorf("expected system prompt and options in request, got %s", body)
	}
}

func TestGenerateResponse_ToolCall(t *testing.T) {
	var body string
	server := chatServer(t, toolReply, http.StatusOK, &body)
	defer server.Close()

	oracle := NewOracle(Config{Host: server.URL})
	req := &eywa.OracleRequest{
		Model:    "llama3.1",
		Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "look up"}},
		UseTools: true,
		Tools:    []eywa.OracleTool{{Name: "lookup", Description: "find", Parameters: map[string]any{"type": "object"}}},
	}

	resp, err := oracle.GenerateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "lookup" {
		t.Fatalf("expected one tool call 'lookup', got %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].Arguments["q"] != "hi" {
		t.Errorf("expected tool arguments, got %v", resp.ToolCalls[0].Arguments)
	}
	if resp.StopReason != eywa.StopReasonToolCalls {
		t.Errorf("expected tool-calls stop reason, got %q", resp.StopReason)
	}
	if !strings.Contains(body, `"tools"`) || !strings.Contains(body, `"lookup"`) {
		t.Errorf("expected tools in request, got %s", body)
	}
}

func TestGenerateResponse_PropagatesAssistantToolCallsInHistory(t *testing.T) {
	var body string
	server := chatServer(t, textReply, http.StatusOK, &body)
	defer server.Close()

	oracle := NewOracle(Config{Host: server.URL})
	req := &eywa.OracleRequest{
		Model: "llama3.1",
		Messages: []eywa.OracleMessage{
			{Role: eywa.RoleAssistant, ToolCalls: []eywa.OracleToolCall{{Name: "lookup", Arguments: map[string]any{"q": "x"}}}},
			{Role: eywa.RoleTool, ToolName: "lookup", Content: "result"},
		},
	}
	if _, err := oracle.GenerateResponse(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, `"tool_calls"`) || !strings.Contains(body, `"result"`) {
		t.Errorf("expected assistant tool_calls and tool result in request, got %s", body)
	}
}

func TestGenerateResponse_HTTPError(t *testing.T) {
	var body string
	server := chatServer(t, "", http.StatusInternalServerError, &body)
	defer server.Close()

	oracle := NewOracle(Config{Host: server.URL})
	req := &eywa.OracleRequest{Model: "llama3.1", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	if _, err := oracle.GenerateResponse(context.Background(), req); err == nil {
		t.Error("expected an error on HTTP 500")
	}
}

func TestGenerateResponse_DecodeError(t *testing.T) {
	var body string
	server := chatServer(t, "not json", http.StatusOK, &body)
	defer server.Close()

	oracle := NewOracle(Config{Host: server.URL})
	req := &eywa.OracleRequest{Model: "llama3.1", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	if _, err := oracle.GenerateResponse(context.Background(), req); err == nil {
		t.Error("expected a decode error on malformed JSON")
	}
}

func TestGenerateResponse_RequestError(t *testing.T) {
	oracle := NewOracle(Config{Host: "http://127.0.0.1:0"})
	req := &eywa.OracleRequest{Model: "llama3.1", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	if _, err := oracle.GenerateResponse(context.Background(), req); err == nil {
		t.Error("expected a request error against an unreachable host")
	}
}

func TestGenerateResponse_AllSamplingOptions(t *testing.T) {
	var body string
	server := chatServer(t, textReply, http.StatusOK, &body)
	defer server.Close()

	oracle := NewOracle(Config{Host: server.URL})
	req := &eywa.OracleRequest{
		Model:         "llama3.1",
		Messages:      []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}},
		Temperature:   0.7,
		MaxTokens:     32,
		TopP:          0.9,
		TopK:          40,
		StopSequences: []string{"END"},
	}
	if _, err := oracle.GenerateResponse(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{`"top_p":0.9`, `"top_k":40`, `"stop":["END"]`, `"temperature":0.7`} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %s in options, got %s", want, body)
		}
	}
}

func TestGenerateResponse_BadHost_BuildRequestError(t *testing.T) {
	oracle := NewOracle(Config{Host: "http://\x7f"})
	req := &eywa.OracleRequest{Model: "llama3.1", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	if _, err := oracle.GenerateResponse(context.Background(), req); err == nil {
		t.Error("expected a build-request error for an invalid host")
	}
}

func TestNormalizeStopReason(t *testing.T) {
	cases := map[string]string{
		"length":  eywa.StopReasonLength,
		"stop":    eywa.StopReasonComplete,
		"":        eywa.StopReasonComplete,
		"unknown": eywa.StopReasonComplete,
	}
	for in, want := range cases {
		if got := normalizeStopReason(in); got != want {
			t.Errorf("normalizeStopReason(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewOracle_Defaults(t *testing.T) {
	o := NewOracle(Config{})
	if o.config.Host != defaultHost {
		t.Errorf("expected default host, got %q", o.config.Host)
	}
	if o.config.Timeout != defaultTimeout {
		t.Errorf("expected default timeout, got %v", o.config.Timeout)
	}
	if o.GetName() != ProviderName {
		t.Errorf("expected default name, got %q", o.GetName())
	}
}

func TestOracleMetadata(t *testing.T) {
	o := NewOracle(Config{Name: "local", Host: "http://h:11434", Timeout: time.Second, KeepAlive: "5m"})
	if !o.IsAvailable() {
		t.Error("expected IsAvailable true when host set")
	}
	if o.GetName() != "local" {
		t.Errorf("unexpected name %q", o.GetName())
	}
	if o.GetAvailableModels() != nil {
		t.Error("expected nil models")
	}
	if cfg := o.GetConfig(); cfg["host"] != "http://h:11434" || cfg["keep_alive"] != "5m" {
		t.Errorf("unexpected config %v", cfg)
	}
	if o.SupportsImages("llava") || o.SupportsAudio("x") || o.SupportsDocuments("x") {
		t.Error("expected media support false in this version")
	}
}

func TestIsAvailable_FalseWhenNoHost(t *testing.T) {
	// Force an empty host to exercise the negative branch (NewOracle would default it).
	o := &OllamaOracle{config: Config{}}
	if o.IsAvailable() {
		t.Error("expected IsAvailable false with empty host")
	}
}
