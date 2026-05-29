package middleware

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

type stubValidator struct {
	claims *eywa.AuthClaims
	err    error
}

func (v *stubValidator) Validate(_ context.Context, _ string) (*eywa.AuthClaims, error) {
	return v.claims, v.err
}

func buildTestApp(validators ...eywa.TokenValidator) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(AuthMiddleware(validators...))
	app.Get("/", func(c *fiber.Ctx) error {
		claims := ClaimsFromCtx(c)
		if claims == nil {
			return c.SendString("no-claims")
		}
		return c.SendString(claims.Role)
	})
	return app
}

func TestAuthMiddleware_MissingHeader_Returns401(t *testing.T) {
	app := buildTestApp(&stubValidator{claims: &eywa.AuthClaims{Role: "admin"}})
	req := httptest.NewRequest("GET", "/", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_NonBearerScheme_Returns401(t *testing.T) {
	app := buildTestApp(&stubValidator{claims: &eywa.AuthClaims{Role: "admin"}})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic abc123")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("want 401 for non-Bearer scheme, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_ValidToken_SetsClaimsAndCallsNext(t *testing.T) {
	app := buildTestApp(&stubValidator{claims: &eywa.AuthClaims{Role: "admin"}})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "admin" {
		t.Errorf("want body 'admin', got %q", string(body))
	}
}

func TestAuthMiddleware_InvalidToken_Returns401(t *testing.T) {
	app := buildTestApp(&stubValidator{err: fmt.Errorf("invalid")})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_FirstValidatorWins(t *testing.T) {
	failing := &stubValidator{err: fmt.Errorf("nope")}
	succeeding := &stubValidator{claims: &eywa.AuthClaims{Role: "operator"}}
	app := buildTestApp(failing, succeeding)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200 (second validator succeeds), got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "operator" {
		t.Errorf("want body 'operator', got %q", string(body))
	}
}

func TestRequireRole_AllowedRole_PassesThrough(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(AuthMiddleware(&stubValidator{claims: &eywa.AuthClaims{Role: "admin"}}))
	app.Get("/admin", RequireRole("admin"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestRequireRole_InsufficientRole_Returns403(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(AuthMiddleware(&stubValidator{claims: &eywa.AuthClaims{Role: "operator"}}))
	app.Get("/admin", RequireRole("admin"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 403 {
		t.Errorf("want 403, got %d", resp.StatusCode)
	}
}

func TestRequireRole_NilClaims_Returns401(t *testing.T) {
	// No AuthMiddleware — claims are nil in context
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/admin", RequireRole("admin"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req := httptest.NewRequest("GET", "/admin", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("want 401 for nil claims, got %d", resp.StatusCode)
	}
}

func TestRequireRole_MultipleAllowedRoles(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(AuthMiddleware(&stubValidator{claims: &eywa.AuthClaims{Role: "operator"}}))
	app.Get("/ops", RequireRole("admin", "operator"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req := httptest.NewRequest("GET", "/ops", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}
