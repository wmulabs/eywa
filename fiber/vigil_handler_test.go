package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
)

// stubVigilRepo implements eywa.VigilRepository for tests.
type stubVigilRepo struct {
	vigil        *eywa.Vigil
	err          error
	refreshErr   error
	acquireErr   error
	lastOperator string
	released     bool
	refreshed    bool
}

func (s *stubVigilRepo) Acquire(_ context.Context, _, operatorID string, _ time.Duration) error {
	s.lastOperator = operatorID
	if s.acquireErr != nil {
		return s.acquireErr
	}
	now := time.Now().UTC()
	s.vigil = &eywa.Vigil{OperatorID: operatorID, SeatSince: now, ExpiresAt: now.Add(30 * time.Minute)}
	return nil
}
func (s *stubVigilRepo) Release(_ context.Context, _ string) error {
	s.released = true
	s.vigil = nil
	return s.err
}
func (s *stubVigilRepo) Get(_ context.Context, _ string) (*eywa.Vigil, error) {
	return s.vigil, s.err
}
func (s *stubVigilRepo) Refresh(_ context.Context, _ string, _ time.Duration) error {
	s.refreshed = true
	if s.refreshErr != nil {
		return s.refreshErr
	}
	return s.err
}
func (s *stubVigilRepo) ListAll(_ context.Context) ([]*eywa.Vigil, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.vigil != nil {
		return []*eywa.Vigil{s.vigil}, nil
	}
	return []*eywa.Vigil{}, nil
}

// stubEchoRepoForVigil implements eywa.EchoRepository for tests.
type stubEchoRepoForVigil struct {
	appended []eywa.Echo
	err      error
}

func newVigilEchoRepo() *stubEchoRepoForVigil { return &stubEchoRepoForVigil{} }

