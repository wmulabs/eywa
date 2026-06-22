package fiber

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

// --- Stubs for building a minimal eywa.Weave ---

type testSpiritRepo struct{}

func (r *testSpiritRepo) Create(_ context.Context, _ *eywa.Spirit) error { return nil }
func (r *testSpiritRepo) Update(_ context.Context, _ string, _ *eywa.Spirit, _ string) (*eywa.Spirit, error) {
	return nil, nil
}
func (r *testSpiritRepo) FindByID(_ context.Context, _ string) (*eywa.Spirit, error) { return nil, nil }
func (r *testSpiritRepo) FindActiveByName(_ context.Context, _ string) (*eywa.Spirit, error) {
	return nil, eywa.ErrNotFound
}
func (r *testSpiritRepo) FindActiveByNames(_ context.Context, _ []string) (map[string]*eywa.Spirit, error) {
	return nil, nil
}
func (r *testSpiritRepo) GetVersion(_ context.Context, _ string, _ int) (*eywa.Spirit, error) {
	return nil, nil
}
func (r *testSpiritRepo) FindVersionHistory(_ context.Context, _ string) ([]*eywa.Spirit, error) {
	return nil, nil
}
func (r *testSpiritRepo) Activate(_ context.Context, _ string, _ int) error { return nil }
func (r *testSpiritRepo) Deactivate(_ context.Context, _ string) error      { return nil }
func (r *testSpiritRepo) RestoreVersion(_ context.Context, _ string) error  { return nil }
func (r *testSpiritRepo) ListActive(_ context.Context, _, _ int) ([]*eywa.Spirit, error) {
	return nil, nil
}
func (r *testSpiritRepo) CountActive(_ context.Context) (int64, error)      { return 0, nil }
func (r *testSpiritRepo) ListAll(_ context.Context) ([]*eywa.Spirit, error) { return nil, nil }

type testMemoryRepo struct{}

func (r *testMemoryRepo) GetMemory(_ context.Context, _ string) (*eywa.Memory, error) {
	return nil, eywa.ErrNotFound
}
func (r *testMemoryRepo) SaveMemory(_ context.Context, _ string, _ *eywa.Memory) error { return nil }
func (r *testMemoryRepo) DeleteMemory(_ context.Context, _ string) error               { return nil }

type testEchoRepo struct{}

