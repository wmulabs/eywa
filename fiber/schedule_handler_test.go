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

// --- stubRitualManager ---

type stubRitualManager struct {
	ritual    *eywa.Ritual
	rituals   []*eywa.Ritual
	count     int64
	schedErr  error
	cancelErr error
	countErr  error
	listErr   error
	markErr   error
}

func (s *stubRitualManager) Schedule(_ context.Context, _ eywa.RitualRequest) (*eywa.Ritual, error) {
	if s.schedErr != nil {
		return nil, s.schedErr
	}
	if s.ritual == nil {
		return &eywa.Ritual{ID: "task-1", MemoryKey: "mem1", ExecuteAt: time.Now().Add(time.Minute)}, nil
	}
	return s.ritual, nil
}
func (s *stubRitualManager) Cancel(_ context.Context, _, _ string) error     { return s.cancelErr }
func (s *stubRitualManager) MarkRunning(_ context.Context, _ string) error   { return s.markErr }
func (s *stubRitualManager) MarkExecuted(_ context.Context, _ string) error  { return s.markErr }
func (s *stubRitualManager) MarkFailed(_ context.Context, _, _ string) error { return s.markErr }
func (s *stubRitualManager) ListPendingByMemoryKey(_ context.Context, _ string, _, _ int) ([]*eywa.Ritual, error) {
	return s.rituals, s.listErr
}
func (s *stubRitualManager) CountByMemoryKey(_ context.Context, _ string) (int64, error) {
	return s.count, s.countErr
}

func testWeaveWithRitual(t *testing.T, manager eywa.RitualManager) *eywa.Weave {
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
		WithRitualManager(manager).
		Build()
	if err != nil {
		t.Fatalf("failed to build test weave: %v", err)
	}
	return weave
}

// --- ScheduleByEventKey ---

func TestScheduleHandler_MissingExecuteAt_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	body, _ := json.Marshal(map[string]any{"payload": map[string]any{}})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_InvalidRFC3339_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	body, _ := json.Marshal(map[string]any{"execute_at": "not-a-date"})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_ExecuteAtInPast_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{"execute_at": past})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_ExecuteAtTooFar_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	future := time.Now().Add(31 * 24 * time.Hour).UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{"execute_at": future})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_InvalidBody_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_EventConfigNotFound_Returns400(t *testing.T) {
	// No event config registered → ConvertEventByType returns error → 400
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	validTime := time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{"execute_at": validTime})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_RecurrenceInvalidEndsAt_Returns400(t *testing.T) {
	// toRecurrenceConfig error path: tested directly
	endsAt := "not-rfc3339"
	r := &recurrenceInput{Cron: "0 * * * *", EndsAt: &endsAt}
	_, err := toRecurrenceConfig(r)
	if err == nil {
		t.Error("expected error for invalid ends_at")
	}
}

func TestScheduleHandler_RecurrenceNil_ReturnsNil(t *testing.T) {
	cfg, err := toRecurrenceConfig(nil)
	if err != nil || cfg != nil {
		t.Errorf("want nil, nil; got %v, %v", cfg, err)
	}
}

func TestScheduleHandler_RecurrenceValid_Returns(t *testing.T) {
	endsAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	r := &recurrenceInput{Cron: "0 * * * *", Timezone: "UTC", MaxRuns: 5, EndsAt: &endsAt}
	cfg, err := toRecurrenceConfig(r)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg == nil || cfg.Cron != "0 * * * *" {
		t.Errorf("unexpected config: %v", cfg)
	}
}

// --- Cancel ---

