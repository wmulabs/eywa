package gemini

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

const structuredResponse = `{"candidates":[{"content":{"parts":[{"text":"{\"name\":\"ann\",\"age\":30}"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`

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

func structuredReq() *eywa.OracleRequest {
	return &eywa.OracleRequest{
		Model:          "gemini",
		SystemPrompt:   "who is it",
		ResponseFormat: &eywa.ResponseFormat{Name: "person", Schema: personSchema(), Strict: true},
	}
}

func TestGenerateResponse_StructuredOutput_SendsSchemaAndParses(t *testing.T) {
	var body string
	server := captureServer(t, structuredResponse, &body)
	defer server.Close()

	oracle, err := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("creating oracle: %v", err)
	}

	resp, err := oracle.GenerateResponse(context.Background(), structuredReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, "responseJsonSchema") {
		t.Errorf("expected responseJsonSchema in request, got %s", body)
	}
	if !strings.Contains(body, "application/json") {
		t.Errorf("expected responseMimeType application/json, got %s", body)
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

func TestGenerateResponse_NoResponseFormat_LeavesStructuredEmpty(t *testing.T) {
	var body string
	server := captureServer(t, structuredResponse, &body)
	defer server.Close()

	oracle, err := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("creating oracle: %v", err)
	}

	req := &eywa.OracleRequest{Model: "gemini", SystemPrompt: "hi"}
	resp, err := oracle.GenerateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Structured) != 0 {
		t.Errorf("expected empty Structured without ResponseFormat, got %s", resp.Structured)
	}
	if strings.Contains(body, "responseJsonSchema") {
		t.Errorf("did not expect responseJsonSchema in request, got %s", body)
	}
}

func TestGenerateResponse_StructuredOutput_ChatPath(t *testing.T) {
	var body string
	server := captureServer(t, structuredResponse, &body)
	defer server.Close()

	oracle, err := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("creating oracle: %v", err)
	}

	req := &eywa.OracleRequest{
		Model:          "gemini",
		Messages:       []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "who is it"}},
		ResponseFormat: &eywa.ResponseFormat{Name: "person", Schema: personSchema()},
	}
	resp, err := oracle.GenerateResponse(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Structured) == 0 {
		t.Fatal("expected Structured populated on the chat path")
	}
	if !strings.Contains(body, "responseJsonSchema") {
		t.Errorf("expected responseJsonSchema in request, got %s", body)
	}
}

func TestGenerateResponse_StructuredOutput_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	oracle, err := NewOracleWithConfig(Config{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("creating oracle: %v", err)
	}

	if _, err := oracle.GenerateResponse(context.Background(), structuredReq()); err == nil {
		t.Error("expected an error when the API rejects the request")
	}
}

func TestSupportsStructuredOutput(t *testing.T) {
	oracle, err := NewOracleWithConfig(Config{APIKey: "k"})
	if err != nil {
		t.Fatalf("creating oracle: %v", err)
	}
	if !oracle.SupportsStructuredOutput("gemini-1.5-pro") {
		t.Error("expected Gemini oracle to report native structured-output support")
	}
}
