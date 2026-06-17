package ollama

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

const structuredReply = `{"model":"llama3.1","message":{"role":"assistant","content":"{\"name\":\"ann\",\"age\":30}"},"done":true,"done_reason":"stop","prompt_eval_count":4,"eval_count":6}`

func personSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}, "age": map[string]any{"type": "integer"}},
		"required":   []any{"name"},
	}
}

func captureServer(t *testing.T, reply string, captured *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*captured = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
}

func TestGenerateResponse_StructuredOutput(t *testing.T) {
	var body string
	server := captureServer(t, structuredReply, &body)
	defer server.Close()

	oracle := NewOracle(Config{Host: server.URL})
	req := &eywa.OracleRequest{
		Model:          "llama3.1",
		Messages:       []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "who"}},
		ResponseFormat: &eywa.ResponseFormat{Name: "person", Schema: personSchema()},
	}

	resp, err := oracle.GenerateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, `"format"`) || !strings.Contains(body, `"properties"`) {
		t.Errorf("expected the schema in the request format field, got %s", body)
	}
	if len(resp.Structured) == 0 {
		t.Fatal("expected Structured populated for a structured request")
	}
	var got map[string]any
	if err := json.Unmarshal(resp.Structured, &got); err != nil {
		t.Fatalf("Structured is not valid JSON: %v", err)
	}
	if got["name"] != "ann" {
		t.Errorf("expected parsed name 'ann', got %v", got["name"])
	}
}

func TestGenerateResponse_NoResponseFormat_NoStructured(t *testing.T) {
	var body string
	server := captureServer(t, textReply, &body)
	defer server.Close()

	oracle := NewOracle(Config{Host: server.URL})
	req := &eywa.OracleRequest{Model: "llama3.1", Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}

	resp, err := oracle.GenerateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Structured) != 0 {
		t.Errorf("expected empty Structured without ResponseFormat, got %s", resp.Structured)
	}
	if strings.Contains(body, `"format"`) {
		t.Errorf("did not expect a format field without ResponseFormat, got %s", body)
	}
}

func TestSupportsStructuredOutput(t *testing.T) {
	if !NewOracle(Config{}).SupportsStructuredOutput("llama3.1") {
		t.Error("expected Ollama oracle to report structured-output support")
	}
}
