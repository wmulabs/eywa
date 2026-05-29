package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

type stubLinkRepo struct {
	links map[string]*eywa.Link
	err   error
}

func newStubLinkRepo(links ...*eywa.Link) *stubLinkRepo {
	s := &stubLinkRepo{links: make(map[string]*eywa.Link)}
	for _, l := range links {
		s.links[l.EventType] = l
	}
	return s
}

func (s *stubLinkRepo) FindAll(_ context.Context) ([]*eywa.Link, error) {
	result := make([]*eywa.Link, 0, len(s.links))
	for _, l := range s.links {
		result = append(result, l)
	}
	return result, s.err
}
func (s *stubLinkRepo) FindByKey(_ context.Context, eventType string) (*eywa.Link, error) {
	l, ok := s.links[eventType]
	if !ok {
		return nil, eywa.ErrNotFound
	}
	return l, s.err
}
func (s *stubLinkRepo) Save(_ context.Context, link *eywa.Link) error {
	s.links[link.EventType] = link
	return s.err
}
func (s *stubLinkRepo) Delete(_ context.Context, eventType string) error {
	delete(s.links, eventType)
	return s.err
}

func linkCacheDeps(repo *stubLinkRepo) ManagementDeps {
	cache := eywa.NewConfigCache(repo, nil, nil)
	_ = cache.LoadAll(context.Background())
	return ManagementDeps{
		APIKeys:     map[string]string{"test-key": "admin"},
		ConfigCache: cache,
	}
}

func TestLinkMgmtHandler_List_Returns200(t *testing.T) {
	repo := newStubLinkRepo(
		eywa.NewLink("whatsapp_message").WithDefaultSpirit("support").Build(),
	)
	app := buildMgmtTestApp(linkCacheDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/event-configurations", nil)
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
		t.Errorf("want 1 link, got %d", len(body.Items))
	}
}

func TestLinkMgmtHandler_List_ReturnsEmptySlice(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	req := httptest.NewRequest("GET", "/api/v1/event-configurations", nil)
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

func TestLinkMgmtHandler_GetByKey_Returns200(t *testing.T) {
	repo := newStubLinkRepo(
		eywa.NewLink("whatsapp_message").WithDefaultSpirit("support").Build(),
	)
	app := buildMgmtTestApp(linkCacheDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/event-configurations/whatsapp_message", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["event_type"] != "whatsapp_message" {
		t.Errorf("want event_type whatsapp_message, got %v", body["event_type"])
	}
}

func TestLinkMgmtHandler_GetByKey_NotFound_Returns404(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	req := httptest.NewRequest("GET", "/api/v1/event-configurations/unknown", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Save_Returns200(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	body := map[string]any{
		"event_type":    "sms_message",
		"default_agent": "support",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/event-configurations/sms_message", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Save_MismatchedKey_Returns400(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	body := map[string]any{"event_type": "other_event"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/event-configurations/sms_message", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_List_WithGuards_Returns200(t *testing.T) {
	link := eywa.NewLink("guarded_event").
		WithGuards(eywa.Guard{Field: "sender", AllowList: []string{"allowed"}}).
		Build()
	repo := newStubLinkRepo(link)
	app := buildMgmtTestApp(linkCacheDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/event-configurations", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Save_InvalidBody_Returns400(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	req := httptest.NewRequest("PUT", "/api/v1/event-configurations/sms", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Save_RepoError_Returns500(t *testing.T) {
	repo := newStubLinkRepo()
	repo.err = errInternal
	app := buildMgmtTestApp(linkCacheDeps(repo))

	body := map[string]any{"event_type": "sms"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/event-configurations/sms", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Save_WithGuards_Returns200(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	body := map[string]any{
		"event_type": "event_with_guards",
		"guards": []map[string]any{
			{"field": "sender", "allow_list": []string{"trusted"}},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/event-configurations/event_with_guards", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Delete_RepoError_Returns500(t *testing.T) {
	repo := newStubLinkRepo(eywa.NewLink("whatsapp_message").Build())
	repo.err = errInternal
	app := buildMgmtTestApp(linkCacheDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/event-configurations/whatsapp_message", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Delete_Returns204(t *testing.T) {
	repo := newStubLinkRepo(
		eywa.NewLink("whatsapp_message").Build(),
	)
	app := buildMgmtTestApp(linkCacheDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/event-configurations/whatsapp_message", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 204 {
		t.Errorf("want 204, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Save_EmptyEventType_UsesURLKey_Returns200(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	// body has no event_type → handler defaults to URL key
	body := map[string]any{}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/event-configurations/sms_message", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200 when event_type absent in body, got %d", resp.StatusCode)
	}
}

func TestLinkHandler_Save_EventTypeMismatch_Returns400(t *testing.T) {
	repo := newStubLinkRepo()
	app := buildMgmtTestApp(linkCacheDeps(repo))

	// URL says "whatsapp_message" but body has event_type "other_event" → mismatch
	body := map[string]any{"event_type": "other_event"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/event-configurations/whatsapp_message", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400 for event_type mismatch, got %d", resp.StatusCode)
	}
}
