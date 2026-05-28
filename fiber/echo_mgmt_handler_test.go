package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type stubEchoQueryRepo struct {
	sessions []*eywa.SessionSummary
	total    int64
	echoes   []*eywa.Echo
	err      error
}

func (s *stubEchoQueryRepo) ListSessions(_ context.Context, opts eywa.SessionListOptions) ([]*eywa.SessionSummary, int64, error) {
	if opts.MemoryKey != "" {
		for _, sess := range s.sessions {
			if sess.MemoryKey == opts.MemoryKey {
				return []*eywa.SessionSummary{sess}, 1, s.err
			}
		}
		return []*eywa.SessionSummary{}, 0, s.err
	}
	return s.sessions, s.total, s.err
}
func (s *stubEchoQueryRepo) FindByMemoryKeyBefore(_ context.Context, _, _ string, _ int) ([]*eywa.Echo, error) {
	return s.echoes, s.err
}

type stubEchoRepoForMgmt struct {
	appended []eywa.Echo
	err      error
}

func (s *stubEchoRepoForMgmt) Append(_ context.Context, msg eywa.Echo) error {
	if s.err != nil {
		return s.err
	}
	s.appended = append(s.appended, msg)
	return nil
}
func (s *stubEchoRepoForMgmt) FindByMemoryKey(_ context.Context, _ string, _, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForMgmt) FindByMemoryKeyAndSubject(_ context.Context, _, _ string, _, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForMgmt) FindRecentByMemoryKey(_ context.Context, _ string, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForMgmt) FindRecentByMemoryKeyAndSubject(_ context.Context, _, _ string, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForMgmt) FindBySubjectKey(_ context.Context, _ string, _, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForMgmt) CountByMemoryKey(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (s *stubEchoRepoForMgmt) CountByMemoryKeyAndSubject(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}
func (s *stubEchoRepoForMgmt) CountBySubjectKey(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func echoDeps(queryRepo *stubEchoQueryRepo) ManagementDeps {
	return ManagementDeps{
		APIKeys:       map[string]string{"test-key": "admin"},
		EchoQueryRepo: queryRepo,
	}
}

func echoSendDeps(queryRepo *stubEchoQueryRepo, echoRepo eywa.EchoRepository) ManagementDeps {
	return ManagementDeps{
		APIKeys:       map[string]string{"test-key": "admin"},
		EchoQueryRepo: queryRepo,
		EchoRepo:      echoRepo,
	}
}

func TestEchoMgmtHandler_ListSessions_Returns200(t *testing.T) {
	now := time.Now()
	stub := &stubEchoQueryRepo{
		sessions: []*eywa.SessionSummary{
			{MemoryKey: "whatsapp:+5511999999999", MessageCount: 10, LastMessageAt: now},
		},
		total: 1,
	}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []*eywa.SessionSummary `json:"items"`
		Total int64                  `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Errorf("want 1 session, got %d", len(body.Items))
	}
	if body.Total != 1 {
		t.Errorf("want total 1, got %d", body.Total)
	}
}

func TestEchoMgmtHandler_ListSessions_ReturnsEmptySlice(t *testing.T) {
	stub := &stubEchoQueryRepo{sessions: nil, total: 0}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions", nil)
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
		t.Error("items must be empty slice, not null")
	}
}

func TestEchoMgmtHandler_SessionDetail_Returns200(t *testing.T) {
	now := time.Now()
	stub := &stubEchoQueryRepo{
		sessions: []*eywa.SessionSummary{
			{MemoryKey: "whatsapp:+5511", MessageCount: 5, LastMessageAt: now},
		},
	}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions/whatsapp:+5511", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		MemoryKey    string `json:"memory_key"`
		MessageCount int64  `json:"message_count"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.MemoryKey != "whatsapp:+5511" {
		t.Errorf("want memory_key whatsapp:+5511, got %q", body.MemoryKey)
	}
	if body.MessageCount != 5 {
		t.Errorf("want message_count 5, got %d", body.MessageCount)
	}
}

func TestEchoMgmtHandler_SessionDetail_NotFound_Returns404(t *testing.T) {
	stub := &stubEchoQueryRepo{sessions: nil}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions/whatsapp:+unknown", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListEchoes_MissingMemoryKey_Returns400(t *testing.T) {
	stub := &stubEchoQueryRepo{}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListEchoes_Returns200WithMessages(t *testing.T) {
	stub := &stubEchoQueryRepo{
		echoes: []*eywa.Echo{
			{MemoryKey: "whatsapp:+5511", Role: "user", Content: "hello"},
		},
	}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes?memory_key=whatsapp:+5511", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []*eywa.Echo `json:"items"`
		Limit int          `json:"limit"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Errorf("want 1 echo, got %d", len(body.Items))
	}
	if body.Limit != resthttp.DefaultPageLimit {
		t.Errorf("want limit %d, got %d", resthttp.DefaultPageLimit, body.Limit)
	}
}

func TestEchoMgmtHandler_ListEchoes_ReturnsEmptySlice(t *testing.T) {
	stub := &stubEchoQueryRepo{echoes: nil}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes?memory_key=whatsapp:+5511", nil)
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
		t.Error("items must be empty slice, not null")
	}
}

func TestEchoMgmtHandler_SendMessage_Returns201(t *testing.T) {
	queryRepo := &stubEchoQueryRepo{}
	echoRepo := &stubEchoRepoForMgmt{}
	app := buildMgmtTestApp(echoSendDeps(queryRepo, echoRepo))

	payload := map[string]string{"content": "Hello!", "operator_id": "op-1"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/echoes/sessions/whatsapp:+5511/messages", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["message_id"] == nil {
		t.Error("want message_id in response")
	}
	if len(echoRepo.appended) != 1 {
		t.Fatalf("want 1 echo appended, got %d", len(echoRepo.appended))
	}
	appended := echoRepo.appended[0]
	if appended.Role != "operator" {
		t.Errorf("want role=operator, got %q", appended.Role)
	}
	if appended.MemoryKey != "whatsapp:+5511" {
		t.Errorf("want memory_key=whatsapp:+5511, got %q", appended.MemoryKey)
	}
	if !appended.IsUserFacing {
		t.Error("want is_user_facing=true")
	}
}

func TestEchoMgmtHandler_SendMessage_MissingFields_Returns400(t *testing.T) {
	queryRepo := &stubEchoQueryRepo{}
	echoRepo := &stubEchoRepoForMgmt{}
	app := buildMgmtTestApp(echoSendDeps(queryRepo, echoRepo))

	payload := map[string]string{"content": "Hello!"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/echoes/sessions/whatsapp:+5511/messages", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListSessions_WithDateFrom_Returns200(t *testing.T) {
	stub := &stubEchoQueryRepo{sessions: []*eywa.SessionSummary{{MemoryKey: "m1"}}, total: 1}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListSessions_InvalidDateFrom_Returns400(t *testing.T) {
	app := buildMgmtTestApp(echoDeps(&stubEchoQueryRepo{}))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions?date_from=not-a-date", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListSessions_InvalidDateTo_Returns400(t *testing.T) {
	app := buildMgmtTestApp(echoDeps(&stubEchoQueryRepo{}))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions?date_from=2026-01-01T00:00:00Z&date_to=bad", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListSessions_RepoError_Returns500(t *testing.T) {
	stub := &stubEchoQueryRepo{err: errInternal}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListSessions_NilSessions_Returns200EmptySlice(t *testing.T) {
	stub := &stubEchoQueryRepo{sessions: nil}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var body struct{ Items []any `json:"items"` }
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Items == nil {
		t.Error("items must not be null")
	}
}

func TestEchoMgmtHandler_ListSessions_MaxLimitEnforced(t *testing.T) {
	stub := &stubEchoQueryRepo{}
	app := buildMgmtTestApp(echoDeps(stub))

	// Request limit > max — should not fail
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/echoes/sessions?limit=%d", resthttp.MaxPageLimit+100), nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_SessionDetail_RepoError_Returns500(t *testing.T) {
	stub := &stubEchoQueryRepo{err: errInternal}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions/whatsapp:+5511", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_SendMessage_InvalidBody_Returns400(t *testing.T) {
	queryRepo := &stubEchoQueryRepo{}
	echoRepo := &stubEchoRepoForMgmt{}
	app := buildMgmtTestApp(echoSendDeps(queryRepo, echoRepo))

	req := httptest.NewRequest("POST", "/api/v1/echoes/sessions/whatsapp:+5511/messages", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_SendMessage_AppendError_Returns500(t *testing.T) {
	queryRepo := &stubEchoQueryRepo{}
	echoRepo := &stubEchoRepoForMgmt{err: errInternal}
	app := buildMgmtTestApp(echoSendDeps(queryRepo, echoRepo))

	payload := map[string]string{"content": "Hello!", "operator_id": "op-1"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/echoes/sessions/whatsapp:+5511/messages", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListEchoes_MaxLimitEnforced_Returns200(t *testing.T) {
	stub := &stubEchoQueryRepo{}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/echoes?memory_key=mem1&limit=%d", resthttp.MaxPageLimit+100), nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_SendMessage_NoSubjectKey_DefaultsToDefault(t *testing.T) {
	queryRepo := &stubEchoQueryRepo{}
	echoRepo := &stubEchoRepoForMgmt{}
	app := buildMgmtTestApp(echoSendDeps(queryRepo, echoRepo))

	// No subject_key → defaults to "default"
	payload := map[string]string{"content": "Hello!", "operator_id": "op-1"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/echoes/sessions/whatsapp:+5511/messages", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	if len(echoRepo.appended) == 0 {
		t.Fatal("want echo appended")
	}
	if echoRepo.appended[0].SubjectKey != "default" {
		t.Errorf("want subject_key=default, got %q", echoRepo.appended[0].SubjectKey)
	}
}

func TestEchoMgmtHandler_SendMessage_NilEchoRepo_Returns501(t *testing.T) {
	queryRepo := &stubEchoQueryRepo{}
	app := buildMgmtTestApp(echoSendDeps(queryRepo, nil))

	payload := map[string]string{"content": "Hello!", "operator_id": "op-1"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/echoes/sessions/whatsapp:+5511/messages", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 501 {
		t.Errorf("want 501, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListEchoes_RepoError_Returns500(t *testing.T) {
	stub := &stubEchoQueryRepo{err: errInternal}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes?memory_key=mem1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}
