package fiber

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

// stubChronicleQueryRepo is a test double for ChronicleQueryRepository.
type stubChronicleQueryRepo struct {
	chronicles []*eywa.Chronicle
	total      int64
	tokens     []eywa.TokenSeries
	actions    []eywa.ActionStats
	spirits    []eywa.SpiritStats
	err        error
}

func (s *stubChronicleQueryRepo) List(_ context.Context, _ eywa.ChronicleListOptions) ([]*eywa.Chronicle, int64, error) {
	return s.chronicles, s.total, s.err
}
func (s *stubChronicleQueryRepo) FindByID(_ context.Context, id string) (*eywa.Chronicle, error) {
	if s.err != nil {
		return nil, s.err
	}
	for _, c := range s.chronicles {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, eywa.ErrNotFound
}
func (s *stubChronicleQueryRepo) AggregateTokens(_ context.Context, _ string, _, _ time.Time, _ string) ([]eywa.TokenSeries, error) {
	return s.tokens, s.err
}
func (s *stubChronicleQueryRepo) AggregateActions(_ context.Context, _ string, _, _ time.Time) ([]eywa.ActionStats, error) {
	return s.actions, s.err
}
func (s *stubChronicleQueryRepo) AggregateSpirits(_ context.Context, _, _ time.Time) ([]eywa.SpiritStats, error) {
	return s.spirits, s.err
}

func buildMgmtTestApp(deps ManagementDeps) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	RegisterManagementRoutes(app, nil, deps)
	return app
}

func chronicleDeps(repo *stubChronicleQueryRepo) ManagementDeps {
	return ManagementDeps{
		APIKeys:            map[string]string{"test-key": "admin"},
		ChronicleQueryRepo: repo,
	}
}

func TestChronicleHandler_List_Returns200WithItems(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		chronicles: []*eywa.Chronicle{{ID: "abc", MemoryKey: "k1"}},
		total:      1,
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []*eywa.Chronicle `json:"items"`
		Total int64             `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Errorf("want 1 item, got %d", len(body.Items))
	}
	if body.Total != 1 {
		t.Errorf("want total 1, got %d", body.Total)
	}
}

func TestChronicleHandler_List_ReturnsEmptySlice(t *testing.T) {
	stub := &stubChronicleQueryRepo{chronicles: nil, total: 0}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle", nil)
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

func TestChronicleHandler_List_NoAuth_Returns401(t *testing.T) {
	stub := &stubChronicleQueryRepo{}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_FindByID_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		chronicles: []*eywa.Chronicle{{ID: "abc123", MemoryKey: "k1"}},
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle/abc123", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var item eywa.Chronicle
	json.NewDecoder(resp.Body).Decode(&item)
	if item.MemoryKey != "k1" {
		t.Errorf("want memory_key k1, got %q", item.MemoryKey)
	}
}

func TestChronicleHandler_FindByID_NotFound_Returns404(t *testing.T) {
	stub := &stubChronicleQueryRepo{chronicles: nil}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle/notexist", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateTokens_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		tokens: []eywa.TokenSeries{
			{SpiritName: "support", PromptTokens: 100, CompletionTokens: 50},
		},
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []eywa.TokenSeries `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 {
		t.Errorf("want 1 series, got %d", len(body.Data))
	}
}

func TestChronicleHandler_AggregateTokens_MissingDates_Returns400(t *testing.T) {
	app := buildMgmtTestApp(chronicleDeps(&stubChronicleQueryRepo{}))

	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateActions_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		actions: []eywa.ActionStats{
			{ActionName: "search_lore", CallCount: 10, ErrorCount: 1, AvgLatencyMs: 120.5},
		},
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/actions?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []eywa.ActionStats `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 {
		t.Errorf("want 1 action stat, got %d", len(body.Data))
	}
}

func TestChronicleHandler_AggregateSpirits_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		spirits: []eywa.SpiritStats{
			{SpiritName: "support", AvgIterations: 2.3, ErrorRate: 0.05, AvgDurationMs: 850},
		},
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/spirits?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []eywa.SpiritStats `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 {
		t.Errorf("want 1 spirit stat, got %d", len(body.Data))
	}
}

func TestChronicleHandler_Analytics_ReturnsEmptySlice(t *testing.T) {
	stub := &stubChronicleQueryRepo{}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/spirits?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []any `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Data == nil {
		t.Error("data must be empty slice, not null")
	}
}

func TestChronicleHandler_List_WithDateFilters_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{chronicles: []*eywa.Chronicle{{ID: "x"}}, total: 1}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_List_InvalidDateFrom_Returns400(t *testing.T) {
	stub := &stubChronicleQueryRepo{}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle?date_from=not-a-date", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_List_InvalidDateTo_Returns400(t *testing.T) {
	stub := &stubChronicleQueryRepo{}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle?date_from=2026-01-01T00:00:00Z&date_to=not-a-date", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_List_LimitCapped_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{chronicles: []*eywa.Chronicle{}, total: 0}
	app := buildMgmtTestApp(chronicleDeps(stub))

	// limit=200 exceeds MaxPageLimit(100) → should be capped
	req := httptest.NewRequest("GET", "/api/v1/chronicle?limit=200", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_List_RepoError_Returns500(t *testing.T) {
	stub := &stubChronicleQueryRepo{err: errInternal}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateTokens_InvalidDateFrom_Returns400(t *testing.T) {
	app := buildMgmtTestApp(chronicleDeps(&stubChronicleQueryRepo{}))

	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?date_from=bad&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateTokens_RepoError_Returns500(t *testing.T) {
	stub := &stubChronicleQueryRepo{err: errInternal}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateTokens_InvalidDateTo_Returns400(t *testing.T) {
	app := buildMgmtTestApp(chronicleDeps(&stubChronicleQueryRepo{}))

	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?date_from=2026-01-01T00:00:00Z&date_to=bad", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateTokens_NilResult_Returns200WithEmptyData(t *testing.T) {
	stub := &stubChronicleQueryRepo{tokens: nil}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct{ Data []any `json:"data"` }
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Data == nil {
		t.Error("data must not be null when tokens is nil")
	}
}

func TestChronicleHandler_AggregateActions_InvalidDateTo_Returns400(t *testing.T) {
	app := buildMgmtTestApp(chronicleDeps(&stubChronicleQueryRepo{}))

	req := httptest.NewRequest("GET", "/api/v1/analytics/actions?date_from=2026-01-01T00:00:00Z&date_to=bad", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateSpirits_InvalidDateTo_Returns400(t *testing.T) {
	app := buildMgmtTestApp(chronicleDeps(&stubChronicleQueryRepo{}))

	req := httptest.NewRequest("GET", "/api/v1/analytics/spirits?date_from=2026-01-01T00:00:00Z&date_to=bad", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateSpirits_NilResult_Returns200WithEmptyData(t *testing.T) {
	stub := &stubChronicleQueryRepo{spirits: nil}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/spirits?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct{ Data []any `json:"data"` }
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Data == nil {
		t.Error("data must not be null when spirits is nil")
	}
}

func TestChronicleHandler_AggregateActions_EmptyResult_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{actions: nil}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/actions?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var body struct{ Data []any `json:"data"` }
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Data == nil {
		t.Error("data must not be null")
	}
}

func TestChronicleHandler_AggregateActions_RepoError_Returns500(t *testing.T) {
	stub := &stubChronicleQueryRepo{err: errInternal}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/actions?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateSpirits_RepoError_Returns500(t *testing.T) {
	stub := &stubChronicleQueryRepo{err: errInternal}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/spirits?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}
