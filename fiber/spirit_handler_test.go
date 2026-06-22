package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

var errInternal = errors.New("internal error")

// --- stubSpiritRepository ---

type stubSpiritRepository struct {
	spirit        *eywa.Spirit
	spirits       []*eywa.Spirit
	count         int64
	createErr     error
	findErr       error
	updateErr     error
	listErr       error
	countErr      error
	activateErr   error
	deactivateErr error
	versionsErr   error
}

func (s *stubSpiritRepository) Create(_ context.Context, spirit *eywa.Spirit) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.spirit = spirit
	return nil
}
func (s *stubSpiritRepository) Update(_ context.Context, _ string, sp *eywa.Spirit, _ string) (*eywa.Spirit, error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if s.spirit == nil {
		s.spirit = sp
	}
	return s.spirit, nil
}
func (s *stubSpiritRepository) FindByID(_ context.Context, _ string) (*eywa.Spirit, error) {
	return s.spirit, s.findErr
}
func (s *stubSpiritRepository) FindActiveByName(_ context.Context, _ string) (*eywa.Spirit, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.spirit == nil {
		return nil, eywa.ErrNotFound
	}
	return s.spirit, nil
}
func (s *stubSpiritRepository) FindActiveByNames(_ context.Context, _ []string) (map[string]*eywa.Spirit, error) {
	return nil, s.findErr
}
func (s *stubSpiritRepository) GetVersion(_ context.Context, _ string, _ int) (*eywa.Spirit, error) {
	return s.spirit, s.findErr
}
func (s *stubSpiritRepository) FindVersionHistory(_ context.Context, _ string) ([]*eywa.Spirit, error) {
	return s.spirits, s.versionsErr
}
func (s *stubSpiritRepository) Activate(_ context.Context, _ string, _ int) error {
	return s.activateErr
}
func (s *stubSpiritRepository) Deactivate(_ context.Context, _ string) error {
	return s.deactivateErr
}
func (s *stubSpiritRepository) RestoreVersion(_ context.Context, _ string) error { return nil }
func (s *stubSpiritRepository) ListActive(_ context.Context, _, _ int) ([]*eywa.Spirit, error) {
	return s.spirits, s.listErr
}
func (s *stubSpiritRepository) CountActive(_ context.Context) (int64, error) {
	return s.count, s.countErr
}
func (s *stubSpiritRepository) ListAll(_ context.Context) ([]*eywa.Spirit, error) {
	return s.spirits, s.listErr
}

// buildSpiritTestApp wires a spirit repo into RegisterRoutes behind API-key auth; requests use
// authedRequest to carry the token.
func buildSpiritTestApp(repo eywa.SpiritRepository, weave *eywa.Weave) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, weave, RouteDeps{SpiritRepo: repo, APIKeys: authedAPIKeys()}); err != nil {
		panic(err)
	}
	return app
}

func validSpiritBody() []byte {
	b, _ := json.Marshal(map[string]any{
		"name":          "test_spirit",
		"system_prompt": "You are a helpful assistant that answers questions.",
		"model_config":  map[string]any{"provider": "openai", "model": "gpt-4"},
	})
	return b
}

// --- CreateSpirit ---

