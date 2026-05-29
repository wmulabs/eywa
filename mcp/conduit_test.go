package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
)

// --- helpers ---

// newMCPConduit creates a conduit with URL validation disabled, allowing test
// servers bound to loopback addresses to be used without SSRF rejection.
func newMCPConduit(t *testing.T, url string) *MCPConduit {
	t.Helper()
	c := NewConduit(ConduitConfig{
		Name:      "test-mcp",
		Transport: "http",
		URL:       url,
	}).(*MCPConduit)
	c.urlValidator = func(string) error { return nil }
	return c
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func mcpOK(id int, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func mcpErr(id int, code int, msg string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0", "id": id,
		"error": map[string]any{"code": code, "message": msg},
	}
}

func toolsListResult(tools ...map[string]any) map[string]any {
	return map[string]any{"tools": tools}
}

// --- NewConduit ---

func TestNewConduit_DefaultTimeout(t *testing.T) {
	c := NewConduit(ConduitConfig{Name: "x"}).(*MCPConduit)
	if c.cfg.Timeout != 30*time.Second {
		t.Errorf("expected 30s default timeout, got %v", c.cfg.Timeout)
	}
}

func TestNewConduit_CustomTimeout(t *testing.T) {
	c := NewConduit(ConduitConfig{Name: "x", Timeout: 5 * time.Second}).(*MCPConduit)
	if c.cfg.Timeout != 5*time.Second {
		t.Errorf("expected 5s, got %v", c.cfg.Timeout)
	}
}

func TestNewConduit_ClientNotNil(t *testing.T) {
	c := NewConduit(ConduitConfig{Name: "x"}).(*MCPConduit)
	if c.client == nil {
		t.Error("expected non-nil http.Client")
	}
}

// --- GetName / Close ---

func TestGetName(t *testing.T) {
	c := NewConduit(ConduitConfig{Name: "my-mcp"}).(*MCPConduit)
	if c.GetName() != "my-mcp" {
		t.Errorf("expected my-mcp, got %s", c.GetName())
	}
}

func TestClose_ReturnsNil(t *testing.T) {
	c := NewConduit(ConduitConfig{Name: "x"}).(*MCPConduit)
	if err := c.Close(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// --- ListTools ---

func TestListTools_ReturnsCachedTools(t *testing.T) {
	c := NewConduit(ConduitConfig{Name: "x"}).(*MCPConduit)
	c.tools = []eywa.ActionDefinition{{Name: "foo"}}
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "foo" {
		t.Errorf("unexpected tools: %v", tools)
	}
}

// --- Connect ---

func TestConnect_UnsupportedTransport_ReturnsError(t *testing.T) {
	c := NewConduit(ConduitConfig{Name: "x", Transport: "stdio"}).(*MCPConduit)
	if err := c.Connect(context.Background()); err == nil {
		t.Error("expected error for unsupported transport")
	}
}

func TestConnect_NetworkError_ReturnsError(t *testing.T) {
	c := newMCPConduit(t, "http://127.0.0.1:1")
	if err := c.Connect(context.Background()); err == nil {
		t.Error("expected error when server unreachable")
	}
}

func TestConnect_PrivateURL_RejectsSSRF(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"loopback ipv4", "http://127.0.0.1/mcp"},
		{"private 10.x", "http://10.0.0.1/mcp"},
		{"private 192.168.x", "http://192.168.1.1/mcp"},
		{"file scheme", "file:///etc/passwd"},
		{"no scheme", "/etc/passwd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewConduit(ConduitConfig{Name: "x", Transport: "http", URL: tc.url}).(*MCPConduit)
			if err := c.Connect(context.Background()); err == nil {
				t.Errorf("expected SSRF error for URL %q, got nil", tc.url)
			}
		})
	}
}

// successMCPServer returns a handler that responds correctly to initialize then tools/list.
func successMCPServer(t *testing.T, extraTools ...map[string]any) *httptest.Server {
	t.Helper()
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := call.Add(1)
		switch n {
		case 1: // initialize
			respondJSON(w, mcpOK(1, map[string]any{}))
		case 2: // tools/list
			respondJSON(w, mcpOK(2, toolsListResult(extraTools...)))
		default:
			respondJSON(w, mcpOK(int(n), map[string]any{}))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestConnect_Success_PopulatesTools(t *testing.T) {
	srv := successMCPServer(t,
		map[string]any{"name": "search", "description": "Search tool"},
	)
	c := newMCPConduit(t, srv.URL)
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.tools) != 1 || c.tools[0].Name != "search" {
		t.Errorf("unexpected tools: %v", c.tools)
	}
}

func TestConnect_EmptyTransport_TreatedAsHTTP(t *testing.T) {
	srv := successMCPServer(t)
	c := NewConduit(ConduitConfig{Name: "x", Transport: "", URL: srv.URL}).(*MCPConduit)
	c.urlValidator = func(string) error { return nil }
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("empty transport should work like http: %v", err)
	}
}

