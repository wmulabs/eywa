package openai

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

const structuredCompletion = `{"id":"1","object":"chat.completion","created":1,"model":"gpt","choices":[{"index":0,"message":{"role":"assistant","content":"{\"name\":\"ann\",\"age\":30}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

func personSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}, "age": map[string]any{"type": "integer"}},
		"required":   []any{"name"},
	}
}

// captureServer returns a server that records the last request body and replies with body.
func captureServer(t *testing.T, reply string, captured *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*captured = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
}

func TestGenerateResponse_StructuredOutput_SendsSchemaAndParses(t *testing.T) {
	var body string
	server := captureServer(t, structuredCompletion, &body)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	req := &eywa.OracleRequest{
		Model:          "gpt",
		Messages:       []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "who"}},
		ResponseFormat: &eywa.ResponseFormat{Name: "person", Schema: personSchema(), Strict: true},
	}

	resp, err := oracle.GenerateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, `"response_format"`) || !strings.Contains(body, `"json_schema"`) {
		t.Errorf("expected response_format json_schema in request, got %s", body)
	}
	if !strings.Contains(body, `"name":"person"`) || !strings.Contains(body, `"strict":true`) {
		t.Errorf("expected schema name and strict flag in request, got %s", body)
	}

	if len(resp.Structured) == 0 {
		t.Fatal("expected Structured to be populated for a structured request")
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
	server := captureServer(t, structuredCompletion, &body)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	req := &eywa.OracleRequest{Model: "gpt", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}

	resp, err := oracle.GenerateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Structured) != 0 {
		t.Errorf("expected empty Structured without ResponseFormat, got %s", resp.Structured)
	}
	if strings.Contains(body, "response_format") {
		t.Errorf("did not expect response_format in request, got %s", body)
	}
}

func TestGenerateResponse_StructuredOutput_ParseError(t *testing.T) {
	var body string
	server := captureServer(t, `{"id":"1","object":"chat.completion","created":1,"model":"gpt","choices":[]}`, &body)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	req := &eywa.OracleRequest{
		Model:          "gpt",
		Messages:       []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "who"}},
		ResponseFormat: &eywa.ResponseFormat{Name: "person", Schema: personSchema()},
	}

	if _, err := oracle.GenerateResponse(context.Background(), req); err == nil {
		t.Error("expected an error when the completion has no choices")
	}
}

func TestSupportsStructuredOutput(t *testing.T) {
	oracle := NewOracleWithConfig(Config{APIKey: "k"})
	if !oracle.SupportsStructuredOutput("gpt-4o") {
		t.Error("expected OpenAI oracle to report native structured-output support")
	}
}

func TestResponseFormatName_DefaultsWhenEmpty(t *testing.T) {
	var body string
	server := captureServer(t, structuredCompletion, &body)
	defer server.Close()

	oracle := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	req := &eywa.OracleRequest{
		Model:          "gpt",
		Messages:       []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "who"}},
		ResponseFormat: &eywa.ResponseFormat{Schema: personSchema()},
	}

	if _, err := oracle.GenerateResponse(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, `"name":"response"`) {
		t.Errorf("expected default schema name 'response', got %s", body)
	}
}
