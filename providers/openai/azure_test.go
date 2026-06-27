package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

func TestNewAzureOracle_Config(t *testing.T) {
	o := NewAzureOracle("https://res.openai.azure.com", "k", "2024-10-21")
	if o.GetName() != ProviderNameAzure {
		t.Errorf("name = %q, want %q", o.GetName(), ProviderNameAzure)
	}
	if o.config.Endpoint != "https://res.openai.azure.com" || o.config.APIVersion != "2024-10-21" {
		t.Errorf("unexpected config: %+v", o.config)
	}
}

const azureChatReply = `{"id":"1","object":"chat.completion","created":1,"model":"dep","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

func TestAzureOracle_RequestShape(t *testing.T) {
	var path, query, apiKey, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		apiKey, auth = r.Header.Get("api-key"), r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(azureChatReply))
	}))
	defer srv.Close()

	oracle := NewAzureOracle(srv.URL, "secret-key", "2024-10-21")
	resp, err := oracle.GenerateResponse(context.Background(), &eywa.OracleRequest{
		Model:    "my-deployment",
		Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi?"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hi" {
		t.Errorf("content = %q, want hi", resp.Content)
	}
	if !strings.Contains(path, "/openai/deployments/my-deployment/chat/completions") {
		t.Errorf("path = %q, want deployment-based Azure path", path)
	}
	if !strings.Contains(query, "api-version=2024-10-21") {
		t.Errorf("query = %q, want api-version", query)
	}
	if apiKey != "secret-key" {
		t.Errorf("api-key header = %q, want secret-key", apiKey)
	}
	if auth != "" {
		t.Errorf("Azure must use the api-key header, not Authorization bearer (got %q)", auth)
	}
}

func TestAzureOracle_DefaultAPIVersion(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(azureChatReply))
	}))
	defer srv.Close()

	oracle := NewAzureOracle(srv.URL, "k", "") // empty -> default
	if _, err := oracle.GenerateResponse(context.Background(), &eywa.OracleRequest{
		Model:    "dep",
		Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "x"}},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "api-version="+defaultAzureAPIVersion) {
		t.Errorf("query = %q, want default api-version %s", query, defaultAzureAPIVersion)
	}
}