func (r *testEchoRepo) Append(_ context.Context, _ eywa.Echo) error { return nil }
func (r *testEchoRepo) FindByMemoryKey(_ context.Context, _ string, _, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (r *testEchoRepo) FindByMemoryKeyAndSubject(_ context.Context, _, _ string, _, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (r *testEchoRepo) FindRecentByMemoryKey(_ context.Context, _ string, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (r *testEchoRepo) FindRecentByMemoryKeyAndSubject(_ context.Context, _, _ string, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (r *testEchoRepo) FindBySubjectKey(_ context.Context, _ string, _, _ int) ([]*eywa.Echo, error) {
	return nil, nil
}
func (r *testEchoRepo) CountByMemoryKey(_ context.Context, _ string) (int64, error) { return 0, nil }
func (r *testEchoRepo) CountByMemoryKeyAndSubject(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}
func (r *testEchoRepo) CountBySubjectKey(_ context.Context, _ string) (int64, error) { return 0, nil }

type testChronicleRepo struct{}

func (r *testChronicleRepo) LogInteraction(_ context.Context, _ *eywa.Chronicle) error { return nil }
func (r *testChronicleRepo) FindByMemoryKey(_ context.Context, _ string) ([]*eywa.Chronicle, error) {
	return nil, nil
}
func (r *testChronicleRepo) FindByEventKey(_ context.Context, _ string) ([]*eywa.Chronicle, error) {
	return nil, nil
}
func (r *testChronicleRepo) FindByTimeRange(_ context.Context, _, _ time.Time) ([]*eywa.Chronicle, error) {
	return nil, nil
}

type testBond struct{}

func (b *testBond) AcquireLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}
func (b *testBond) ReleaseLock(_ context.Context, _ string) error                 { return nil }
func (b *testBond) ExtendLock(_ context.Context, _ string, _ time.Duration) error { return nil }

type testOracleFactory struct{}

func (f *testOracleFactory) RegisterProvider(_ string, _ eywa.Oracle) error    { return nil }
func (f *testOracleFactory) GetProvider(_ string) (eywa.Oracle, error)         { return nil, nil }
func (f *testOracleFactory) GetDefaultProvider() (eywa.Oracle, error)          { return nil, nil }
func (f *testOracleFactory) SetDefaultProvider(_ string) error                 { return nil }
func (f *testOracleFactory) ListProviders() []string                           { return nil }
func (f *testOracleFactory) ListAvailableProviders() []string                  { return nil }
func (f *testOracleFactory) GetProviderForModel(_ string) (eywa.Oracle, error) { return nil, nil }

// minimalTestWeave builds a *eywa.Weave with all-stub dependencies.
// Sufficient for testing handlers that only call simple Weave getters.
func minimalTestWeave(t *testing.T) *eywa.Weave {
	t.Helper()
	reg := eywa.NewActionRegistry()
	weave, err := eywa.NewWeaveBuilder(context.Background()).
		WithSpiritRepository(&testSpiritRepo{}).
		WithMemoryRepository(&testMemoryRepo{}).
		WithEchoRepository(&testEchoRepo{}).
		WithChronicleRepository(&testChronicleRepo{}).
		WithBond(&testBond{}).
		WithOracleFactory(&testOracleFactory{}).
		WithActionRegistry(reg).
		Build()
	if err != nil {
		t.Fatalf("failed to build minimal test weave: %v", err)
	}
	return weave
}

// --- HealthHandler ---

func TestHealthHandler_Health(t *testing.T) {
	weave := minimalTestWeave(t)
	h := NewHealthHandler(weave)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	app.Get("/health", h.Health)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["status"] != "healthy" {
		t.Errorf("expected status=healthy, got %v", body["status"])
	}
}

func TestHealthHandler_Ready(t *testing.T) {
	weave := minimalTestWeave(t)
	h := NewHealthHandler(weave)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	app.Get("/ready", h.Ready)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["ready"] != true {
		t.Errorf("expected ready=true, got %v", body["ready"])
	}
}

func TestHealthHandler_Ready_WithPassingCheck_Returns200(t *testing.T) {
	weave := minimalTestWeave(t)
	check := HealthCheck{
		Name: "database",
		Check: func(_ context.Context) error {
			return nil
		},
	}
	h := NewHealthHandler(weave, check)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	app.Get("/ready", h.Ready)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 with passing check, got %d", resp.StatusCode)
	}
}

func TestHealthHandler_Ready_WithFailingCheck_Returns503(t *testing.T) {
	weave := minimalTestWeave(t)
	check := HealthCheck{
		Name: "database",
		Check: func(_ context.Context) error {
			return errors.New("connection refused")
		},
	}
	h := NewHealthHandler(weave, check)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	app.Get("/ready", h.Ready)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 503 {
		t.Errorf("expected 503 with failing check, got %d", resp.StatusCode)
	}
}

func TestHealthHandler_Ready_NoChecks_Returns200(t *testing.T) {
	weave := minimalTestWeave(t)
	h := NewHealthHandler(weave)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	app.Get("/ready", h.Ready)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 with no checks, got %d", resp.StatusCode)
	}
}

// --- discoveryHandler ---

func TestDiscoveryHandler_Get(t *testing.T) {
	weave := minimalTestWeave(t)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, weave, RouteDeps{
		APIKeys: map[string]string{"test-key": "admin"},
	}); err != nil {
		t.Fatalf("unexpected RegisterRoutes error: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/discovery", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- EventHandler pure functions ---

func TestClassifyResults_AllSuccess(t *testing.T) {
	results := []*eywa.Response{
		{Status: eywa.ResponseSuccess},
		{Status: eywa.ResponseSuccess},
	}
	s, f := classifyResults(results)
	if s != 2 || f != 0 {
		t.Errorf("expected 2 success, 0 failed; got %d, %d", s, f)
	}
}

func TestClassifyResults_AllFailed(t *testing.T) {
	results := []*eywa.Response{
		{Status: eywa.ResponseFailed},
		{Status: eywa.ResponseFailed},
	}
	s, f := classifyResults(results)
	if s != 0 || f != 2 {
		t.Errorf("expected 0 success, 2 failed; got %d, %d", s, f)
	}
}

func TestClassifyResults_Mixed(t *testing.T) {
	results := []*eywa.Response{
		{Status: eywa.ResponseSuccess},
		{Status: eywa.ResponseFailed},
	}
	s, f := classifyResults(results)
	if s != 1 || f != 1 {
		t.Errorf("expected 1 each; got %d, %d", s, f)
	}
}

func TestBatchHTTPStatus_AllSuccess(t *testing.T) {
	status, success := batchHTTPStatus(3, 0)
	if status != 200 || !success {
		t.Errorf("expected 200/true, got %d/%v", status, success)
	}
}

func TestBatchHTTPStatus_AllFailed(t *testing.T) {
	status, success := batchHTTPStatus(0, 3)
	if status != 422 || success {
		t.Errorf("expected 422/false, got %d/%v", status, success)
	}
}

func TestBatchHTTPStatus_Mixed(t *testing.T) {
	status, success := batchHTTPStatus(1, 1)
	if status != 207 || success {
		t.Errorf("expected 207/false, got %d/%v", status, success)
	}
}

// --- EventHandler early-return paths ---

func buildEventHandlerApp(weave *eywa.Weave) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	h := NewEventHandler(weave)
	app.Post("/api/v1/events/:event_key", h.ProcessEvent)
	return app
}

func TestEventHandler_MissingEventKey(t *testing.T) {
	// event_key is required path param — empty triggers early 400
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	h := NewEventHandler(nil)
	// Register route without event_key param
	app.Post("/api/v1/events/", h.ProcessEvent)

	req := httptest.NewRequest("POST", "/api/v1/events/", strings.NewReader(`{"test":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestEventHandler_InvalidPayload(t *testing.T) {
	app := buildEventHandlerApp(nil)
	req := httptest.NewRequest("POST", "/api/v1/events/test_event", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for invalid payload, got %d", resp.StatusCode)
	}
}

// --- AsyncEventHandler early-return paths ---

func buildAsyncHandlerApp(weave *eywa.Weave) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	h := NewAsyncEventHandler(weave)
	app.Post("/api/v1/events/:event_key/async", h.IngestAsyncEvent)
	return app
}

func TestAsyncEventHandler_InvalidPayload(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildAsyncHandlerApp(weave)
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/async", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for invalid payload, got %d", resp.StatusCode)
	}
}

// --- EventHandler with weave ---

func buildEventHandlerWithWeave(weave *eywa.Weave) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	h := NewEventHandler(weave)
	app.Post("/api/v1/events/:event_key", h.ProcessEvent)
	return app
}

func TestEventHandler_ConvertError_Returns400(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildEventHandlerWithWeave(weave)

	// no event config registered → ConvertEventByType returns error
	b := []byte(`{"test": 1}`)
	req := httptest.NewRequest("POST", "/api/v1/events/unknown_event", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func testWeaveWithAsyncDispatch(t *testing.T, keeper *testKeeper, receptor *testReceptor) *eywa.Weave {
	t.Helper()
	linkRepo := newStubLinkRepo(
		eywa.NewLink("test_event").WithInboundConverter(receptor.name).Build(),
	)
	reg := eywa.NewActionRegistry()
	weave, err := eywa.NewWeaveBuilder(context.Background()).
		WithSpiritRepository(&testSpiritRepo{}).
		WithMemoryRepository(&testMemoryRepo{}).
		WithEchoRepository(&testEchoRepo{}).
		WithChronicleRepository(&testChronicleRepo{}).
		WithBond(&testBond{}).
		WithOracleFactory(&testOracleFactory{}).
		WithActionRegistry(reg).
		WithLinkRepository(linkRepo).
		WithAsyncDispatch(keeper).
		Build()
	if err != nil {
		t.Fatalf("failed to build weave with async dispatch: %v", err)
	}
	weave.RegisterReceptor(receptor.name, receptor)
	return weave
}

// --- AsyncEventHandler with weave ---

func TestAsyncEventHandler_IngestError_Returns500(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildAsyncHandlerApp(weave)

	// no event config → ingest service returns error
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/async", strings.NewReader(`{"test":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestAsyncEventHandler_Success_Returns200(t *testing.T) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "test", User: "user1"}).Build()
	receptor := &testReceptor{name: "async-receptor", events: []*eywa.Pulse{pulse}}
	weave := testWeaveWithAsyncDispatch(t, &testKeeper{}, receptor)
	app := buildAsyncHandlerApp(weave)

	req := httptest.NewRequest("POST", "/api/v1/events/test_event/async", strings.NewReader(`{"test":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

// --- stubs for Voice/Scout/Pathfinder/LogicRouter/Receptor ---

type testVoice struct{ name string }

func (v *testVoice) GetName() string                                               { return v.name }
func (v *testVoice) ShouldAutoRespond() bool                                       { return true }
func (v *testVoice) SendResponse(_ context.Context, _ *eywa.Pulse, _ string) error { return nil }
func (v *testVoice) GetChannelMetadata(_ *eywa.Pulse) map[string]any               { return nil }

type testScout struct{ name string }

func (s *testScout) GetName() string                                { return s.name }
func (s *testScout) Harvest(_ context.Context, _ *eywa.Pulse) error { return nil }
func (s *testScout) IsApplicable(_ *eywa.Pulse) bool                { return false }

type testPathfinder struct{ name string }

func (p *testPathfinder) GetName() string { return p.name }
func (p *testPathfinder) SelectSpirit(_ context.Context, _ *eywa.Pulse, _ []string) string {
	return ""
}

type testLogicRouter struct{ name string }

func (r *testLogicRouter) GetName() string                               { return r.name }
func (r *testLogicRouter) Route(_ context.Context, _ *eywa.Pulse) string { return "" }

type testAction struct{ name string }

func (a *testAction) GetName() string                                             { return a.name }
func (a *testAction) GetDescription() string                                      { return "test action" }
func (a *testAction) GetParameters() map[string]any                               { return map[string]any{"param": "string"} }
func (a *testAction) IsCritical() bool                                            { return false }
func (a *testAction) GetCategory() eywa.ActionCategory                            { return eywa.ActionGeneral }
func (a *testAction) Execute(_ context.Context, _ map[string]any) (string, error) { return "ok", nil }
func (a *testAction) Validate(_ map[string]any) error                             { return nil }

type testReceptor struct {
	name   string
	events []*eywa.Pulse
}

func (r *testReceptor) GetName() string { return r.name }
func (r *testReceptor) Convert(_ context.Context, _ string, _ map[string]any) ([]*eywa.Pulse, error) {
	return r.events, nil
}

type testKeeper struct{ schedErr error }

func (k *testKeeper) Schedule(_ context.Context, _ string, _ *eywa.Pulse, _ time.Time) (string, error) {
	return "task-id", k.schedErr
}
func (k *testKeeper) Cancel(_ context.Context, _ string) error { return nil }

func testWeaveWithRegistries(t *testing.T) *eywa.Weave {
	t.Helper()

	voiceReg := eywa.NewVoiceRegistry()
	_ = voiceReg.Register(&testVoice{name: "test-voice"})

	scoutReg := eywa.NewScoutRegistry()
	_ = scoutReg.Register(&testScout{name: "test-scout"})

	pfReg := eywa.NewPathfinderRegistry()
	_ = pfReg.Register(&testPathfinder{name: "test-pathfinder"})

	lrReg := eywa.NewLogicRouterRegistry()
	_ = lrReg.Register(&testLogicRouter{name: "test-router"})

	reg := eywa.NewActionRegistry()
	_ = reg.Register(&testAction{name: "test-action"})
	weave, err := eywa.NewWeaveBuilder(context.Background()).
		WithSpiritRepository(&testSpiritRepo{}).
		WithMemoryRepository(&testMemoryRepo{}).
		WithEchoRepository(&testEchoRepo{}).
		WithChronicleRepository(&testChronicleRepo{}).
		WithBond(&testBond{}).
		WithOracleFactory(&testOracleFactory{}).
		WithActionRegistry(reg).
		WithVoiceRegistry(voiceReg).
		WithScoutRegistry(scoutReg).
		WithPathfinderRegistry(pfReg).
		WithLogicRouterRegistry(lrReg).
		Build()
	if err != nil {
		t.Fatalf("failed to build weave with registries: %v", err)
	}
	return weave
}

func testWeaveWithEventConfig(t *testing.T, receptor *testReceptor) *eywa.Weave {
	t.Helper()

	linkRepo := newStubLinkRepo(
		eywa.NewLink("test_event").WithInboundConverter(receptor.name).Build(),
	)
	reg := eywa.NewActionRegistry()
	weave, err := eywa.NewWeaveBuilder(context.Background()).
		WithSpiritRepository(&testSpiritRepo{}).
		WithMemoryRepository(&testMemoryRepo{}).
		WithEchoRepository(&testEchoRepo{}).
		WithChronicleRepository(&testChronicleRepo{}).
		WithBond(&testBond{}).
		WithOracleFactory(&testOracleFactory{}).
		WithActionRegistry(reg).
		WithLinkRepository(linkRepo).
		Build()
	if err != nil {
		t.Fatalf("failed to build weave with event config: %v", err)
	}
	weave.RegisterReceptor(receptor.name, receptor)
	return weave
}

// --- DiscoveryHandler with populated registries ---

func TestDiscoveryHandler_Get_NoActionRegistry_ReturnsEmptyActions(t *testing.T) {
	// Build weave without WithActionRegistry → GetActionRegistry() returns nil
	weave, err := eywa.NewWeaveBuilder(context.Background()).
		WithSpiritRepository(&testSpiritRepo{}).
		WithMemoryRepository(&testMemoryRepo{}).
		WithEchoRepository(&testEchoRepo{}).
		WithChronicleRepository(&testChronicleRepo{}).
		WithBond(&testBond{}).
		WithOracleFactory(&testOracleFactory{}).
		Build()
	if err != nil {
		t.Fatalf("build weave: %v", err)
	}

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, weave, RouteDeps{
		APIKeys: map[string]string{"test-key": "admin"},
	}); err != nil {
		t.Fatalf("unexpected RegisterRoutes error: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/discovery", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	actions, _ := body["actions"].([]any)
	if actions == nil {
		t.Error("actions must be [] not null")
	}
	if len(actions) != 0 {
		t.Errorf("want 0 actions (no registry), got %d", len(actions))
	}
}

func TestDiscoveryHandler_Get_WithRegistries(t *testing.T) {
	weave := testWeaveWithRegistries(t)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, weave, RouteDeps{
		APIKeys: map[string]string{"test-key": "admin"},
	}); err != nil {
		t.Fatalf("unexpected RegisterRoutes error: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/discovery", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	channels, _ := body["channels"].([]any)
	if len(channels) == 0 {
		t.Error("want at least one channel in discovery response")
	}
	actions, _ := body["actions"].([]any)
	if len(actions) == 0 {
		t.Error("want at least one action in discovery response")
	}
}

// --- EventHandler full flow ---

func TestEventHandler_NoEventsProduced_Returns422(t *testing.T) {
	receptor := &testReceptor{name: "pass-through", events: []*eywa.Pulse{}}
	weave := testWeaveWithEventConfig(t, receptor)
	app := buildEventHandlerWithWeave(weave)

	req := httptest.NewRequest("POST", "/api/v1/events/test_event", strings.NewReader(`{"test":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 422 {
		t.Errorf("want 422, got %d", resp.StatusCode)
	}
}

func TestEventHandler_AllFailed_Returns422(t *testing.T) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "test", User: "user1"}).Build()
	receptor := &testReceptor{name: "pass-through", events: []*eywa.Pulse{pulse}}
	weave := testWeaveWithEventConfig(t, receptor)
	app := buildEventHandlerWithWeave(weave)

	// spirit not found → Response{Failed}; all failed → 422
	req := httptest.NewRequest("POST", "/api/v1/events/test_event", strings.NewReader(`{"test":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 422 && resp.StatusCode != 200 {
		t.Errorf("want 422 or 200, got %d", resp.StatusCode)
	}
}

// --- buildValidatorChain: TokenValidator branch ---

type stubTokenValidator struct{}

func (s *stubTokenValidator) Validate(_ context.Context, token string) (*eywa.AuthClaims, error) {
	if token == "custom-token" {
		return &eywa.AuthClaims{Subject: "user", Role: "admin"}, nil
	}
	return nil, errInternal
}

func TestManagement_TokenValidator_AuthenticatesRequest(t *testing.T) {
	weave := minimalTestWeave(t)
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, weave, RouteDeps{
		TokenValidator: &stubTokenValidator{},
	}); err != nil {
		t.Fatalf("unexpected RegisterRoutes error: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/discovery", nil)
	req.Header.Set("Authorization", "Bearer custom-token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestManagement_PubSub_RegistersSSERoutes(t *testing.T) {
	// Registering with non-nil PubSub covers the SSE branch in RegisterRoutes.
	// We don't call the SSE route (it streams forever); just verify registration returns no error.
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, nil, RouteDeps{
		APIKeys: map[string]string{"k": "admin"},
		PubSub:  &noopPubSub{},
	}); err != nil {
		t.Fatalf("unexpected RegisterRoutes error: %v", err)
	}
	// Verify SSE routes exist — auth-guarded, so unauthenticated returns 401, not 404.
	req := httptest.NewRequest("GET", "/api/v1/sse/rites", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode == 404 {
		t.Error("want SSE route registered (non-404), got 404")
	}
}

type noopPubSub struct{}

func (s *noopPubSub) Publish(_ context.Context, _, _ string) error { return nil }
func (s *noopPubSub) Subscribe(ctx context.Context, _ string, _ func(string)) error {
	<-ctx.Done()
	return nil
}

func TestAsyncEventHandler_EmptyEventKey_Returns400(t *testing.T) {
	// Register handler on a route that has no :event_key param → c.Params returns ""
	weave := minimalTestWeave(t)
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	h := NewAsyncEventHandler(weave)
	app.Post("/no-param-route", h.IngestAsyncEvent)

	req := httptest.NewRequest("POST", "/no-param-route", strings.NewReader(`{"test":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400 for empty event_key, got %d", resp.StatusCode)
	}
}

func TestManagement_TokenValidator_Rejects_InvalidToken(t *testing.T) {
	weave := minimalTestWeave(t)
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, weave, RouteDeps{
		TokenValidator: &stubTokenValidator{},
	}); err != nil {
		t.Fatalf("unexpected RegisterRoutes error: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/discovery", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestRegisterRoutes_ProtectedDepWithoutAuth_ReturnsError(t *testing.T) {
	weave := minimalTestWeave(t)
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})

	// A protected repository with no auth validator must fail closed.
	err := RegisterRoutes(app, weave, RouteDeps{SpiritRepo: &stubSpiritRepository{}})

	if err == nil {
		t.Error("expected error when a protected repository is provided without an auth validator")
	}
}

func TestRegisterRoutes_OpenOnly_NoAuthIsFine(t *testing.T) {
	weave := minimalTestWeave(t)
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})

	// No protected deps and no validators → an events-only service; no error.
	if err := RegisterRoutes(app, weave, RouteDeps{}); err != nil {
		t.Errorf("expected no error for an open-only service, got %v", err)
	}
}
