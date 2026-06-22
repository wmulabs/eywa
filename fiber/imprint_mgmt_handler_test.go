package fiber

import (
	"context"
	"net/http/httptest"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"github.com/wmulabs/eywa/internal/domain/entities"
)

// stubImprintRepo is a configurable ImprintRepository.
type stubImprintRepo struct {
	imprints []entities.Imprint
	total    int64
	listErr  error
	delErr   error
}

func (s *stubImprintRepo) Save(_ context.Context, _ entities.Imprint) error { return nil }
func (s *stubImprintRepo) GetByUserKey(_ context.Context, _, _ string) ([]entities.Imprint, error) {
	return nil, nil
}
func (s *stubImprintRepo) List(_ context.Context, _ eywa.ImprintListOptions) ([]entities.Imprint, int64, error) {
	return s.imprints, s.total, s.listErr
}
func (s *stubImprintRepo) Delete(_ context.Context, _ string) error          { return s.delErr }
func (s *stubImprintRepo) Prune(_ context.Context, _, _ string, _ int) error { return nil }

func imprintDeps(repo eywa.ImprintRepository) RouteDeps {
	return RouteDeps{
		APIKeys:     map[string]string{"test-key": "admin"},
		ImprintRepo: repo,
	}
}

// --- list ---

func TestImprintHandler_List_Returns200(t *testing.T) {
	repo := &stubImprintRepo{imprints: []entities.Imprint{{ID: "i1"}}, total: 1}
	app := buildMgmtTestApp(imprintDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/imprints", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestImprintHandler_List_WithFilters_Returns200(t *testing.T) {
	repo := &stubImprintRepo{}
	app := buildMgmtTestApp(imprintDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/imprints?user_key=u1&spirit_name=s1&category=c1&limit=10&offset=0", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestImprintHandler_List_RepoError_Returns500(t *testing.T) {
	repo := &stubImprintRepo{listErr: errInternal}
	app := buildMgmtTestApp(imprintDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/imprints", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- delete ---

func TestImprintHandler_Delete_Returns200(t *testing.T) {
	repo := &stubImprintRepo{}
	app := buildMgmtTestApp(imprintDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/imprints/imprint-id", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestImprintHandler_Delete_NotFound_Returns404(t *testing.T) {
	repo := &stubImprintRepo{delErr: eywa.ErrNotFound}
	app := buildMgmtTestApp(imprintDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/imprints/missing-id", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestImprintHandler_Delete_RepoError_Returns500(t *testing.T) {
	repo := &stubImprintRepo{delErr: errInternal}
	app := buildMgmtTestApp(imprintDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/imprints/some-id", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}
