package fiber

import (
	"errors"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type vigilHandler struct {
	vigilRepo   eywa.VigilRepository
	vigilConfig eywa.VigilConfig
	echoRepo    eywa.EchoRepository // optional
	eventBus    *EventBus           // optional; nil = no SSE fanout
}

func newVigilHandler(
	vigilRepo eywa.VigilRepository,
	vigilConfig eywa.VigilConfig,
	echoRepo eywa.EchoRepository,
	eventBus *EventBus,
) *vigilHandler {
	return &vigilHandler{
		vigilRepo:   vigilRepo,
		vigilConfig: vigilConfig,
		echoRepo:    echoRepo,
		eventBus:    eventBus,
	}
}

func (h *vigilHandler) ttl() time.Duration {
	if h.vigilConfig.InactivityTimeout > 0 {
		return h.vigilConfig.InactivityTimeout
	}
	return 30 * time.Minute
}

func (h *vigilHandler) takeSeat(c *fiberlib.Ctx) error {
	memoryKey := c.Params("memoryKey")
	var body struct {
		OperatorID string `json:"operator_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.OperatorID == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "operator_id is required"})
	}
	if err := h.vigilRepo.Acquire(c.Context(), memoryKey, body.OperatorID, h.ttl()); err != nil {
		if errors.Is(err, eywa.ErrSessionHeld) {
			return c.Status(fiberlib.StatusConflict).JSON(fiberlib.Map{"error": "seat already held by another operator"})
		}
		return resthttp.ErrorResponse(c, err)
	}
	vigil, err := h.vigilRepo.Get(c.Context(), memoryKey)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	h.eventBus.PublishVigilAcquired(c.Context(), memoryKey, body.OperatorID)
	return c.JSON(toVigilResponse(vigil))
}

func (h *vigilHandler) releaseSeat(c *fiberlib.Ctx) error {
	memoryKey := c.Params("memoryKey")
	if err := h.vigilRepo.Release(c.Context(), memoryKey); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	h.eventBus.PublishVigilReleased(c.Context(), memoryKey)
	return c.JSON(fiberlib.Map{"status": "released"})
}

func (h *vigilHandler) getStatus(c *fiberlib.Ctx) error {
	memoryKey := c.Params("memoryKey")
	vigil, err := h.vigilRepo.Get(c.Context(), memoryKey)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if vigil == nil {
		return c.JSON(fiberlib.Map{"status": "spirit", "memory_key": memoryKey})
	}
	return c.JSON(toVigilResponse(vigil))
}

func (h *vigilHandler) listAll(c *fiberlib.Ctx) error {
	vigils, err := h.vigilRepo.ListAll(c.Context())
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	items := make([]fiberlib.Map, 0, len(vigils))
	for _, v := range vigils {
		items = append(items, toVigilResponse(v))
	}
	return c.JSON(fiberlib.Map{"vigils": items, "total": len(items)})
}

func (h *vigilHandler) sendMessage(c *fiberlib.Ctx) error {
	if h.echoRepo == nil {
		return c.Status(fiberlib.StatusNotImplemented).JSON(fiberlib.Map{"error": "echo repository not configured"})
	}
	memoryKey := c.Params("memoryKey")
	var body struct {
		Content    string `json:"content"`
		OperatorID string `json:"operator_id"`
		SubjectKey string `json:"subject_key"`
		Deliver    bool   `json:"deliver"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.Content == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "content is required"})
	}

	echo := eywa.Echo{
		ID:           eywa.GenerateID(),
		MemoryKey:    memoryKey,
		SubjectKey:   body.SubjectKey,
		Role:         eywa.RoleOperator,
		Content:      body.Content,
		Timestamp:    time.Now().UTC(),
		IsUserFacing: true,
		Metadata:     map[string]any{"operator_id": body.OperatorID},
	}
	if err := h.echoRepo.Append(c.Context(), echo); err != nil {
		return resthttp.ErrorResponse(c, err)
	}

	// Refresh sliding TTL on every operator message.
	if err := h.vigilRepo.Refresh(c.Context(), memoryKey, h.ttl()); err != nil {
		_ = err // non-critical
	}

	h.eventBus.PublishEchoAdded(c.Context(), memoryKey, echo)
	return c.Status(fiberlib.StatusCreated).JSON(echo)
}

func toVigilResponse(v *eywa.Vigil) fiberlib.Map {
	heldFor := int64(0)
	if !v.SeatSince.IsZero() {
		heldFor = int64(time.Since(v.SeatSince).Seconds())
	}
	return fiberlib.Map{
		"status":           "human",
		"memory_key":       v.MemoryKey,
		"operator_id":      v.OperatorID,
		"seat_since":       v.SeatSince,
		"expires_at":       v.ExpiresAt,
		"held_for_seconds": heldFor,
	}
}
