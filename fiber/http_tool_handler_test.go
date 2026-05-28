package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	"github.com/wmulabs/eywa/fiber/middleware"
)

type stubHTTPToolRepo struct {
	tools map[string]eywa.HTTPTool
	err   error
}

func newStubHTTPToolRepo() *stubHTTPToolRepo {
	return &stubHTTPToolRepo{tools: make(map[string]eywa.HTTPTool)}
}

func (s *stubHTTPToolRepo) List(_ context.Context) ([]eywa.HTTPTool, error) {
	result := make([]eywa.HTTPTool, 0, len(s.tools))
	for _, t := range s.tools {
		result = append(result, t)
	}
	return result, s.err
}
func (s *stubHTTPToolRepo) FindBySpiritID(_ context.Context, spiritID string) ([]eywa.HTTPTool, error) {
	var result []eywa.HTTPTool
	for _, t := range s.tools {
		for _, id := range t.SpiritIDs {
			if id == spiritID {
				result = append(result, t)
				break
			}
		}
	}
	return result, s.err
}
func (s *stubHTTPToolRepo) FindByID(_ context.Context, id string) (*eywa.HTTPTool, error) {
	t, ok := s.tools[id]
	if !ok {
		return nil, eywa.ErrNotFound
	}
	return &t, s.err
}
func (s *stubHTTPToolRepo) Save(_ context.Context, tool eywa.HTTPTool) error {
	s.tools[tool.ID] = tool
	return s.err
}
func (s *stubHTTPToolRepo) Update(_ context.Context, tool eywa.HTTPTool) error {
	s.tools[tool.ID] = tool
	return s.err
}
func (s *stubHTTPToolRepo) Delete(_ context.Context, id string) error {
	delete(s.tools, id)
	return s.err
}

func httpToolDeps(repo *stubHTTPToolRepo) ManagementDeps {
	return ManagementDeps{
		APIKeys:      map[string]string{"test-key": "admin"},
		HTTPToolRepo: repo,
	}
}

func TestHTTPToolHandler_List_Returns200(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.tools["tool-1"] = eywa.HTTPTool{ID: "tool-1", Name: "get_order", Method: "GET", URL: "https://api.example.com/orders"}
	app := buildMgmtTestApp(httpToolDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/http-tools", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Errorf("want 1 item, got %d", len(body.Items))
	}
}

