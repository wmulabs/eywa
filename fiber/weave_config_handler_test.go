package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

type stubWeaveConfigRepo struct {
	config *eywa.WeaveConfig
	err    error
}

func (s *stubWeaveConfigRepo) Find(_ context.Context) (*eywa.WeaveConfig, error) {
	if s.config == nil {
		return eywa.DefaultWeaveConfig(), s.err
	}
	return s.config, s.err
}

func (s *stubWeaveConfigRepo) Save(_ context.Context, cfg *eywa.WeaveConfig) error {
	s.config = cfg
	return s.err
}

func weaveConfigDeps(repo *stubWeaveConfigRepo) RouteDeps {
	return RouteDeps{
		APIKeys:         map[string]string{"test-key": "admin"},
		WeaveConfigRepo: repo,
	}
}

func TestWeaveConfigHandler_Get_Returns200(t *testing.T) {
	repo := &stubWeaveConfigRepo{config: eywa.DefaultWeaveConfig()}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/admin/engine-config", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["max_actions_per_cycle"] == nil {
		t.Error("want max_actions_per_cycle in response")
	}
}

func TestWeaveConfigHandler_Save_Returns200(t *testing.T) {
	repo := &stubWeaveConfigRepo{}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	cfg := eywa.DefaultWeaveConfig()
	cfg.MaxActionsPerCycle = 42
	b, _ := json.Marshal(cfg)

	req := httptest.NewRequest("PUT", "/api/v1/admin/engine-config", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if repo.config == nil || repo.config.MaxActionsPerCycle != 42 {
		t.Error("expected config to be saved with MaxActionsPerCycle=42")
	}
}

func TestWeaveConfigHandler_Reload_Returns200(t *testing.T) {
	repo := &stubWeaveConfigRepo{}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	req := httptest.NewRequest("POST", "/api/v1/admin/config/reload", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestWeaveConfigHandler_Get_RepoError_Returns500(t *testing.T) {
	repo := &stubWeaveConfigRepo{err: errInternal}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/admin/engine-config", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestWeaveConfigHandler_Save_InvalidBody_Returns400(t *testing.T) {
	repo := &stubWeaveConfigRepo{}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	req := httptest.NewRequest("PUT", "/api/v1/admin/engine-config", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestWeaveConfigHandler_Save_RepoError_Returns500(t *testing.T) {
	repo := &stubWeaveConfigRepo{err: errInternal}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	cfg := eywa.DefaultWeaveConfig()
	b, _ := json.Marshal(cfg)
	req := httptest.NewRequest("PUT", "/api/v1/admin/engine-config", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestWeaveConfigHandler_Save_ValidationError_Returns400(t *testing.T) {
	repo := &stubWeaveConfigRepo{}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	// LockTTL <= 0 triggers validation error
	cfg := eywa.DefaultWeaveConfig()
	cfg.LockTTL = 0
	b, _ := json.Marshal(cfg)
	req := httptest.NewRequest("PUT", "/api/v1/admin/engine-config", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestWeaveConfigHandler_Reload_WithConfigCache_Returns200(t *testing.T) {
	repo := &stubWeaveConfigRepo{}
	linkRepo := newStubLinkRepo()
	cache := eywa.NewConfigCache(linkRepo, nil, nil)
	_ = cache.LoadAll(context.Background())

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	RegisterRoutes(app, nil, RouteDeps{
		APIKeys:         map[string]string{"test-key": "admin"},
		WeaveConfigRepo: repo,
		ConfigCache:     cache,
	})

	req := httptest.NewRequest("POST", "/api/v1/admin/config/reload", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestWeaveConfigHandler_Reload_ConfigCacheError_Returns500(t *testing.T) {
	repo := &stubWeaveConfigRepo{}
	linkRepo := newStubLinkRepo()
	linkRepo.err = errInternal
	cache := eywa.NewConfigCache(linkRepo, nil, nil)

	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	RegisterRoutes(app, nil, RouteDeps{
		APIKeys:         map[string]string{"test-key": "admin"},
		WeaveConfigRepo: repo,
		ConfigCache:     cache,
	})

	req := httptest.NewRequest("POST", "/api/v1/admin/config/reload", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}
