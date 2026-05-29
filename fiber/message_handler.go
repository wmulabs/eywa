package fiber

import (
	"context"

	"fmt"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	"go.opentelemetry.io/otel/trace/noop"
)

const defaultMessageLimit = 50

// MessageHandler provides paginated echo (message) retrieval.
type MessageHandler struct {
	echoRepo eywa.EchoRepository
}

func NewMessageHandler(echoRepo eywa.EchoRepository) *MessageHandler {
	return &MessageHandler{echoRepo: echoRepo}
}

// GetMessages handles GET /api/v1/messages.
func (h *MessageHandler) GetMessages(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Message/GetMessages")
	defer span.End()

	log := newLogger()

	memoryKey := resthttp.QueryParam(c, "memory_key")
	subjectKey := resthttp.QueryParam(c, "subject_key")
	limit := c.QueryInt("limit", defaultMessageLimit)
	offset := c.QueryInt("offset", 0)

	if memoryKey == "" && subjectKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "memory_key or subject_key is required",
		})
	}

	total, err := h.queryCount(ctx, memoryKey, subjectKey)
	if err != nil {
		log.Errorw("failed to count messages", "memory_key", memoryKey, "subject_key", subjectKey, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "failed to retrieve messages"})
	}

	echoes, err := h.queryMessages(ctx, memoryKey, subjectKey, limit, offset)
	if err != nil {
		log.Errorw("failed to retrieve messages", "memory_key", memoryKey, "subject_key", subjectKey, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "failed to retrieve messages"})
	}

	if echoes == nil {
		echoes = []*eywa.Echo{}
	}

	return c.JSON(fiber.Map{"items": echoes, "total": total, "limit": limit, "offset": offset})
}

func (h *MessageHandler) queryMessages(ctx context.Context, memoryKey, subjectKey string, limit, offset int) ([]*eywa.Echo, error) {
	var (
		msgs []*eywa.Echo
		err  error
	)
	switch {
	case memoryKey != "" && subjectKey != "":
		msgs, err = h.echoRepo.FindByMemoryKeyAndSubject(ctx, memoryKey, subjectKey, limit, offset)
	case memoryKey != "":
		msgs, err = h.echoRepo.FindByMemoryKey(ctx, memoryKey, limit, offset)
	default:
		msgs, err = h.echoRepo.FindBySubjectKey(ctx, subjectKey, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("find messages: %w", err)
	}
	return msgs, nil
}

func (h *MessageHandler) queryCount(ctx context.Context, memoryKey, subjectKey string) (int64, error) {
	var (
		count int64
		err   error
	)
	switch {
	case memoryKey != "" && subjectKey != "":
		count, err = h.echoRepo.CountByMemoryKeyAndSubject(ctx, memoryKey, subjectKey)
	case memoryKey != "":
		count, err = h.echoRepo.CountByMemoryKey(ctx, memoryKey)
	default:
		count, err = h.echoRepo.CountBySubjectKey(ctx, subjectKey)
	}
	if err != nil {
		return 0, fmt.Errorf("count messages: %w", err)
	}
	return count, nil
}
