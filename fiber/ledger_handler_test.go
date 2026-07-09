package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

type stubLedgerRepository struct {
	entries []eywa.LedgerEntry
	budgets []eywa.TokenBudget
	budget  eywa.TokenBudget
	saved   *eywa.TokenBudget
	err     error
}

func (s *stubLedgerRepository) IncrementUsage(_ context.Context, _ string, _ int64, _ float64) error {
	return s.err
}
func (s *stubLedgerRepository) GetMonthUsage(_ context.Context, _, _ string) (eywa.LedgerEntry, error) {
	return eywa.LedgerEntry{}, s.err
}
func (s *stubLedgerRepository) ListMonthUsage(_ context.Context, _ string) ([]eywa.LedgerEntry, error) {
	return s.entries, s.err
}
func (s *stubLedgerRepository) GetBudget(_ context.Context, spiritName string) (eywa.TokenBudget, error) {
	if s.err != nil {
		return eywa.TokenBudget{}, s.err
	}
	if s.budget.SpiritName == "" {
		return eywa.TokenBudget{SpiritName: spiritName}, nil
	}
	return s.budget, nil
}
func (s *stubLedgerRepository) SetBudget(_ context.Context, budget eywa.TokenBudget) error {
	if s.err != nil {
		return s.err
	}
	s.saved = &budget
	return nil
}
func (s *stubLedgerRepository) ListBudgets(_ context.Context) ([]eywa.TokenBudget, error) {
	return s.budgets, s.err
}

func buildLedgerTestApp(t *testing.T, repo eywa.LedgerRepository) *fiberlib.App {
	t.Helper()
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	err := RegisterRoutes(app, minimalTestWeave(t), RouteDeps{APIKeys: authedAPIKeys(), LedgerRepo: repo})
	if err != nil {
		t.Fatalf("RegisterRoutes: %v", err)
	}
	return app
}

func TestLedgerHandler_ListMonthUsage_TotalsAndDefaults(t *testing.T) {
	repo := &stubLedgerRepository{entries: []eywa.LedgerEntry{
		{SpiritName: "a", Month: "2026-07", TokensUsed: 100, EstCostUSD: 0.5},
		{SpiritName: "b", Month: "2026-07", TokensUsed: 50, EstCostUSD: 0.25},
	}}
	app := buildLedgerTestApp(t, repo)

	resp, err := app.Test(authedRequest("GET", "/api/v1/ledger", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Month        string             `json:"month"`
		Items        []eywa.LedgerEntry `json:"items"`
		TotalTokens  int64              `json:"total_tokens"`
		TotalCostUSD float64            `json:"total_cost_usd"`
	}
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body.Month == "" {
		t.Error("expected default month")
	}
	if body.TotalTokens != 150 || body.TotalCostUSD != 0.75 {
		t.Errorf("totals wrong: tokens=%d cost=%f", body.TotalTokens, body.TotalCostUSD)
	}
}

func TestLedgerHandler_ListBudgets_Returns200(t *testing.T) {
	repo := &stubLedgerRepository{budgets: []eywa.TokenBudget{{SpiritName: "a", MonthlyTokenLimit: 1000}}}
	app := buildLedgerTestApp(t, repo)

	resp, _ := app.Test(authedRequest("GET", "/api/v1/budgets", nil))
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestLedgerHandler_GetBudget_NotConfigured_Returns404(t *testing.T) {
	app := buildLedgerTestApp(t, &stubLedgerRepository{})

	resp, _ := app.Test(authedRequest("GET", "/api/v1/budgets/ghost", nil))
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestLedgerHandler_GetBudget_Found_Returns200(t *testing.T) {
	repo := &stubLedgerRepository{budget: eywa.TokenBudget{SpiritName: "bot", MonthlyTokenLimit: 500}}
	app := buildLedgerTestApp(t, repo)

	resp, _ := app.Test(authedRequest("GET", "/api/v1/budgets/bot", nil))
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestLedgerHandler_SetBudget_Validation(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"valid alert", map[string]any{"monthly_token_limit": 1000, "on_exceed": "alert", "alert_threshold": 0.8}, 200},
		{"valid block", map[string]any{"monthly_token_limit": 1000, "on_exceed": "block"}, 200},
		{"valid downgrade", map[string]any{"monthly_token_limit": 1000, "on_exceed": "downgrade",
			"downgrade_model": map[string]any{"provider": "openai", "model": "gpt-4o-mini"}}, 200},
		{"zero limit", map[string]any{"monthly_token_limit": 0, "on_exceed": "alert"}, 400},
		{"bad on_exceed", map[string]any{"monthly_token_limit": 1000, "on_exceed": "explode"}, 400},
		{"downgrade without model", map[string]any{"monthly_token_limit": 1000, "on_exceed": "downgrade"}, 400},
		{"threshold out of range", map[string]any{"monthly_token_limit": 1000, "on_exceed": "alert", "alert_threshold": 1.5}, 400},
		{"name mismatch", map[string]any{"spirit_name": "other", "monthly_token_limit": 1000, "on_exceed": "alert"}, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := buildLedgerTestApp(t, &stubLedgerRepository{})
			b, _ := json.Marshal(tc.body)
			req := authedRequest("PUT", "/api/v1/budgets/bot", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req)
			if resp.StatusCode != tc.want {
				t.Errorf("want %d, got %d", tc.want, resp.StatusCode)
			}
		})
	}
}

func TestLedgerHandler_RequiresAuth(t *testing.T) {
	app := buildLedgerTestApp(t, &stubLedgerRepository{})

	for _, target := range []string{"/api/v1/ledger", "/api/v1/budgets"} {
		resp, _ := app.Test(plainRequest("GET", target, nil))
		if resp.StatusCode != 401 {
			t.Errorf("%s: want 401 without token, got %d", target, resp.StatusCode)
		}
	}
}
