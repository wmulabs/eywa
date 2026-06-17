package anthropic

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

const structuredMessage = `{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[{"type":"tool_use","id":"t1","name":"structured_output","input":{"name":"ann","age":30}}],"stop_reason":"tool_use","usage":{"input_tokens":5,"output_tokens":4}}`

func personSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}, "age": map[string]any{"type": "integer"}},
		"required":   []any{"name"},
	}
}

func captureServer(t *testing.T, reply string, status int, captured *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func structuredReq() *eywa.OracleRequest {
	return &eywa.OracleRequest{
		Model:          "claude",
		Messages:       []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "who"}},
		ResponseFormat: &eywa.ResponseFormat{Name: "person", Schema: personSchema(), Strict: true},
	}
}

func TestGenerateResponse_StructuredOutput_ForcesToolAndParses(t *testing.T) {
	var body string
	server := captureServer(t, structuredMessage, http.StatusOK, &body)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	resp, err := oracle.GenerateResponse(context.Background(), structuredReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, `"tool_choice"`) || !strings.Contains(body, `"structured_output"`) {
		t.Errorf("expected forced tool_choice on structured_output, got %s", body)
	}
	if !strings.Contains(body, `"input_schema"`) {
		t.Errorf("expected the schema as the tool input_schema, got %s", body)
	}

	if len(resp.Structured) == 0 {
		t.Fatal("expected Structured populated from the forced tool input")
	}
	var got map[string]any
	if err := json.Unmarshal(resp.Structured, &got); err != nil {
		t.Fatalf("Structured is not valid JSON: %v", err)
	}
	if got["name"] != "ann" {
		t.Errorf("expected parsed name 'ann', got %v", got["name"])
	}
}

func TestGenerateResponse_NoResponseFormat_LeavesStructuredEmpty(t *testing.T) {
	var body string
	reply := `{"id":"m","type":"message","role":"assistant","model":"claude","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`
	server := captureServer(t, reply, http.StatusOK, &body)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	req := &eywa.OracleRequest{Model: "claude", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	resp, err := oracle.GenerateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Structured) != 0 {
		t.Errorf("expected empty Structured without ResponseFormat, got %s", resp.Structured)
	}
	if strings.Contains(body, "tool_choice") {
		t.Errorf("did not expect tool_choice without ResponseFormat, got %s", body)
	}
}

func TestGenerateResponse_StructuredOutput_Error(t *testing.T) {
	var body string
	server := captureServer(t, "", http.StatusBadRequest, &body)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL, MaxRetries: 1})
	if _, err := oracle.GenerateResponse(context.Background(), structuredReq()); err == nil {
		t.Error("expected an error when the API rejects the request")
	}
}

func TestGenerateResponse_StructuredOutput_ParseError(t *testing.T) {
	var body string
	// tool_use input is an array, not an object -> parseToolUseBlock fails to unmarshal.
	reply := `{"id":"m","type":"message","role":"assistant","model":"claude","content":[{"type":"tool_use","id":"t1","name":"structured_output","input":[1,2]}],"stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1}}`
	server := captureServer(t, reply, http.StatusOK, &body)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	if _, err := oracle.GenerateResponse(context.Background(), structuredReq()); err == nil {
		t.Error("expected an error when the tool input is not a JSON object")
	}
}

func TestExtractStructured_NoMatchingTool_ReturnsNil(t *testing.T) {
	oracle := NewOracleWithConfig(Config{APIKey: "k"})
	calls := []eywa.OracleToolCall{{Name: "other", Arguments: map[string]any{"x": 1}}}
	if raw := oracle.extractStructured(calls); raw != nil {
		t.Errorf("expected nil when no structured_output tool present, got %s", raw)
	}
}

func TestSupportsStructuredOutput(t *testing.T) {
	oracle := NewOracleWithConfig(Config{APIKey: "k"})
	if !oracle.SupportsStructuredOutput("claude-3-5-sonnet") {
		t.Error("expected Anthropic oracle to report structured-output support")
	}
}
