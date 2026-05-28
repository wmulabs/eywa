package fiber

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

// HealthCheck is a named readiness probe function.
// Return nil to signal the component is healthy; return an error to signal failure.
type HealthCheck struct {
	Name  string
	Check func(ctx context.Context) error
}

// HealthHandler serves liveness (/health) and readiness (/ready) probes.
// Register HealthChecks via NewHealthHandler to perform real connectivity tests.
// If no checks are registered the /ready endpoint always returns 200.
type HealthHandler struct {
	weave  *eywa.Weave
	checks []HealthCheck
}

// NewHealthHandler creates a HealthHandler. Pass HealthCheck values to perform
// real component checks on the /ready endpoint.
//
//	h := fiber.NewHealthHandler(weave,
//	    fiber.HealthCheck{Name: "redis", Check: func(ctx context.Context) error {
//	        return redisClient.Ping(ctx).Err()
//	    }},
//	    fiber.HealthCheck{Name: "mongo", Check: func(ctx context.Context) error {
//	        return db.Client().Ping(ctx, nil)
//	    }},
//	)
func NewHealthHandler(weave *eywa.Weave, checks ...HealthCheck) *HealthHandler {
	return &HealthHandler{weave: weave, checks: checks}
}

func (h *HealthHandler) Health(c *fiber.Ctx) error {
	info := h.weave.GetAppInfo()
	return c.JSON(fiber.Map{
		"status":  "healthy",
		"service": info.Name,
		"version": info.Version,
	})
}

func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	log := newLogger()
	info := h.weave.GetAppInfo()

	checks := h.runChecks(ctx)
	allReady := true
	for _, ch := range checks {
		if ch.Status != "ready" {
			allReady = false
			break
		}
	}

	statusCode := fiber.StatusOK
	if !allReady {
		statusCode = fiber.StatusServiceUnavailable
		log.Warnw("readiness check failed", "checks", checks)
	}

	return c.Status(statusCode).JSON(fiber.Map{
		"ready":   allReady,
		"service": info.Name,
		"version": info.Version,
		"checks":  checks,
	})
}

// CheckResult is a single named readiness outcome included in the /ready response body.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (h *HealthHandler) runChecks(ctx context.Context) []CheckResult {
	results := make([]CheckResult, 0, len(h.checks))
	for _, hc := range h.checks {
		r := CheckResult{Name: hc.Name}
		if err := hc.Check(ctx); err != nil {
			r.Status = "not_ready"
			r.Message = err.Error()
		} else {
			r.Status = "ready"
		}
		results = append(results, r)
	}
	return results
}
