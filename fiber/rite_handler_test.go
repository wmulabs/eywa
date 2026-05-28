package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

type stubRiteRepoForFiber struct {
	rites      map[string]*eywa.Rite
	decideErr  error
	err        error
	lastDecide struct {
		id         string
		operatorID string
		status     eywa.RiteStatus
	}
}

func newStubRiteRepo() *stubRiteRepoForFiber {
	return &stubRiteRepoForFiber{rites: make(map[string]*eywa.Rite)}
}

func (s *stubRiteRepoForFiber) Create(_ context.Context, rite *eywa.Rite) error {
	s.rites[rite.ID] = rite
	return s.err
}
func (s *stubRiteRepoForFiber) FindByID(_ context.Context, id string) (*eywa.Rite, error) {
	r, ok := s.rites[id]
	if !ok {
		return nil, eywa.ErrNotFound
	}
	return r, s.err
}
func (s *stubRiteRepoForFiber) List(_ context.Context, _ eywa.RiteListOptions) ([]*eywa.Rite, int64, error) {
	rites := make([]*eywa.Rite, 0, len(s.rites))
	for _, r := range s.rites {
		rites = append(rites, r)
	}
	return rites, int64(len(rites)), s.err
}
func (s *stubRiteRepoForFiber) Decide(_ context.Context, id, operatorID string, status eywa.RiteStatus) error {
	if s.decideErr != nil {
		return s.decideErr
	}
	s.lastDecide.id = id
	s.lastDecide.operatorID = operatorID
	s.lastDecide.status = status
	if r, ok := s.rites[id]; ok {
		r.Status = status
		r.OperatorID = operatorID
	}
	return nil
}

func riteDeps(rr *stubRiteRepoForFiber) ManagementDeps {
	return ManagementDeps{
		APIKeys:  map[string]string{"test-key": "admin"},
		RiteRepo: rr,
	}
}

func seedRite(repo *stubRiteRepoForFiber, id, memoryKey, eventKey string, status eywa.RiteStatus) *eywa.Rite {
	r := &eywa.Rite{
		ID:          id,
		MemoryKey:   memoryKey,
		EventKey:    eventKey,
		Reason:      "needs approval",
		Status:      status,
		RequestedAt: time.Now().UTC(),
	}
	repo.rites[id] = r
	return r
}

func TestRiteHandler_List_Returns200(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/rites", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	items, _ := result["items"].([]any)
	if len(items) != 1 {
		t.Errorf("want 1 item, got %d", len(items))
	}
}

func TestRiteHandler_GetByID_Returns200(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/rites/rite-1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["id"] != "rite-1" {
		t.Errorf("want id=rite-1, got %v", result["id"])
	}
}

func TestRiteHandler_GetByID_NotFound_Returns404(t *testing.T) {
	repo := newStubRiteRepo()
	app := buildMgmtTestApp(riteDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/rites/unknown", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Approve_Returns200_CallsDecide(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if repo.lastDecide.id != "rite-1" {
		t.Errorf("want Decide called with rite-1, got %s", repo.lastDecide.id)
	}
	if repo.lastDecide.status != eywa.RiteApproved {
		t.Errorf("want status=approved, got %s", repo.lastDecide.status)
	}
}

func TestRiteHandler_Approve_MissingOperatorID_Returns400(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]any{}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Reject_Returns200_CallsDecide(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]any{"operator_id": "op-1", "note": "not authorized"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/reject", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if repo.lastDecide.status != eywa.RiteRejected {
		t.Errorf("want status=rejected, got %s", repo.lastDecide.status)
	}
}

func TestRiteHandler_Approve_NotFound_Returns404(t *testing.T) {
	repo := newStubRiteRepo()
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/unknown/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Approve_InvalidBody_Returns400(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/approve", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Approve_DecideError_Returns500(t *testing.T) {
	repo := newStubRiteRepo()
	repo.decideErr = errInternal
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_List_RepoError_Returns500(t *testing.T) {
	repo := newStubRiteRepo()
	repo.err = errInternal
	app := buildMgmtTestApp(riteDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/rites", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_GetByID_RepoError_Returns500(t *testing.T) {
	repo := newStubRiteRepo()
	// seed a rite but make FindByID return error via err field
	repo.rites["rite-1"] = &eywa.Rite{ID: "rite-1"}
	repo.err = errInternal
	app := buildMgmtTestApp(riteDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/rites/rite-1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Approve_FindByID_GenericError_Returns500(t *testing.T) {
	repo := newStubRiteRepo()
	repo.rites["rite-1"] = &eywa.Rite{ID: "rite-1"}
	repo.err = errInternal
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Approve_WithWeave_Returns200(t *testing.T) {
	repo := newStubRiteRepo()
	// memoryKey with ":" so SplitN returns 2 parts → fires pulse
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	weave := minimalTestWeave(t)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	RegisterManagementRoutes(app, weave, ManagementDeps{
		APIKeys:  map[string]string{"test-key": "admin"},
		RiteRepo: repo,
	})

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Approve_WithWeaveAndNote_Returns200(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-2", "sms:+55199", "order_event", eywa.RitePending)
	weave := minimalTestWeave(t)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	RegisterManagementRoutes(app, weave, ManagementDeps{
		APIKeys:  map[string]string{"test-key": "admin"},
		RiteRepo: repo,
	})

	body := map[string]any{"operator_id": "op-1", "note": "approved with note"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-2/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Approve_WithWeave_InvalidMemoryKey_Returns200(t *testing.T) {
	repo := newStubRiteRepo()
	// memoryKey without ":" → SplitN returns 1 part → skips pulse firing
	seedRite(repo, "rite-3", "nomemorykeyseparator", "order_event", eywa.RitePending)
	weave := minimalTestWeave(t)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	RegisterManagementRoutes(app, weave, ManagementDeps{
		APIKeys:  map[string]string{"test-key": "admin"},
		RiteRepo: repo,
	})

	body := map[string]any{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-3/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}
