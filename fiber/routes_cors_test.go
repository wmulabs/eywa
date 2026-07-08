package fiber

import (
	"net/http"
	"net/http/httptest"
	"testing"

	fiberlib "github.com/gofiber/fiber/v2"
)

func corsTestApp(t *testing.T, origins []string) *fiberlib.App {
	t.Helper()
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	err := RegisterRoutes(app, minimalTestWeave(t), RouteDeps{
		APIKeys:     authedAPIKeys(),
		SpiritRepo:  &stubSpiritRepository{},
		CORSOrigins: origins,
	})
	if err != nil {
		t.Fatalf("RegisterRoutes: %v", err)
	}
	return app
}

func TestRegisterRoutes_CORSOrigins_PreflightAllowed(t *testing.T) {
	app := corsTestApp(t, []string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/spirits", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Errorf("preflight status = %d, want 204/200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Allow-Origin = %q, want the requesting origin", got)
	}
}

func TestRegisterRoutes_CORSOrigins_HeaderOnResponse(t *testing.T) {
	app := corsTestApp(t, []string{"http://localhost:3000"})

	req := authedRequest("GET", "/api/v1/spirits", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Allow-Origin = %q, want the requesting origin", got)
	}
}

func TestRegisterRoutes_NoCORSOrigins_NoCORSHeaders(t *testing.T) {
	app := corsTestApp(t, nil)

	req := authedRequest("GET", "/api/v1/spirits", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty when CORS is off", got)
	}
}