func TestSpiritHandler_Create_Returns201(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("POST", "/api/v1/spirits", bytes.NewReader(validSpiritBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("want 201, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["spirit"] == nil {
		t.Error("expected spirit in response")
	}
}

func TestSpiritHandler_Create_InvalidJSON_Returns400(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("POST", "/api/v1/spirits", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Create_MissingName_Returns400(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	body, _ := json.Marshal(map[string]any{
		"system_prompt": "You are a helpful assistant that answers questions.",
		"model_config":  map[string]any{"provider": "openai", "model": "gpt-4"},
	})
	req := authedRequest("POST", "/api/v1/spirits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Create_RepoError_Returns500(t *testing.T) {
	repo := &stubSpiritRepository{createErr: errInternal}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("POST", "/api/v1/spirits", bytes.NewReader(validSpiritBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- GetSpirit ---

func TestSpiritHandler_Get_Returns200(t *testing.T) {
	spirit := &eywa.Spirit{ID: "s1", Name: "test_spirit"}
	repo := &stubSpiritRepository{spirit: spirit}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("GET", "/api/v1/spirits/test_spirit", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Get_NotFound_Returns404(t *testing.T) {
	repo := &stubSpiritRepository{findErr: eywa.ErrNotFound}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("GET", "/api/v1/spirits/missing", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

// --- UpdateSpirit ---

func TestSpiritHandler_Update_Returns200(t *testing.T) {
	spirit := &eywa.Spirit{ID: "s1", Name: "test_spirit", Version: 2}
	repo := &stubSpiritRepository{spirit: spirit}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	body, _ := json.Marshal(map[string]any{
		"system_prompt": "Updated system prompt for tests.",
		"model_config":  map[string]any{"provider": "openai", "model": "gpt-4"},
	})
	req := authedRequest("PUT", "/api/v1/spirits/test_spirit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Update_InvalidJSON_Returns400(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("PUT", "/api/v1/spirits/test_spirit", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Update_ValidationFails_Returns400(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	// empty system_prompt fails validation
	body, _ := json.Marshal(map[string]any{
		"system_prompt": "",
		"model_config":  map[string]any{"provider": "openai", "model": "gpt-4"},
	})
	req := authedRequest("PUT", "/api/v1/spirits/test_spirit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Update_RepoError_Returns500(t *testing.T) {
	repo := &stubSpiritRepository{updateErr: errInternal}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	body, _ := json.Marshal(map[string]any{
		"system_prompt": "Updated system prompt for tests.",
		"model_config":  map[string]any{"provider": "openai", "model": "gpt-4"},
	})
	req := authedRequest("PUT", "/api/v1/spirits/test_spirit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- DeleteSpirit ---

func TestSpiritHandler_Delete_Returns200(t *testing.T) {
	spirit := &eywa.Spirit{ID: "s1", Name: "test_spirit"}
	repo := &stubSpiritRepository{spirit: spirit}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("DELETE", "/api/v1/spirits/test_spirit", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Delete_NotFound_Returns404(t *testing.T) {
	repo := &stubSpiritRepository{findErr: eywa.ErrNotFound}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("DELETE", "/api/v1/spirits/missing", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Delete_DeactivateError_Returns500(t *testing.T) {
	spirit := &eywa.Spirit{ID: "s1", Name: "test_spirit"}
	repo := &stubSpiritRepository{spirit: spirit, deactivateErr: errInternal}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("DELETE", "/api/v1/spirits/test_spirit", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- ListSpirits ---

func TestSpiritHandler_List_Returns200(t *testing.T) {
	spirits := []*eywa.Spirit{{ID: "s1", Name: "a"}, {ID: "s2", Name: "b"}}
	repo := &stubSpiritRepository{spirits: spirits, count: 2}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("GET", "/api/v1/spirits", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["items"] == nil {
		t.Error("expected items in response")
	}
}

func TestSpiritHandler_List_EmptyRepo_ReturnsEmptySlice(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("GET", "/api/v1/spirits", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_List_CountError_Returns500(t *testing.T) {
	repo := &stubSpiritRepository{countErr: errInternal}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("GET", "/api/v1/spirits", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_List_ListError_Returns500(t *testing.T) {
	repo := &stubSpiritRepository{listErr: errInternal}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("GET", "/api/v1/spirits", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- ActivateSpirit ---

func TestSpiritHandler_Activate_Returns200(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	body, _ := json.Marshal(map[string]int{"version": 2})
	req := authedRequest("POST", "/api/v1/spirits/test_spirit/activate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Activate_InvalidBody_Returns400(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("POST", "/api/v1/spirits/test_spirit/activate", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Activate_ZeroVersion_Returns400(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	body, _ := json.Marshal(map[string]int{"version": 0})
	req := authedRequest("POST", "/api/v1/spirits/test_spirit/activate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Activate_RepoError_Returns500(t *testing.T) {
	repo := &stubSpiritRepository{activateErr: errInternal}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	body, _ := json.Marshal(map[string]int{"version": 1})
	req := authedRequest("POST", "/api/v1/spirits/test_spirit/activate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- DeactivateSpirit ---

func TestSpiritHandler_Deactivate_Returns200(t *testing.T) {
	spirit := &eywa.Spirit{ID: "s1", Name: "test_spirit"}
	repo := &stubSpiritRepository{spirit: spirit}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("POST", "/api/v1/spirits/test_spirit/deactivate", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Deactivate_NotFound_Returns404(t *testing.T) {
	repo := &stubSpiritRepository{findErr: eywa.ErrNotFound}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("POST", "/api/v1/spirits/missing/deactivate", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_Deactivate_DeactivateError_Returns500(t *testing.T) {
	spirit := &eywa.Spirit{ID: "s1", Name: "test_spirit"}
	repo := &stubSpiritRepository{spirit: spirit, deactivateErr: errInternal}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("POST", "/api/v1/spirits/test_spirit/deactivate", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- GetSpiritVersions ---

func TestSpiritHandler_GetVersions_Returns200(t *testing.T) {
	spirits := []*eywa.Spirit{{ID: "s1", Version: 1}, {ID: "s2", Version: 2}}
	repo := &stubSpiritRepository{spirits: spirits}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("GET", "/api/v1/spirits/test_spirit/versions", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["versions"] == nil {
		t.Error("expected versions in response")
	}
}

func TestSpiritHandler_GetVersions_EmptyHistory_ReturnsEmptySlice(t *testing.T) {
	repo := &stubSpiritRepository{}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("GET", "/api/v1/spirits/test_spirit/versions", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestSpiritHandler_GetVersions_RepoError_Returns500(t *testing.T) {
	repo := &stubSpiritRepository{versionsErr: errInternal}
	weave := minimalTestWeave(t)
	app := buildSpiritTestApp(repo, weave)

	req := authedRequest("GET", "/api/v1/spirits/test_spirit/versions", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- RegisterRoutes with async dispatcher ---

func TestRegisterRoutes_WithAsyncDispatcher_RegistersAsyncRoute(t *testing.T) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "test", User: "u"}).Build()
	receptor := &testReceptor{name: "rcpt-async-routes", events: []*eywa.Pulse{pulse}}
	weave := testWeaveWithAsyncDispatch(t, &testKeeper{}, receptor)
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, weave, RouteDeps{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	req := authedRequest("POST", "/api/v1/events/test_event/async", bytes.NewReader([]byte(`{"test":1}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	// route is registered — any response (200/400/422) means it was reached
	if resp.StatusCode == 404 {
		t.Error("want async route registered, got 404")
	}
}

// --- WithInternalMiddleware ---

func TestWithInternalMiddleware_RegistersMiddleware(t *testing.T) {
	// Verify WithInternalMiddleware sets up the option correctly.
	// RegisterRoutes with RitualManager+AsyncDispatcher nil → no /internal route.
	// We just check that calling it with a middleware doesn't panic.
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	called := false
	mw := func(c *fiberlib.Ctx) error {
		called = true
		return c.Next()
	}
	// Register routes including the internal middleware
	if err := RegisterRoutes(app, weave, RouteDeps{InternalMiddleware: []fiberlib.Handler{mw}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// /internal/execute-event is registered when RitualManager != nil
	body, _ := json.Marshal(map[string]any{
		"event_key": "test",
		"event":     map[string]any{"memory_key": "m1"},
	})
	req := authedRequest("POST", "/internal/execute-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	app.Test(req) //nolint:errcheck

	if !called {
		t.Error("expected middleware to be called")
	}
}