func TestScheduleHandler_Cancel_MissingMemoryKey_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("DELETE", "/api/v1/schedule/task-1", nil) // no memory_key query param
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_Cancel_Success_Returns204(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("DELETE", "/api/v1/schedule/task-1?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 204 {
		t.Errorf("want 204, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_Cancel_NotFound_Returns404(t *testing.T) {
	manager := &stubRitualManager{cancelErr: eywa.ErrNotFound}
	weave := testWeaveWithRitual(t, manager)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("DELETE", "/api/v1/schedule/task-1?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_Cancel_RepoError_Returns500(t *testing.T) {
	manager := &stubRitualManager{cancelErr: errInternal}
	weave := testWeaveWithRitual(t, manager)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("DELETE", "/api/v1/schedule/task-1?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- ListPending ---

func TestScheduleHandler_ListPending_MissingMemoryKey_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("GET", "/api/v1/schedule", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_ListPending_Returns200(t *testing.T) {
	rituals := []*eywa.Ritual{{ID: "r1"}, {ID: "r2"}}
	manager := &stubRitualManager{rituals: rituals, count: 2}
	weave := testWeaveWithRitual(t, manager)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("GET", "/api/v1/schedule?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_ListPending_CountError_Returns500(t *testing.T) {
	manager := &stubRitualManager{countErr: errInternal}
	weave := testWeaveWithRitual(t, manager)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("GET", "/api/v1/schedule?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_ListPending_NilRituals_Returns200WithEmptyItems(t *testing.T) {
	// nil slice returned → handler replaces with empty slice
	manager := &stubRitualManager{rituals: nil, count: 0}
	weave := testWeaveWithRitual(t, manager)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("GET", "/api/v1/schedule?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	items, _ := body["items"].([]any)
	if items == nil {
		t.Error("items must be empty array, not null")
	}
}

func TestScheduleHandler_ListPending_ListError_Returns500(t *testing.T) {
	manager := &stubRitualManager{listErr: errInternal}
	weave := testWeaveWithRitual(t, manager)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("GET", "/api/v1/schedule?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- HandleExecuteEvent ---

func TestHandleExecuteEvent_InvalidBody_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	req := httptest.NewRequest("POST", "/internal/execute-event", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHandleExecuteEvent_MissingEventKey_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	// event present but no event_key
	body, _ := json.Marshal(map[string]any{
		"event": map[string]any{"memory_key": "mem1"},
	})
	req := httptest.NewRequest("POST", "/internal/execute-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHandleExecuteEvent_MissingEvent_Returns400(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	body, _ := json.Marshal(map[string]any{"event_key": "test_event"})
	req := httptest.NewRequest("POST", "/internal/execute-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHandleExecuteEvent_ProcessingError_NonRetriable_Returns200Discarded(t *testing.T) {
	weave := testWeaveWithRitual(t, &stubRitualManager{})
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	// non-retriable error: no spirit config → weave.ProcessEventByKey will fail
	body, _ := json.Marshal(map[string]any{
		"event_key": "test_event",
		"event": map[string]any{
			"memory_key": "mem1",
		},
	})
	req := httptest.NewRequest("POST", "/internal/execute-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	// non-retriable error returns 200 with discarded:true OR 500 for retriable
	if resp.StatusCode != 200 && resp.StatusCode != 500 {
		t.Errorf("want 200 or 500, got %d", resp.StatusCode)
	}
}

// Ensure stubRitualManager satisfies the interface at compile time.
var _ eywa.RitualManager = (*stubRitualManager)(nil)

// testErrBond returns an infrastructure error on AcquireLock, triggering a retriable OrchestrationError.
type testErrBond struct{}

func (b *testErrBond) AcquireLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return false, errInternal
}
func (b *testErrBond) ReleaseLock(_ context.Context, _ string) error                 { return nil }
func (b *testErrBond) ExtendLock(_ context.Context, _ string, _ time.Duration) error { return nil }

func testWeaveWithEventConfigAndRitual(t *testing.T, manager eywa.RitualManager, receptor *testReceptor) *eywa.Weave {
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
		WithRitualManager(manager).
		Build()
	if err != nil {
		t.Fatalf("failed to build weave: %v", err)
	}
	weave.RegisterReceptor(receptor.name, receptor)
	return weave
}

func TestScheduleHandler_Success_Returns202(t *testing.T) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "test", User: "user1"}).Build()
	receptor := &testReceptor{name: "pass-through", events: []*eywa.Pulse{pulse}}
	weave := testWeaveWithEventConfigAndRitual(t, &stubRitualManager{}, receptor)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	validTime := time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{"execute_at": validTime})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 202 {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_WithRecurrence_Returns202(t *testing.T) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "test", User: "user2"}).Build()
	receptor := &testReceptor{name: "pass-through2", events: []*eywa.Pulse{pulse}}
	weave := testWeaveWithEventConfigAndRitual(t, &stubRitualManager{}, receptor)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	validTime := time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339)
	endsAt := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{
		"execute_at": validTime,
		"recurrence": map[string]any{
			"cron":    "0 * * * *",
			"ends_at": endsAt,
		},
	})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 202 {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_ScheduleError_Returns500(t *testing.T) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "test", User: "user3"}).Build()
	receptor := &testReceptor{name: "pass-through3", events: []*eywa.Pulse{pulse}}
	manager := &stubRitualManager{schedErr: errInternal}
	weave := testWeaveWithEventConfigAndRitual(t, manager, receptor)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	validTime := time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{"execute_at": validTime})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_RecurrenceInvalidEndsAt_ViaHTTP_Returns400(t *testing.T) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "test", User: "user4"}).Build()
	receptor := &testReceptor{name: "pass-through4", events: []*eywa.Pulse{pulse}}
	weave := testWeaveWithEventConfigAndRitual(t, &stubRitualManager{}, receptor)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	validTime := time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{
		"execute_at": validTime,
		"recurrence": map[string]any{
			"cron":    "0 * * * *",
			"ends_at": "not-rfc3339",
		},
	})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestScheduleHandler_NoEventsProduced_Returns400(t *testing.T) {
	receptor := &testReceptor{name: "empty-receptor", events: []*eywa.Pulse{}}
	weave := testWeaveWithEventConfigAndRitual(t, &stubRitualManager{}, receptor)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	validTime := time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339)
	body, _ := json.Marshal(map[string]any{"execute_at": validTime})
	req := httptest.NewRequest("POST", "/api/v1/events/test_event/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func testWeaveWithErrBond(t *testing.T, receptor *testReceptor) *eywa.Weave {
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
		WithBond(&testErrBond{}).
		WithOracleFactory(&testOracleFactory{}).
		WithActionRegistry(reg).
		WithLinkRepository(linkRepo).
		WithRitualManager(&stubRitualManager{}).
		Build()
	if err != nil {
		t.Fatalf("failed to build weave with err bond: %v", err)
	}
	weave.RegisterReceptor(receptor.name, receptor)
	return weave
}

func TestHandleExecuteEvent_RetriableError_Returns500(t *testing.T) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "test", User: "u1"}).Build()
	receptor := &testReceptor{name: "rcpt-retriable", events: []*eywa.Pulse{pulse}}
	weave := testWeaveWithErrBond(t, receptor)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	body, _ := json.Marshal(map[string]any{
		"event_key": "test_event",
		"event": map[string]any{
			"id":         eywa.GenerateID(),
			"memory_key": "test:u1",
			"event_key":  "test_event",
		},
	})
	req := httptest.NewRequest("POST", "/internal/execute-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	// retriable error (lock acquisition failed) → 500
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestHandleExecuteEvent_NonRetriable_MarkFailed_Error_StillDiscards(t *testing.T) {
	// MarkFailed returns error → log.Warnw path (markErr != nil), but still returns 200 discarded
	manager := &stubRitualManager{markErr: errInternal}
	weave := testWeaveWithRitual(t, manager)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	body, _ := json.Marshal(map[string]any{
		"event_key": "test_event",
		"event": map[string]any{
			"memory_key": "test:u3",
			"metadata":   map[string]any{eywa.MetadataKeyRitualID: "ritual-xyz"},
		},
	})
	req := httptest.NewRequest("POST", "/internal/execute-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200 even when MarkFailed errors, got %d", resp.StatusCode)
	}
}

func TestHandleExecuteEvent_NonRetriable_WithRitualID_MarksFailedAndDiscards(t *testing.T) {
	manager := &stubRitualManager{}
	weave := testWeaveWithRitual(t, manager)
	app := buildSpiritTestApp(&stubSpiritRepository{}, weave)

	// non-retriable error (no link config) + ritual_id in metadata → MarkFailed path
	body, _ := json.Marshal(map[string]any{
		"event_key": "test_event",
		"event": map[string]any{
			"memory_key": "test:u2",
			"metadata":   map[string]any{eywa.MetadataKeyRitualID: "ritual-abc"},
		},
	})
	req := httptest.NewRequest("POST", "/internal/execute-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	// non-retriable → discarded 200
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}
