package fiber

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Async event handler — dispatches to Keeper, returns < 100ms.
type AsyncEventHandler struct {
	ingestionService *eywa.AsyncIngestionService
}

func NewAsyncEventHandler(weave *eywa.Weave) *AsyncEventHandler {
	return &AsyncEventHandler{
		ingestionService: eywa.NewAsyncIngestionService(weave),
	}
}

// IngestAsyncEvent handles POST /api/v1/events/:event_key/async.
func (h *AsyncEventHandler) IngestAsyncEvent(c *fiber.Ctx) error {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/AsyncEvent/IngestAsyncEvent")
	defer span.End()

	log := newLogger()

	eventKey := c.Params("event_key")
	if eventKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "event_key is required in path",
		})
	}

	span.SetAttributes(attribute.String("event.key", eventKey))

	rawPayload, err := resthttp.ParseWebhookPayload(c)
	if err != nil {
		log.Errorw("failed to parse event payload", "error", err, "event_key", eventKey)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	result, err := h.ingestionService.Ingest(ctx, eventKey, rawPayload)
	if err != nil {
		log.Errorw("failed to ingest async event", "error", err, "event_key", eventKey)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	log.Infow("async event dispatched",
		"event_key", eventKey,
		"event_count", result.EventCount,
		"response_time_ms", result.ResponseTime.Milliseconds(),
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":     true,
		"event_count": result.EventCount,
		"event_ids":   result.EventIDs,
		"message":     fmt.Sprintf("%d event(s) accepted for processing", result.EventCount),
	})
}