func TestHTTPToolHandler_List_ReturnsEmptySlice(t *testing.T) {
	app := buildMgmtTestApp(httpToolDeps(newStubHTTPToolRepo()))

	req := httptest.NewRequest("GET", "/api/v1/http-tools", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []any `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Items == nil {
		t.Error("items must be [] not null")
	}
}

func TestHTTPToolHandler_Create_Returns201(t *testing.T) {
	app := buildMgmtTestApp(httpToolDeps(newStubHTTPToolRepo()))

	tool := map[string]any{
		"name":        "get_order",
		"description": "Fetches an order",
		"method":      "GET",
		"url":         "https://api.example.com/orders/{{order_id}}",
	}
	b, _ := json.Marshal(tool)
	req := httptest.NewRequest("POST", "/api/v1/http-tools", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["id"] == nil || body["id"] == "" {
		t.Error("want non-empty id in response")
	}
}

func TestHTTPToolHandler_GetByID_Returns200(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.tools["tool-1"] = eywa.HTTPTool{ID: "tool-1", Name: "get_order", Method: "GET", URL: "https://api.example.com"}
	app := buildMgmtTestApp(httpToolDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/http-tools/tool-1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["id"] != "tool-1" {
		t.Errorf("want id tool-1, got %v", body["id"])
	}
}

func TestHTTPToolHandler_GetByID_NotFound_Returns404(t *testing.T) {
	app := buildMgmtTestApp(httpToolDeps(newStubHTTPToolRepo()))

	req := httptest.NewRequest("GET", "/api/v1/http-tools/unknown", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Update_Returns200(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.tools["tool-1"] = eywa.HTTPTool{ID: "tool-1", Name: "get_order", Method: "GET", URL: "https://api.example.com"}
	app := buildMgmtTestApp(httpToolDeps(repo))

	updated := map[string]any{
		"id":     "tool-1",
		"name":   "get_order_v2",
		"method": "GET",
		"url":    "https://api.example.com/v2/orders",
	}
	b, _ := json.Marshal(updated)
	req := httptest.NewRequest("PUT", "/api/v1/http-tools/tool-1", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Delete_Returns204(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.tools["tool-1"] = eywa.HTTPTool{ID: "tool-1", Name: "get_order", Method: "GET", URL: "https://api.example.com"}
	app := buildMgmtTestApp(httpToolDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/http-tools/tool-1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 204 {
		t.Errorf("want 204, got %d", resp.StatusCode)
	}
}

func buildMgmtTestAppWithTester(
	deps ManagementDeps,
	testerFunc func(eywa.HTTPTool, map[string]any) (*eywa.HTTPToolTestResult, error),
) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	validators := buildValidatorChain(deps)
	authMW := middleware.AuthMiddleware(validators...)
	api := app.Group("/api/v1", authMW)
	if deps.HTTPToolRepo != nil {
		hth := &httpToolHandler{repo: deps.HTTPToolRepo, testerFunc: testerFunc}
		httpTools := api.Group("/http-tools")
		httpTools.Get("", hth.list)
		httpTools.Post("", hth.create)
		httpTools.Get("/:id", hth.getByID)
		httpTools.Put("/:id", hth.update)
		httpTools.Delete("/:id", hth.delete)
		httpTools.Post("/:id/test", hth.test)
	}
	return app
}

func TestHTTPToolHandler_Test_Returns200(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.tools["tool-1"] = eywa.HTTPTool{
		ID:     "tool-1",
		Name:   "get_order",
		Method: "GET",
		URL:    "https://api.example.com/orders/{{order_id}}",
	}
	stubTester := func(tool eywa.HTTPTool, args map[string]any) (*eywa.HTTPToolTestResult, error) {
		return &eywa.HTTPToolTestResult{
			StatusCode:  200,
			Response:    `{"status":"ok"}`,
			ResolvedURL: "https://api.example.com/orders/123",
			LatencyMS:   5,
		}, nil
	}
	app := buildMgmtTestAppWithTester(httpToolDeps(repo), stubTester)

	payload := map[string]any{
		"args": map[string]any{"order_id": "123"},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/http-tools/tool-1/test", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		StatusCode  int    `json:"status_code"`
		Response    string `json:"response"`
		ResolvedURL string `json:"resolved_url"`
		LatencyMS   int64  `json:"latency_ms"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.StatusCode != 200 {
		t.Errorf("want status_code 200, got %d", body.StatusCode)
	}
	if body.ResolvedURL == "" {
		t.Error("want non-empty resolved_url")
	}
}

func TestHTTPToolHandler_List_NilSlice_Returns200WithEmptyItems(t *testing.T) {
	// stubNilListRepo returns nil slice — covers the nil guard in list()
	type nilListRepo struct{ *stubHTTPToolRepo }
	orig := newStubHTTPToolRepo()
	repo := &nilListRepo{stubHTTPToolRepo: orig}
	// Override via function-level stub that returns nil
	deps := ManagementDeps{
		APIKeys:      map[string]string{"test-key": "admin"},
		HTTPToolRepo: &stubNilHTTPToolRepo{},
	}
	app := buildMgmtTestApp(deps)

	req := httptest.NewRequest("GET", "/api/v1/http-tools", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	items, _ := result["items"].([]any)
	if items == nil {
		t.Error("want empty items array, got nil")
	}
	_ = repo
}

type stubNilHTTPToolRepo struct{ *stubHTTPToolRepo }

func (s *stubNilHTTPToolRepo) List(_ context.Context) ([]eywa.HTTPTool, error)             { return nil, nil }
func (s *stubNilHTTPToolRepo) FindBySpiritID(_ context.Context, _ string) ([]eywa.HTTPTool, error) { return nil, nil }
func (s *stubNilHTTPToolRepo) FindByID(_ context.Context, _ string) (*eywa.HTTPTool, error) { return nil, eywa.ErrNotFound }
func (s *stubNilHTTPToolRepo) Save(_ context.Context, _ eywa.HTTPTool) error                { return nil }
func (s *stubNilHTTPToolRepo) Update(_ context.Context, _ eywa.HTTPTool) error              { return nil }
func (s *stubNilHTTPToolRepo) Delete(_ context.Context, _ string) error                     { return nil }

func TestHTTPToolHandler_List_RepoError_Returns500(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.err = errInternal
	app := buildMgmtTestApp(httpToolDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/http-tools", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Create_InvalidBody_Returns400(t *testing.T) {
	app := buildMgmtTestApp(httpToolDeps(newStubHTTPToolRepo()))

	req := httptest.NewRequest("POST", "/api/v1/http-tools", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Create_RepoError_Returns500(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.err = errInternal
	app := buildMgmtTestApp(httpToolDeps(repo))

	b, _ := json.Marshal(map[string]any{"name": "tool", "method": "GET", "url": "https://example.com"})
	req := httptest.NewRequest("POST", "/api/v1/http-tools", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_GetByID_GenericError_Returns500(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.tools["tool-1"] = eywa.HTTPTool{ID: "tool-1"}
	repo.err = errInternal
	app := buildMgmtTestApp(httpToolDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/http-tools/tool-1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Update_InvalidBody_Returns400(t *testing.T) {
	app := buildMgmtTestApp(httpToolDeps(newStubHTTPToolRepo()))

	req := httptest.NewRequest("PUT", "/api/v1/http-tools/tool-1", bytes.NewReader([]byte("bad")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Update_RepoError_Returns500(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.err = errInternal
	app := buildMgmtTestApp(httpToolDeps(repo))

	b, _ := json.Marshal(map[string]any{"id": "tool-1", "name": "x", "method": "GET", "url": "https://x.com"})
	req := httptest.NewRequest("PUT", "/api/v1/http-tools/tool-1", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Delete_RepoError_Returns500(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.err = errInternal
	app := buildMgmtTestApp(httpToolDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/http-tools/tool-1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Test_InvalidBody_Returns400(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.tools["tool-1"] = eywa.HTTPTool{ID: "tool-1", Name: "x", Method: "GET", URL: "https://x.com"}
	app := buildMgmtTestApp(httpToolDeps(repo))

	req := httptest.NewRequest("POST", "/api/v1/http-tools/tool-1/test", bytes.NewReader([]byte("bad")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Test_ExecutorError_Returns502(t *testing.T) {
	// Use localhost:1 — connection refused is immediate
	repo := newStubHTTPToolRepo()
	repo.tools["tool-1"] = eywa.HTTPTool{
		ID:     "tool-1",
		Name:   "fail",
		Method: "GET",
		URL:    "http://localhost:1/unreachable",
	}
	app := buildMgmtTestApp(httpToolDeps(repo))

	b, _ := json.Marshal(map[string]any{"args": map[string]any{}})
	req := httptest.NewRequest("POST", "/api/v1/http-tools/tool-1/test", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, 5000)
	if resp.StatusCode != 502 {
		t.Errorf("want 502, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Test_GenericRepoError_Returns500(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.tools["tool-1"] = eywa.HTTPTool{ID: "tool-1"}
	repo.err = errInternal
	app := buildMgmtTestApp(httpToolDeps(repo))

	b, _ := json.Marshal(map[string]any{"args": map[string]any{}})
	req := httptest.NewRequest("POST", "/api/v1/http-tools/tool-1/test", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestHTTPToolHandler_Test_NilArgs_DefaultsToEmptyMap_Returns200(t *testing.T) {
	repo := newStubHTTPToolRepo()
	repo.tools["tool-2"] = eywa.HTTPTool{
		ID:     "tool-2",
		Name:   "ping",
		Method: "GET",
		URL:    "https://api.example.com/orders",
	}
	var receivedArgs map[string]any
	stubTester := func(tool eywa.HTTPTool, args map[string]any) (*eywa.HTTPToolTestResult, error) {
		receivedArgs = args
		return &eywa.HTTPToolTestResult{StatusCode: 200, Response: "ok", ResolvedURL: tool.URL, LatencyMS: 1}, nil
	}
	app := buildMgmtTestAppWithTester(httpToolDeps(repo), stubTester)

	// Omit "args" from body — handler should default to empty map.
	payload := map[string]any{}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/http-tools/tool-2/test", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if receivedArgs == nil {
		t.Error("want non-nil args (defaulted to empty map), got nil")
	}
}

func TestHTTPToolHandler_Test_ToolNotFound_Returns404(t *testing.T) {
	app := buildMgmtTestApp(httpToolDeps(newStubHTTPToolRepo()))

	payload := map[string]any{"args": map[string]any{}}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/http-tools/unknown/test", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}
