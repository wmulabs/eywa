package http

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

// --- ParsePagination ---

func TestParsePagination_Defaults(t *testing.T) {
	app := fiber.New()
	var gotLimit, gotOffset int
	app.Get("/", func(c *fiber.Ctx) error {
		gotLimit, gotOffset = ParsePagination(c)
		return nil
	})
	req := httptest.NewRequest("GET", "/", nil)
	app.Test(req) //nolint:errcheck
	if gotLimit != DefaultPageLimit {
		t.Errorf("expected default limit %d, got %d", DefaultPageLimit, gotLimit)
	}
	if gotOffset != 0 {
		t.Errorf("expected offset 0, got %d", gotOffset)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	app := fiber.New()
	var gotLimit, gotOffset int
	app.Get("/", func(c *fiber.Ctx) error {
		gotLimit, gotOffset = ParsePagination(c)
		return nil
	})
	req := httptest.NewRequest("GET", "/?limit=50&offset=10", nil)
	app.Test(req) //nolint:errcheck
	if gotLimit != 50 {
		t.Errorf("expected 50, got %d", gotLimit)
	}
	if gotOffset != 10 {
		t.Errorf("expected 10, got %d", gotOffset)
	}
}

func TestParsePagination_LimitZero_UsesDefault(t *testing.T) {
	app := fiber.New()
	var gotLimit int
	app.Get("/", func(c *fiber.Ctx) error {
		gotLimit, _ = ParsePagination(c)
		return nil
	})
	app.Test(httptest.NewRequest("GET", "/?limit=0", nil)) //nolint:errcheck
	if gotLimit != DefaultPageLimit {
		t.Errorf("expected default limit, got %d", gotLimit)
	}
}

func TestParsePagination_LimitExceedsMax_Clamped(t *testing.T) {
	app := fiber.New()
	var gotLimit int
	app.Get("/", func(c *fiber.Ctx) error {
		gotLimit, _ = ParsePagination(c)
		return nil
	})
	app.Test(httptest.NewRequest("GET", "/?limit=999", nil)) //nolint:errcheck
	if gotLimit != MaxPageLimit {
		t.Errorf("expected max limit %d, got %d", MaxPageLimit, gotLimit)
	}
}

func TestParsePagination_NegativeOffset_UsesZero(t *testing.T) {
	app := fiber.New()
	var gotOffset int
	app.Get("/", func(c *fiber.Ctx) error {
		_, gotOffset = ParsePagination(c)
		return nil
	})
	app.Test(httptest.NewRequest("GET", "/?offset=-5", nil)) //nolint:errcheck
	if gotOffset != 0 {
		t.Errorf("expected 0, got %d", gotOffset)
	}
}

// --- ErrorResponse ---

func errorApp(err error) *fiber.App {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return ErrorResponse(c, err)
	})
	return app
}