func TestConnect_ListToolsError_ReturnsError(t *testing.T) {
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := call.Add(1)
		if n == 1 {
			respondJSON(w, mcpOK(1, map[string]any{}))
		} else {
			// tools/list returns MCP error
			respondJSON(w, mcpErr(2, -32601, "method not found"))
		}
	}))
	defer srv.Close()

	c := newMCPConduit(t, srv.URL)
	if err := c.Connect(context.Background()); err == nil {
		t.Error("expected error when tools/list fails")
	}
}

// --- Call ---

func TestCall_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "hello world"},
			},
		}
		respondJSON(w, mcpOK(2, result))
	}))
	defer srv.Close()

	c := newMCPConduit(t, srv.URL)
	out, err := c.Call(context.Background(), "greet", map[string]any{"name": "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello world" {
		t.Errorf("expected 'hello world', got %q", out)
	}
}

func TestCall_MCPError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, mcpErr(2, -32600, "tool not found"))
	}))
	defer srv.Close()

	c := newMCPConduit(t, srv.URL)
	_, err := c.Call(context.Background(), "missing", nil)
	if err == nil {
		t.Error("expected error for MCP error response")
	}
}

func TestCall_HTTPError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newMCPConduit(t, srv.URL)
	_, err := c.Call(context.Background(), "foo", nil)
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestCall_NetworkError_ReturnsError(t *testing.T) {
	c := newMCPConduit(t, "http://127.0.0.1:1")
	_, err := c.Call(context.Background(), "foo", nil)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

// --- post ---

func TestPost_InvalidJSONResponse_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not-json")) //nolint:errcheck
	}))
	defer srv.Close()

	c := newMCPConduit(t, srv.URL)
	var resp mcpResponse
	err := c.post(context.Background(), mcpRequest{JSONRPC: "2.0", ID: 1, Method: "test"}, &resp)
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestPost_CustomHeaders_AreSent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		respondJSON(w, mcpOK(1, map[string]any{}))
	}))
	defer srv.Close()

	c := NewConduit(ConduitConfig{
		Name:      "x",
		URL:       srv.URL,
		Transport: "http",
		Headers:   map[string]string{"Authorization": "Bearer tok123"},
	}).(*MCPConduit)
	c.urlValidator = func(string) error { return nil }

	var resp mcpResponse
	_ = c.post(context.Background(), mcpRequest{JSONRPC: "2.0", ID: 1, Method: "init"}, &resp)

	if gotAuth != "Bearer tok123" {
		t.Errorf("expected Authorization header, got %q", gotAuth)
	}
}

// --- parseToolList ---

func TestParseToolList_ValidTools(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"tools": []map[string]any{
			{"name": "search", "description": "Search web", "inputSchema": map[string]any{"type": "object"}},
			{"name": "calc", "description": "Calculator"},
		},
	})
	defs := parseToolList(raw)
	if len(defs) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(defs))
	}
	if defs[0].Name != "search" || defs[0].Description != "Search web" {
		t.Errorf("unexpected first tool: %+v", defs[0])
	}
	if defs[1].Name != "calc" {
		t.Errorf("unexpected second tool: %+v", defs[1])
	}
}

func TestParseToolList_EmptyTools(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"tools": []any{}})
	defs := parseToolList(raw)
	if len(defs) != 0 {
		t.Errorf("expected 0 tools, got %d", len(defs))
	}
}

func TestParseToolList_InvalidJSON_ReturnsNil(t *testing.T) {
	defs := parseToolList(json.RawMessage("not-json"))
	if defs != nil {
		t.Errorf("expected nil for invalid JSON, got %v", defs)
	}
}

// --- extractTextContent ---

func TestExtractTextContent_SingleTextBlock(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": "hello"},
		},
	})
	got := extractTextContent(raw)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestExtractTextContent_MultipleTextBlocks(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": "foo"},
			{"type": "text", "text": "bar"},
		},
	})
	got := extractTextContent(raw)
	if got != "foobar" {
		t.Errorf("expected 'foobar', got %q", got)
	}
}

func TestExtractTextContent_NonTextBlockIgnored(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"content": []map[string]any{
			{"type": "image", "data": "base64..."},
			{"type": "text", "text": "answer"},
		},
	})
	got := extractTextContent(raw)
	if got != "answer" {
		t.Errorf("expected 'answer', got %q", got)
	}
}

func TestExtractTextContent_InvalidJSON_ReturnsRaw(t *testing.T) {
	raw := json.RawMessage("not-json")
	got := extractTextContent(raw)
	if got != "not-json" {
		t.Errorf("expected raw string, got %q", got)
	}
}

func TestExtractTextContent_EmptyContent(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"content": []any{}})
	got := extractTextContent(raw)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestMCPConduit_RequestIDs_AreUnique(t *testing.T) {
	c := NewConduit(ConduitConfig{Name: "x"}).(*MCPConduit)
	id1 := c.nextID()
	id2 := c.nextID()
	id3 := c.nextID()
	if id1 == id2 || id2 == id3 {
		t.Errorf("expected unique IDs, got %d %d %d", id1, id2, id3)
	}
}
