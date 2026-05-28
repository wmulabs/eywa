package fiber

import (
	"errors"
	"strings"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type riteHandler struct {
	riteRepo eywa.RiteRepository
	weave    *eywa.Weave // nil in tests — skip Pulse firing
	eventBus *EventBus   // optional; nil = no SSE fanout
}

func newRiteHandler(riteRepo eywa.RiteRepository, weave *eywa.Weave, eventBus *EventBus) *riteHandler {
	return &riteHandler{riteRepo: riteRepo, weave: weave, eventBus: eventBus}
}

func (h *riteHandler) list(c *fiberlib.Ctx) error {
	opts := eywa.RiteListOptions{
		MemoryKey: c.Query("memory_key"),
		Status:    eywa.RiteStatus(c.Query("status")),
		Page:      c.QueryInt("page", 1),
		Limit:     c.QueryInt("limit", 20),
	}
	rites, total, err := h.riteRepo.List(c.Context(), opts)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	items := make([]riteResponse, 0, len(rites))
	for _, r := range rites {
		items = append(items, toRiteResponse(r))
	}
	return c.JSON(fiberlib.Map{"items": items, "total": total})
}

func (h *riteHandler) getByID(c *fiberlib.Ctx) error {
	id := c.Params("id")
	rite, err := h.riteRepo.FindByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "rite not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(toRiteResponse(rite))
}

func (h *riteHandler) approve(c *fiberlib.Ctx) error {
	return h.decide(c, eywa.RiteApproved)
}

func (h *riteHandler) reject(c *fiberlib.Ctx) error {
	return h.decide(c, eywa.RiteRejected)
}

func (h *riteHandler) decide(c *fiberlib.Ctx, newStatus eywa.RiteStatus) error {
	id := c.Params("id")
	var body struct {
		OperatorID string `json:"operator_id"`
		Note       string `json:"note"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.OperatorID == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "operator_id is required"})
	}

	rite, err := h.riteRepo.FindByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "rite not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}

	if err := h.riteRepo.Decide(c.Context(), id, body.OperatorID, newStatus); err != nil {
		return resthttp.ErrorResponse(c, err)
	}

	// Fire resume Pulse — skipped when weave is nil (tests, or ops without engine wired).
	if h.weave != nil {
		parts := strings.SplitN(rite.MemoryKey, ":", 2)
		if len(parts) == 2 {
			memKey := eywa.MemoryKey{Channel: parts[0], User: parts[1]}
			pb := eywa.NewPulse(memKey).
				WithEventType(rite.EventKey).
				AddKnowledge("rite_id", rite.ID).
				AddKnowledge("rite_status", string(newStatus)).
				AddKnowledge("rite_context", rite.Context).
				AddMetadata("triggered_by", "rite_decision")
			if body.Note != "" {
				pb = pb.AddKnowledge("rite_note", body.Note)
			}
			_, _ = h.weave.ProcessEventByKey(c.Context(), rite.EventKey, pb.Build())
		}
	}

	h.eventBus.PublishRiteDecided(c.Context(), id, newStatus)
	return c.JSON(fiberlib.Map{"id": id, "status": string(newStatus)})
}

type riteResponse struct {
	ID          string          `json:"id"`
	MemoryKey   string          `json:"memory_key"`
	SubjectKey  string          `json:"subject_key,omitempty"`
	EventKey    string          `json:"event_key"`
	Context     map[string]any  `json:"context,omitempty"`
	Reason      string          `json:"reason"`
	Status      eywa.RiteStatus `json:"status"`
	OperatorID  string          `json:"operator_id,omitempty"`
	RequestedAt any             `json:"requested_at"`
	DecidedAt   any             `json:"decided_at,omitempty"`
	ExpiresAt   any             `json:"expires_at,omitempty"`
}

func toRiteResponse(r *eywa.Rite) riteResponse {
	return riteResponse{
		ID:          r.ID,
		MemoryKey:   r.MemoryKey,
		SubjectKey:  r.SubjectKey,
		EventKey:    r.EventKey,
		Context:     r.Context,
		Reason:      r.Reason,
		Status:      r.Status,
		OperatorID:  r.OperatorID,
		RequestedAt: r.RequestedAt,
		DecidedAt:   r.DecidedAt,
		ExpiresAt:   r.ExpiresAt,
	}
}
