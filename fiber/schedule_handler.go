package fiber

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	"go.opentelemetry.io/otel/trace"
)

const (
	minScheduleDelay = 10 * time.Second
	maxScheduleDelay = 30 * 24 * time.Hour
)

type ScheduleHandler struct {
	weave   *eywa.Weave
	manager eywa.RitualManager
}

func NewScheduleHandler(weave *eywa.Weave) *ScheduleHandler {
	return &ScheduleHandler{
		weave:   weave,
		manager: weave.GetRitualManager(),
	}
}

type scheduleByEventKeyRequest struct {
	ExecuteAt  string                 `json:"execute_at"`
	Payload    map[string]any `json:"payload,omitempty"`
	Recurrence *recurrenceInput       `json:"recurrence,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type recurrenceInput struct {
	Cron     string  `json:"cron"`
	Timezone string  `json:"timezone,omitempty"`
	EndsAt   *string `json:"ends_at,omitempty"`
	MaxRuns  int     `json:"max_runs,omitempty"`
}

type scheduledTaskResponse struct {
	TaskID    string `json:"task_id"`
	MemoryKey string `json:"memory_key"`
	ExecuteAt string `json:"execute_at"`
}

// ScheduleByEventKey handles POST /api/v1/events/:event_key/schedule.
func (h *ScheduleHandler) ScheduleByEventKey(c *fiber.Ctx) error {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Schedule/ScheduleByEventKey")
	defer span.End()

	log := newLogger()
	eventKey := c.Params("event_key")

	var req scheduleByEventKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.ExecuteAt == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "execute_at is required"})
	}

	executeAt, err := time.Parse(time.RFC3339, req.ExecuteAt)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "execute_at must be RFC3339 format"})
	}

	now := eywa.NowUTC()
	if executeAt.Before(now.Add(minScheduleDelay)) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "execute_at must be at least 10 seconds in the future"})
	}
	if executeAt.After(now.Add(maxScheduleDelay)) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "execute_at must be within 30 days"})
	}

	if req.Payload == nil {
		req.Payload = make(map[string]any)
	}

	events, err := h.weave.ConvertEventByType(ctx, eventKey, req.Payload)
	if err != nil {
		log.Errorw("failed to convert event for scheduling", "event_key", eventKey, "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if len(events) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "converter returned no events"})
	}

	recurrence, err := toRecurrenceConfig(req.Recurrence)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	tasks := make([]scheduledTaskResponse, 0, len(events))
	for _, event := range events {
		task, err := h.manager.Schedule(ctx, eywa.RitualRequest{
			EventKey:   eventKey,
			Event:      event,
			ExecuteAt:  executeAt,
			CreatedBy:  "api",
			Recurrence: recurrence,
			Metadata:   req.Metadata,
		})
		if err != nil {
			log.Errorw("failed to schedule task", "event_key", eventKey, "memory_key", event.MemoryKey, "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to schedule task"})
		}
		tasks = append(tasks, scheduledTaskResponse{
			TaskID:    task.ID,
			MemoryKey: task.MemoryKey,
			ExecuteAt: task.ExecuteAt.Format(time.RFC3339),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"tasks": tasks})
}

// Cancel handles DELETE /api/v1/schedule/:id.
func (h *ScheduleHandler) Cancel(c *fiber.Ctx) error {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Schedule/Cancel")
	defer span.End()

	id := c.Params("id")
	memoryKey := c.Query("memory_key")

	if id == "" || memoryKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id and memory_key are required"})
	}

	if err := h.manager.Cancel(ctx, id, memoryKey); err != nil {
		return resthttp.ErrorResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListPending handles GET /api/v1/schedule.
func (h *ScheduleHandler) ListPending(c *fiber.Ctx) error {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Schedule/ListPending")
	defer span.End()

	memoryKey := c.Query("memory_key")
	if memoryKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "memory_key query param is required"})
	}

	limit, offset := resthttp.ParsePagination(c)

	total, err := h.manager.CountByMemoryKey(ctx, memoryKey)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}

	rituals, err := h.manager.ListPendingByMemoryKey(ctx, memoryKey, limit, offset)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}

	if rituals == nil {
		rituals = []*eywa.Ritual{}
	}

	return c.JSON(fiber.Map{
		"items":  rituals,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// HandleExecuteEvent is the Keeper callback handler for POST /internal/execute-event.
func HandleExecuteEvent(weave *eywa.Weave) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/ExecuteEvent")
		defer span.End()

		log := newLogger()

		var payload struct {
			EventKey string      `json:"event_key"`
			Event    *eywa.Pulse `json:"event"`
		}
		if err := c.BodyParser(&payload); err != nil || payload.Event == nil || payload.EventKey == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid scheduled event payload"})
		}

		_, err := weave.ProcessEventByKey(ctx, payload.EventKey, payload.Event)
		if err != nil {
			retriable := eywa.IsRetriable(err)
			log.Errorw("scheduled event processing failed",
				"event_key", payload.EventKey,
				"memory_key", payload.Event.MemoryKey,
				"retriable", retriable,
				"error", err,
			)
			if retriable {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			if taskID, ok := payload.Event.GetMetadataString(eywa.MetadataKeyRitualID); ok && weave.GetRitualManager() != nil {
				if markErr := weave.GetRitualManager().MarkFailed(ctx, taskID, err.Error()); markErr != nil {
					log.Warnw("failed to mark ritual as failed", "task_id", taskID, "error", markErr)
				}
			}
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"discarded": true, "reason": err.Error()})
		}

		return c.SendStatus(fiber.StatusOK)
	}
}

func toRecurrenceConfig(r *recurrenceInput) (*eywa.RecurrenceConfig, error) {
	if r == nil {
		return nil, nil
	}
	rec := &eywa.RecurrenceConfig{
		Cron:     r.Cron,
		Timezone: r.Timezone,
		MaxRuns:  r.MaxRuns,
	}
	if r.EndsAt != nil {
		t, err := time.Parse(time.RFC3339, *r.EndsAt)
		if err != nil {
			return nil, fmt.Errorf("recurrence.ends_at must be RFC3339 format")
		}
		rec.EndsAt = &t
	}
	return rec, nil
}
