package fiber

import (
	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	"go.opentelemetry.io/otel/trace"
)

// Synchronous webhook event handler.
type EventHandler struct {
	weave *eywa.Weave
}

func NewEventHandler(weave *eywa.Weave) *EventHandler {
	return &EventHandler{weave: weave}
}

// ProcessEvent handles POST /api/v1/events/:event_key synchronously.
//
// Response codes:
//   - 200: all events succeeded
//   - 207: mixed batch
//   - 422: all events failed
//   - 400: invalid payload or unknown event_key
//   - 500: infrastructure failure before processing
func (h *EventHandler) ProcessEvent(c *fiber.Ctx) error {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Event/ProcessEvent")
	defer span.End()

	log := newLogger()

	eventKey := c.Params("event_key")
	if eventKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "event_key path parameter is required",
		})
	}

	payload, err := resthttp.ParseWebhookPayload(c)
	if err != nil {
		log.Warnw("failed to parse webhook payload", "event_key", eventKey, "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	events, err := h.weave.ConvertEventByType(ctx, eventKey, payload)
	if err != nil {
		log.Errorw("event conversion failed", "event_key", eventKey, "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "failed to convert event: " + err.Error(),
		})
	}

	if len(events) == 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"error":   "no events produced from payload",
		})
	}

	results, err := h.weave.ProcessMultipleEventsByKey(ctx, eventKey, events)
	if err != nil {
		log.Errorw("event processing failed", "event_key", eventKey, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "failed to process events: " + err.Error(),
		})
	}

	successCount, failedCount := classifyResults(results)

	log.Infow("events processed", "event_key", eventKey, "total", len(results), "success", successCount, "failed", failedCount)

	httpStatus, success := batchHTTPStatus(successCount, failedCount)
	return c.Status(httpStatus).JSON(fiber.Map{
		"success": success,
		"count":   len(results),
		"failed":  failedCount,
		"results": results,
	})
}

func classifyResults(results []*eywa.Response) (successCount, failedCount int) {
	for _, r := range results {
		if r.Status == eywa.ResponseFailed {
			failedCount++
		} else {
			successCount++
		}
	}
	return
}

func batchHTTPStatus(successCount, failedCount int) (httpStatus int, success bool) {
	switch {
	case failedCount == 0:
		return fiber.StatusOK, true
	case successCount == 0:
		return fiber.StatusUnprocessableEntity, false
	default:
		return fiber.StatusMultiStatus, false
	}
}