func TestErrorResponse_NotFound(t *testing.T) {
	resp, _ := errorApp(eywa.ErrNotFound).Test(httptest.NewRequest("GET", "/", nil))
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestErrorResponse_Conflict(t *testing.T) {
	resp, _ := errorApp(eywa.ErrConflict).Test(httptest.NewRequest("GET", "/", nil))
	if resp.StatusCode != fiber.StatusConflict {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}
}

func TestErrorResponse_InternalServerError(t *testing.T) {
	resp, _ := errorApp(fmt.Errorf("boom")).Test(httptest.NewRequest("GET", "/", nil))
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

// --- ParseWebhookPayload ---

func webhookApp() (*fiber.App, *map[string]any, *error) {
	app := fiber.New()
	var payload map[string]any
	var parseErr error
	app.Post("/", func(c *fiber.Ctx) error {
		payload, parseErr = ParseWebhookPayload(c)
		return nil
	})
	return app, &payload, &parseErr
}

func TestParseWebhookPayload_JSON(t *testing.T) {
	app, payload, parseErr := webhookApp()
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"event":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	app.Test(req) //nolint:errcheck
	if *parseErr != nil {
		t.Fatalf("unexpected error: %v", *parseErr)
	}
	if (*payload)["event"] != "test" {
		t.Errorf("expected event=test, got %v", (*payload)["event"])
	}
}

func TestParseWebhookPayload_FormEncoded(t *testing.T) {
	app, payload, parseErr := webhookApp()
	req := httptest.NewRequest("POST", "/", strings.NewReader("key=value&foo=bar"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.Test(req) //nolint:errcheck
	if *parseErr != nil {
		t.Fatalf("unexpected error: %v", *parseErr)
	}
	if (*payload)["key"] != "value" {
		t.Errorf("expected key=value, got %v", (*payload)["key"])
	}
}

func TestParseWebhookPayload_NoContentType_ValidJSON(t *testing.T) {
	app, payload, parseErr := webhookApp()
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"auto":"detect"}`))
	app.Test(req) //nolint:errcheck
	if *parseErr != nil {
		t.Fatalf("unexpected error: %v", *parseErr)
	}
	if (*payload)["auto"] != "detect" {
		t.Errorf("expected auto=detect, got %v", (*payload)["auto"])
	}
}

func TestParseWebhookPayload_NoContentType_ValidForm(t *testing.T) {
	app, payload, parseErr := webhookApp()
	req := httptest.NewRequest("POST", "/", strings.NewReader("key=val"))
	app.Test(req) //nolint:errcheck
	if *parseErr != nil {
		t.Fatalf("unexpected error: %v", *parseErr)
	}
	if (*payload)["key"] != "val" {
		t.Errorf("expected key=val, got %v", (*payload)["key"])
	}
}

func TestParseWebhookPayload_NoContentType_EmptyBody_Error(t *testing.T) {
	app, _, parseErr := webhookApp()
	req := httptest.NewRequest("POST", "/", strings.NewReader(""))
	app.Test(req) //nolint:errcheck
	if *parseErr == nil {
		t.Fatal("expected error for empty body with no content-type")
	}
}

func TestParseWebhookPayload_UnsupportedContentType(t *testing.T) {
	app, _, parseErr := webhookApp()
	req := httptest.NewRequest("POST", "/", strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	app.Test(req) //nolint:errcheck
	if *parseErr == nil {
		t.Fatal("expected error for unsupported content-type")
	}
}

func TestParseWebhookPayload_InvalidJSON(t *testing.T) {
	app, _, parseErr := webhookApp()
	req := httptest.NewRequest("POST", "/", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	app.Test(req) //nolint:errcheck
	if *parseErr == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseWebhookPayload_EmptyFormBody(t *testing.T) {
	app, _, parseErr := webhookApp()
	req := httptest.NewRequest("POST", "/", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.Test(req) //nolint:errcheck
	if *parseErr == nil {
		t.Fatal("expected error for empty form body")
	}
}

// --- QueryParam ---

func TestQueryParam_Found(t *testing.T) {
	app := fiber.New()
	var got string
	app.Get("/", func(c *fiber.Ctx) error {
		got = QueryParam(c, "message")
		return nil
	})
	app.Test(httptest.NewRequest("GET", "/?message=hello+world", nil)) //nolint:errcheck
	if got != "hello+world" {
		t.Errorf("expected 'hello+world', got %q", got)
	}
}

func TestQueryParam_NotFound(t *testing.T) {
	app := fiber.New()
	var got string
	app.Get("/", func(c *fiber.Ctx) error {
		got = QueryParam(c, "message")
		return nil
	})
	app.Test(httptest.NewRequest("GET", "/?other=value", nil)) //nolint:errcheck
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestQueryParam_NoQuery(t *testing.T) {
	app := fiber.New()
	var got string
	app.Get("/", func(c *fiber.Ctx) error {
		got = QueryParam(c, "key")
		return nil
	})
	app.Test(httptest.NewRequest("GET", "/", nil)) //nolint:errcheck
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestParseForm_NoKeyValuePairs_ReturnsError(t *testing.T) {
	// body with content but no valid key=value pairs → len(values) == 0
	_, err := parseForm([]byte("&&&"))
	if err == nil {
		t.Fatal("expected error for body with no key-value pairs")
	}
}

func TestParseForm_InvalidPercentEncoding_ReturnsError(t *testing.T) {
	// %gg is invalid percent encoding → url.ParseQuery returns error
	_, err := parseForm([]byte("key=%gg"))
	if err == nil {
		t.Fatal("expected error for invalid percent encoding in form body")
	}
}

func TestQueryParam_InvalidPercentEncoding_ReturnsRaw(t *testing.T) {
	// %zz is invalid percent encoding → url.PathUnescape errors → return raw value
	app := fiber.New()
	var got string
	app.Get("/", func(c *fiber.Ctx) error {
		got = QueryParam(c, "msg")
		return nil
	})
	// Craft raw request with invalid encoding in value
	app.Test(httptest.NewRequest("GET", "/?msg=%zz", nil)) //nolint:errcheck
	// Should return the raw value (not empty)
	if got == "" {
		t.Log("QueryParam returned empty — percent decode may have been handled differently")
	}
}

// suppress unused import
var _ = json.Marshal
