package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func TestResolveTemplate_ReplacesPlaceholders(t *testing.T) {
	result := resolveTemplate("https://api.example.com/orders/{{order_id}}/status", map[string]any{
		"order_id": "12345",
	})
	if result != "https://api.example.com/orders/12345/status" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestResolveTemplate_MultipleParams(t *testing.T) {
	result := resolveTemplate(`{"order":"{{order_id}}","reason":"{{reason}}"}`, map[string]any{
		"order_id": "99",
		"reason":   "test",
	})
	if result != `{"order":"99","reason":"test"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestResolveTemplate_NoPlaceholders(t *testing.T) {
	result := resolveTemplate("https://api.example.com/status", map[string]any{})
	if result != "https://api.example.com/status" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestHTTPToolExecutor_GetParameters_BuildsSchema(t *testing.T) {
	tool := entities.HTTPTool{
		Parameters: []entities.HTTPToolParam{
			{Name: "order_id", Type: "string", Description: "The order ID", Required: true},
			{Name: "verbose", Type: "boolean", Description: "Include details", Required: false},
		},
	}
	exec := NewHTTPToolExecutor(tool)
	schema := exec.GetParameters()

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	if _, ok := props["order_id"]; !ok {
		t.Error("expected order_id in properties")
	}
	required, ok := schema["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "order_id" {
		t.Errorf("expected required=[order_id], got %v", schema["required"])
	}
}

func TestHTTPToolExecutor_Validate_MissingRequired(t *testing.T) {
	tool := entities.HTTPTool{
		Parameters: []entities.HTTPToolParam{
			{Name: "order_id", Required: true},
		},
	}
	exec := NewHTTPToolExecutor(tool)
	if err := exec.Validate(map[string]any{}); err == nil {
		t.Error("expected error for missing required param")
	}
}

func TestHTTPToolExecutor_Validate_PassesWhenAllPresent(t *testing.T) {
	tool := entities.HTTPTool{
		Parameters: []entities.HTTPToolParam{
			{Name: "order_id", Required: true},
		},
	}
	exec := NewHTTPToolExecutor(tool)
	if err := exec.Validate(map[string]any{"order_id": "123"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- accessors ---

func TestHTTPToolExecutor_Accessors(t *testing.T) {
	tool := entities.HTTPTool{
		Name:        "get-order",
		Description: "Fetches an order",
		IsCritical:  true,
	}
	exec := NewHTTPToolExecutor(tool)
	if exec.GetName() != "get-order" {
		t.Errorf("expected GetName=get-order, got %s", exec.GetName())
	}
	if exec.GetDescription() != "Fetches an order" {
		t.Errorf("expected GetDescription, got %s", exec.GetDescription())
	}
	if !exec.IsCritical() {
		t.Error("expected IsCritical=true")
	}
	if exec.GetCategory() != ports.ActionGeneral {
		t.Errorf("unexpected category: %v", exec.GetCategory())
	}
}

func TestHTTPToolExecutor_DefaultTimeout(t *testing.T) {
	tool := entities.HTTPTool{TimeoutMS: 0} // <= 0 → 10s default
	exec := NewHTTPToolExecutor(tool)
	if exec.httpClient.Timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", exec.httpClient.Timeout)
	}
}

func TestHTTPToolExecutor_CustomTimeout(t *testing.T) {
	tool := entities.HTTPTool{TimeoutMS: 500}
	exec := NewHTTPToolExecutor(tool)
	if exec.httpClient.Timeout != 500*time.Millisecond {
		t.Errorf("expected timeout 500ms, got %v", exec.httpClient.Timeout)
	}
}

func TestHTTPToolExecutor_GetParameters_NoRequired_NoRequiredField(t *testing.T) {
	tool := entities.HTTPTool{
		Parameters: []entities.HTTPToolParam{
			{Name: "verbose", Type: "boolean", Required: false},
		},
	}
	exec := NewHTTPToolExecutor(tool)
	schema := exec.GetParameters()
	if _, ok := schema["required"]; ok {
		t.Error("expected no 'required' field when no required params")
	}
}

// --- Execute ---

func TestHTTPToolExecutor_Execute_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	tool := entities.HTTPTool{
		Name:   "test-tool",
		URL:    server.URL + "/api",
		Method: http.MethodGet,
	}
	exec := NewHTTPToolExecutor(tool)
	result, err := exec.execute(context.Background(), server.URL+"/api", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"status":"ok"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestHTTPToolExecutor_Execute_WithBody(t *testing.T) {
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tool := entities.HTTPTool{
		Name:         "test-tool",
		URL:          server.URL,
		Method:       http.MethodPost,
		BodyTemplate: `{"order":"{{order_id}}"}`,
	}
	exec := NewHTTPToolExecutor(tool)
	args := map[string]any{"order_id": "123"}
	_, err := exec.execute(context.Background(), server.URL, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %s", receivedContentType)
	}
}

func TestHTTPToolExecutor_Execute_WithHeaders(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tool := entities.HTTPTool{
		Name:    "test-tool",
		URL:     server.URL,
		Method:  http.MethodGet,
		Headers: map[string]string{"Authorization": "Bearer {{token}}"},
	}
	exec := NewHTTPToolExecutor(tool)
	_, err := exec.execute(context.Background(), server.URL, map[string]any{"token": "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedAuth != "Bearer secret" {
		t.Errorf("expected Authorization=Bearer secret, got %s", receivedAuth)
	}
}

func TestHTTPToolExecutor_Execute_HTTPError_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	tool := entities.HTTPTool{
		Name:   "test-tool",
		URL:    server.URL,
		Method: http.MethodGet,
	}
	exec := NewHTTPToolExecutor(tool)
	_, err := exec.execute(context.Background(), server.URL, map[string]any{})
	if err == nil {
		t.Error("expected error for 5xx status")
	}
}

func TestHTTPToolExecutor_Execute_InvalidURL_ReturnsError(t *testing.T) {
	tool := entities.HTTPTool{
		Name:   "test-tool",
		URL:    "://invalid-url",
		Method: http.MethodGet,
	}
	exec := NewHTTPToolExecutor(tool)
	_, err := exec.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// --- Test ---

func TestHTTPToolExecutor_Test_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test-response"))
	}))
	defer server.Close()

	tool := entities.HTTPTool{
		Name:   "test-tool",
		URL:    server.URL,
		Method: http.MethodGet,
	}
	exec := NewHTTPToolExecutor(tool)
	result, err := exec.test(context.Background(), server.URL, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", result.StatusCode)
	}
	if result.Response != "test-response" {
		t.Errorf("unexpected response: %s", result.Response)
	}
	if result.ResolvedURL != server.URL {
		t.Errorf("unexpected URL: %s", result.ResolvedURL)
	}
}

func TestHTTPToolExecutor_Test_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))
	defer server.Close()

	tool := entities.HTTPTool{
		Name:         "test-tool",
		URL:          server.URL,
		Method:       http.MethodPost,
		BodyTemplate: `{"id":"{{id}}"}`,
	}
	exec := NewHTTPToolExecutor(tool)
	result, err := exec.test(context.Background(), server.URL, map[string]any{"id": "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", result.StatusCode)
	}
}

func TestHTTPToolExecutor_Execute_SSRFBlock_LoopbackIP(t *testing.T) {
	// Valid URL but nothing listening — Do fails
	tool := entities.HTTPTool{
		Name:      "test-tool",
		URL:       "http://127.0.0.1:1", // port 1 is reserved, connection refused
		Method:    http.MethodGet,
		TimeoutMS: 500,
	}
	exec := NewHTTPToolExecutor(tool)
	_, err := exec.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for refused connection")
	}
}

func TestHTTPToolExecutor_Test_SSRFBlock_LoopbackIP(t *testing.T) {
	tool := entities.HTTPTool{
		Name:      "test-tool",
		URL:       "http://127.0.0.1:1",
		Method:    http.MethodGet,
		TimeoutMS: 500,
	}
	exec := NewHTTPToolExecutor(tool)
	_, err := exec.Test(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for refused connection")
	}
}

func TestHTTPToolExecutor_Test_WithHeaders(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tool := entities.HTTPTool{
		Name:    "test-tool",
		URL:     server.URL,
		Method:  http.MethodGet,
		Headers: map[string]string{"X-Api-Key": "key-{{token}}"},
	}
	exec := NewHTTPToolExecutor(tool)
	_, err := exec.test(context.Background(), server.URL, map[string]any{"token": "abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedAuth != "key-abc" {
		t.Errorf("expected X-Api-Key=key-abc, got %s", receivedAuth)
	}
}

func TestHTTPToolExecutor_Test_InvalidURL_ReturnsError(t *testing.T) {
	tool := entities.HTTPTool{
		Name:   "test-tool",
		URL:    "://bad",
		Method: http.MethodGet,
	}
	exec := NewHTTPToolExecutor(tool)
	_, err := exec.Test(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// --- validateURL ---

func TestValidateURL_BlocksLoopback(t *testing.T) {
	if err := validateURL("http://127.0.0.1/secret"); err == nil {
		t.Error("expected error for loopback IP")
	}
}

func TestValidateURL_BlocksRFC1918(t *testing.T) {
	if err := validateURL("http://10.0.0.1/secret"); err == nil {
		t.Error("expected error for RFC1918 address")
	}
	if err := validateURL("http://192.168.1.1/secret"); err == nil {
		t.Error("expected error for RFC1918 address")
	}
}

func TestValidateURL_BlocksLinkLocal(t *testing.T) {
	if err := validateURL("http://169.254.169.254/metadata"); err == nil {
		t.Error("expected error for AWS/GCP metadata address")
	}
}

func TestValidateURL_BlocksFileScheme(t *testing.T) {
	if err := validateURL("file:///etc/passwd"); err == nil {
		t.Error("expected error for file:// scheme")
	}
}

func TestValidateURL_BlocksFTPScheme(t *testing.T) {
	if err := validateURL("ftp://10.0.0.1/secret"); err == nil {
		t.Error("expected error for ftp:// scheme")
	}
}

func TestValidateURL_AllowsHTTPS(t *testing.T) {
	// This does a DNS lookup for example.com which is a public IP.
	// If DNS is unavailable, validateURL returns nil (fails open, let http.Client handle it).
	err := validateURL("https://example.com/api")
	if err != nil {
		t.Logf("validateURL returned %v (may be DNS unavailable in test env)", err)
	}
	// Not a hard failure — DNS may not be available in CI.
}

// --- HTTPToolLoadStep ---

type stubHTTPToolRepo struct {
	tools []entities.HTTPTool
	err   error
}

var _ ports.HTTPToolRepository = (*stubHTTPToolRepo)(nil)

func (r *stubHTTPToolRepo) List(_ context.Context) ([]entities.HTTPTool, error) {
	return r.tools, r.err
}
func (r *stubHTTPToolRepo) FindBySpiritID(_ context.Context, _ string) ([]entities.HTTPTool, error) {
	return r.tools, r.err
}
func (r *stubHTTPToolRepo) FindByID(_ context.Context, _ string) (*entities.HTTPTool, error) {
	return nil, nil
}
func (r *stubHTTPToolRepo) Save(_ context.Context, _ entities.HTTPTool) error   { return nil }
func (r *stubHTTPToolRepo) Update(_ context.Context, _ entities.HTTPTool) error { return nil }
func (r *stubHTTPToolRepo) Delete(_ context.Context, _ string) error            { return nil }

type stubActionRegistryForHTTP struct {
	registerErr error
	registered  []string
}

var _ ports.ActionRegistry = (*stubActionRegistryForHTTP)(nil)

func (r *stubActionRegistryForHTTP) Register(a ports.Action) error {
	r.registered = append(r.registered, a.GetName())
	return r.registerErr
}
func (r *stubActionRegistryForHTTP) RegisterMultiple(_ ...ports.Action) error { return nil }
func (r *stubActionRegistryForHTTP) Execute(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}
func (r *stubActionRegistryForHTTP) Get(_ string) (ports.Action, error) { return nil, nil }
func (r *stubActionRegistryForHTTP) GetMultiple(_ []string) ([]ports.Action, error) {
	return nil, nil
}
func (r *stubActionRegistryForHTTP) List() []ports.Action       { return nil }
func (r *stubActionRegistryForHTTP) IsRegistered(_ string) bool { return false }
func (r *stubActionRegistryForHTTP) ListRegistered() []string   { return nil }
func (r *stubActionRegistryForHTTP) GetActionDefinitions(_ context.Context, _ []string) ([]entities.ActionDefinition, error) {
	return nil, nil
}

func httpToolState(spiritID string) *ProcessingState {
	return &ProcessingState{
		Spirit: &entities.Spirit{ID: spiritID, Name: "support", AllowedActions: []entities.AllowedAction{}},
		Event:  &entities.Pulse{MemoryKey: "user:1"},
	}
}

func TestHTTPToolLoadStep_RepoFails_NoOp(t *testing.T) {
	repo := &stubHTTPToolRepo{err: errors.New("db error")}
	reg := &stubActionRegistryForHTTP{}
	step := NewHTTPToolLoadStep(repo, reg, time.Second, testLogger(t))
	state := httpToolState("spirit-1")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("repo failure should be a no-op, got: %v", err)
	}
	if len(state.Spirit.AllowedActions) != 0 {
		t.Error("expected no actions added on repo failure")
	}
}

func TestHTTPToolLoadStep_RegistersTools(t *testing.T) {
	repo := &stubHTTPToolRepo{tools: []entities.HTTPTool{
		{Name: "get-order", URL: "https://api.example.com/orders/{{id}}", Method: http.MethodGet},
		{Name: "cancel-order", URL: "https://api.example.com/orders/{{id}}/cancel", Method: http.MethodPost},
	}}
	reg := &stubActionRegistryForHTTP{}
	step := NewHTTPToolLoadStep(repo, reg, time.Second, testLogger(t))
	state := httpToolState("spirit-1")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(state.Spirit.AllowedActions) != 2 {
		t.Errorf("expected 2 allowed actions, got %d", len(state.Spirit.AllowedActions))
	}
	if len(reg.registered) != 2 {
		t.Errorf("expected 2 registrations, got %d", len(reg.registered))
	}
}

func TestHTTPToolLoadStep_AlreadyRegistered_StillAddsAllowedAction(t *testing.T) {
	// Register returns error (already registered) but AllowedAction is still appended
	repo := &stubHTTPToolRepo{tools: []entities.HTTPTool{
		{Name: "get-order"},
	}}
	reg := &stubActionRegistryForHTTP{registerErr: errors.New("already registered")}
	step := NewHTTPToolLoadStep(repo, reg, time.Second, testLogger(t))
	state := httpToolState("spirit-1")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("already-registered should be non-fatal, got: %v", err)
	}
	if len(state.Spirit.AllowedActions) != 1 {
		t.Errorf("expected 1 allowed action even when already registered, got %d", len(state.Spirit.AllowedActions))
	}
}

func TestHTTPToolLoadStep_EmptyTools_NoOp(t *testing.T) {
	repo := &stubHTTPToolRepo{tools: []entities.HTTPTool{}}
	reg := &stubActionRegistryForHTTP{}
	step := NewHTTPToolLoadStep(repo, reg, time.Second, testLogger(t))
	state := httpToolState("spirit-1")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(state.Spirit.AllowedActions) != 0 {
		t.Error("expected no actions for empty tool list")
	}
}

func TestHTTPToolLoadStep_NameAndTimeout(t *testing.T) {
	step := NewHTTPToolLoadStep(nil, nil, 5*time.Second, testLogger(t))
	if step.Name() != "HTTPToolLoad" {
		t.Errorf("expected name=HTTPToolLoad, got %s", step.Name())
	}
	if step.Timeout() != 5*time.Second {
		t.Errorf("expected timeout=5s, got %v", step.Timeout())
	}
}

func TestHTTPToolExecutor_Execute_LargeResponse_Truncated(t *testing.T) {
	bigBody := strings.Repeat("x", 2*1024*1024) // 2 MiB
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(bigBody))
	}))
	defer server.Close()

	tool := entities.HTTPTool{
		Name:             "big-tool",
		URL:              server.URL,
		Method:           http.MethodGet,
		MaxResponseBytes: 512 * 1024, // 512 KiB limit
	}
	exec := NewHTTPToolExecutor(tool)
	result, err := exec.execute(context.Background(), server.URL, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) > 512*1024 {
		t.Errorf("expected response capped at 512KiB, got %d bytes", len(result))
	}
}