func (s *stubEchoRepoForVigil) Append(_ context.Context, msg eywa.Echo) error {
	s.appended = append(s.appended, msg)
	return s.err
}
func (s *stubEchoRepoForVigil) FindByMemoryKey(_ context.Context, _ string, _, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForVigil) FindByMemoryKeyAndSubject(_ context.Context, _, _ string, _, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForVigil) FindRecentByMemoryKey(_ context.Context, _ string, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForVigil) FindRecentByMemoryKeyAndSubject(_ context.Context, _, _ string, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForVigil) FindBySubjectKey(_ context.Context, _ string, _, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (s *stubEchoRepoForVigil) CountByMemoryKey(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (s *stubEchoRepoForVigil) CountByMemoryKeyAndSubject(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}
func (s *stubEchoRepoForVigil) CountBySubjectKey(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func vigilDeps(vr *stubVigilRepo, er eywa.EchoRepository) ManagementDeps {
	return ManagementDeps{
		APIKeys:     map[string]string{"test-key": "admin"},
		VigilRepo:   vr,
		VigilConfig: eywa.VigilConfig{InactivityTimeout: 30 * time.Minute},
		EchoRepo:    er,
	}
}

func TestVigilHandler_TakeSeat_Returns200(t *testing.T) {
	repo := &stubVigilRepo{}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if repo.lastOperator != "op-1" {
		t.Errorf("want operator op-1, got %s", repo.lastOperator)
	}
}

func TestVigilHandler_TakeSeat_MissingOperatorID_Returns400(t *testing.T) {
	app := buildMgmtTestApp(vigilDeps(&stubVigilRepo{}, nil))

	body := map[string]any{}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_TakeSeat_AlreadyHeld_Returns409(t *testing.T) {
	now := time.Now().UTC()
	existing := &eywa.Vigil{OperatorID: "op-1", SeatSince: now, ExpiresAt: now.Add(30 * time.Minute)}
	repo := &stubVigilRepo{vigil: existing, acquireErr: eywa.ErrSessionHeld}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	body := map[string]any{"operator_id": "op-2"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 409 {
		t.Errorf("want 409, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_ReleaseSeat_Returns200(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubVigilRepo{vigil: &eywa.Vigil{OperatorID: "op-1", SeatSince: now, ExpiresAt: now.Add(30 * time.Minute)}}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	req := httptest.NewRequest("DELETE", "/api/v1/vigil/user-123", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if !repo.released {
		t.Error("Release must be called")
	}
}

func TestVigilHandler_GetStatus_VigilExists_Returns200(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubVigilRepo{vigil: &eywa.Vigil{
		MemoryKey: "user-123", OperatorID: "op-1",
		SeatSince: now, ExpiresAt: now.Add(30 * time.Minute),
	}}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	req := httptest.NewRequest("GET", "/api/v1/vigil/user-123", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["operator_id"] != "op-1" {
		t.Errorf("want operator_id op-1, got %v", result["operator_id"])
	}
}

func TestVigilHandler_GetStatus_NoVigil_Returns200AgentMode(t *testing.T) {
	repo := &stubVigilRepo{vigil: nil}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	req := httptest.NewRequest("GET", "/api/v1/vigil/user-123", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "spirit" {
		t.Errorf("want status agent when no vigil, got %v", result["status"])
	}
}

func TestVigilHandler_SendMessage_Returns201(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubVigilRepo{vigil: &eywa.Vigil{OperatorID: "op-1", SeatSince: now, ExpiresAt: now.Add(30 * time.Minute)}}
	echoRepo := newVigilEchoRepo()
	app := buildMgmtTestApp(vigilDeps(repo, echoRepo))

	body := map[string]any{
		"content":     "I'm taking over this conversation.",
		"operator_id": "op-1",
		"deliver":     false,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123/echoes", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["role"] != "operator" {
		t.Errorf("want role operator, got %v", result["role"])
	}
	if !repo.refreshed {
		t.Error("Refresh must be called after operator message")
	}
}

func TestVigilHandler_SendMessage_MissingContent_Returns400(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubVigilRepo{vigil: &eywa.Vigil{OperatorID: "op-1", SeatSince: now, ExpiresAt: now.Add(30 * time.Minute)}}
	app := buildMgmtTestApp(vigilDeps(repo, newVigilEchoRepo()))

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123/echoes", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_ListAll_Returns200(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubVigilRepo{vigil: &eywa.Vigil{MemoryKey: "user-123", OperatorID: "op-1", SeatSince: now, ExpiresAt: now.Add(30 * time.Minute)}}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	req := httptest.NewRequest("GET", "/api/v1/vigil", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["vigils"] == nil {
		t.Error("expected vigils in response")
	}
}

func TestVigilHandler_ListAll_Empty_Returns200(t *testing.T) {
	repo := &stubVigilRepo{}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	req := httptest.NewRequest("GET", "/api/v1/vigil", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_ListAll_RepoError_Returns500(t *testing.T) {
	repo := &stubVigilRepo{err: errInternal}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	req := httptest.NewRequest("GET", "/api/v1/vigil", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_TakeSeat_InvalidBody_Returns400(t *testing.T) {
	app := buildMgmtTestApp(vigilDeps(&stubVigilRepo{}, nil))

	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_TakeSeat_AcquireGenericError_Returns500(t *testing.T) {
	repo := &stubVigilRepo{acquireErr: errInternal}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_TakeSeat_GetError_Returns500(t *testing.T) {
	// acquireErr=nil → Acquire succeeds, but err=errInternal → Get fails
	repo := &stubVigilRepo{err: errInternal}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_ReleaseSeat_RepoError_Returns500(t *testing.T) {
	repo := &stubVigilRepo{err: errInternal}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	req := httptest.NewRequest("DELETE", "/api/v1/vigil/user-123", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_GetStatus_RepoError_Returns500(t *testing.T) {
	repo := &stubVigilRepo{err: errInternal}
	app := buildMgmtTestApp(vigilDeps(repo, nil))

	req := httptest.NewRequest("GET", "/api/v1/vigil/user-123", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_SendMessage_NoEchoRepo_Returns501(t *testing.T) {
	app := buildMgmtTestApp(vigilDeps(&stubVigilRepo{}, nil))

	body := map[string]any{"content": "hi", "operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123/echoes", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 501 {
		t.Errorf("want 501, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_SendMessage_InvalidBody_Returns400(t *testing.T) {
	repo := &stubVigilRepo{}
	app := buildMgmtTestApp(vigilDeps(repo, newVigilEchoRepo()))

	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123/echoes", bytes.NewReader([]byte("bad")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_SendMessage_EchoRepoError_Returns500(t *testing.T) {
	repo := &stubVigilRepo{}
	echoRepo := &stubEchoRepoForVigil{err: errInternal}
	app := buildMgmtTestApp(vigilDeps(repo, echoRepo))

	body := map[string]any{"content": "hi", "operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123/echoes", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_SendMessage_RefreshError_Returns201(t *testing.T) {
	// refreshErr set: Refresh fails, but it's non-critical — still returns 201
	now := time.Now().UTC()
	repo := &stubVigilRepo{
		vigil:      &eywa.Vigil{OperatorID: "op-1", SeatSince: now, ExpiresAt: now.Add(30 * time.Minute)},
		refreshErr: errInternal,
	}
	echoRepo := newVigilEchoRepo()
	app := buildMgmtTestApp(vigilDeps(repo, echoRepo))

	body := map[string]any{"content": "hi", "operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/vigil/user-123/echoes", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 201 {
		t.Errorf("want 201 even when Refresh fails, got %d", resp.StatusCode)
	}
}

func TestVigilHandler_Ttl_DefaultValue(t *testing.T) {
	h := &vigilHandler{vigilConfig: eywa.VigilConfig{InactivityTimeout: 0}}
	ttl := h.ttl()
	if ttl != 30*time.Minute {
		t.Errorf("want 30m default ttl, got %v", ttl)
	}
}
