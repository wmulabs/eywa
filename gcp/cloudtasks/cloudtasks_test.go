package cloudtasks

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- queuePath ---

func TestQueuePath_FormatsCorrectly(t *testing.T) {
	keeper := &CloudTasksKeeper{
		config: CloudTasksConfig{
			Project:  "my-project",
			Location: "us-central1",
			Queue:    "my-queue",
		},
	}
	want := "projects/my-project/locations/us-central1/queues/my-queue"
	if got := keeper.queuePath(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// --- buildHTTPRequest ---

func TestBuildHTTPRequest_NoOIDC_NoAuthHeader(t *testing.T) {
	keeper := &CloudTasksKeeper{
		config: CloudTasksConfig{
			TargetBaseURL:  "https://my-service.run.app",
			TargetAudience: "",
		},
	}
	body := []byte(`{"key":"val"}`)
	req := keeper.buildHTTPRequest(body)

	if req.Url != "https://my-service.run.app/internal/execute-event" {
		t.Errorf("unexpected URL: %s", req.Url)
	}
	if req.AuthorizationHeader != nil {
		t.Error("expected no auth header when TargetAudience is empty")
	}
	if req.Headers["Content-Type"] != "application/json" {
		t.Error("expected Content-Type header")
	}
	if string(req.Body) != string(body) {
		t.Errorf("unexpected body: %s", req.Body)
	}
}

func TestBuildHTTPRequest_WithOIDC_SetsAuthHeader(t *testing.T) {
	keeper := &CloudTasksKeeper{
		config: CloudTasksConfig{
			TargetBaseURL:       "https://my-service.run.app",
			TargetAudience:      "https://my-service.run.app",
			ServiceAccountEmail: "svc@project.iam.gserviceaccount.com",
		},
	}
	req := keeper.buildHTTPRequest([]byte("{}"))

	if req.AuthorizationHeader == nil {
		t.Fatal("expected AuthorizationHeader to be set")
	}
}

// --- NewCloudTasksOIDCMiddleware ---

func TestOIDCMiddleware_EmptyAudience_Passthrough(t *testing.T) {
	app := fiber.New()
	app.Use(NewCloudTasksOIDCMiddleware(""))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestOIDCMiddleware_MissingBearerToken_Unauthorized(t *testing.T) {
	app := fiber.New()
	app.Use(NewCloudTasksOIDCMiddleware("https://my-audience.run.app"))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Authorization header
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestOIDCMiddleware_InvalidToken_Unauthorized(t *testing.T) {
	app := fiber.New()
	app.Use(NewCloudTasksOIDCMiddleware("https://my-audience.run.app"))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// --- isNotFound ---

func TestIsNotFound_NotFoundError_True(t *testing.T) {
	err := status.Error(codes.NotFound, "not found")
	if !isNotFound(err) {
		t.Error("expected isNotFound=true for NotFound gRPC status")
	}
}

func TestIsNotFound_OtherError_False(t *testing.T) {
	err := status.Error(codes.Internal, "internal error")
	if isNotFound(err) {
		t.Error("expected isNotFound=false for Internal error")
	}
}

func TestIsNotFound_NilError_False(t *testing.T) {
	if isNotFound(nil) {
		t.Error("expected isNotFound=false for nil error")
	}
}
