package fiber

import (
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type echoMgmtHandler struct {
	queryRepo eywa.EchoQueryRepository
	echoRepo  eywa.EchoRepository
}

func newEchoMgmtHandler(queryRepo eywa.EchoQueryRepository, echoRepo eywa.EchoRepository) *echoMgmtHandler {
	return &echoMgmtHandler{queryRepo: queryRepo, echoRepo: echoRepo}
}

func (h *echoMgmtHandler) listSessions(c *fiberlib.Ctx) error {
	opts := eywa.SessionListOptions{
		Page:  c.QueryInt("page", 1),
		Limit: c.QueryInt("limit", resthttp.DefaultPageLimit),
	}
	if opts.Limit > resthttp.MaxPageLimit {
		opts.Limit = resthttp.MaxPageLimit
	}
	if df := c.Query("date_from"); df != "" {
		t, err := time.Parse(time.RFC3339, df)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_from: use RFC3339"})
		}
		opts.DateFrom = &t
	}
	if dt := c.Query("date_to"); dt != "" {
		t, err := time.Parse(time.RFC3339, dt)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_to: use RFC3339"})
		}
		opts.DateTo = &t
	}

	sessions, total, err := h.queryRepo.ListSessions(c.Context(), opts)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if sessions == nil {
		sessions = []*eywa.SessionSummary{}
	}
	return c.JSON(fiberlib.Map{"items": sessions, "total": total, "page": opts.Page, "limit": opts.Limit})
}

func (h *echoMgmtHandler) sessionDetail(c *fiberlib.Ctx) error {
	memoryKey := c.Params("memoryKey")
	sessions, _, err := h.queryRepo.ListSessions(c.Context(), eywa.SessionListOptions{
		MemoryKey: memoryKey,
		Page:      1,
		Limit:     1,
	})
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if len(sessions) == 0 {
		return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "session not found"})
	}
	return c.JSON(sessions[0])
}

func (h *echoMgmtHandler) sendMessage(c *fiberlib.Ctx) error {
	if h.echoRepo == nil {
		return c.Status(fiberlib.StatusNotImplemented).JSON(fiberlib.Map{"error": "echo repository not configured"})
	}
	memoryKey := c.Params("memoryKey")
	var body struct {
		Content    string `json:"content"`
		OperatorID string `json:"operator_id"`
		SubjectKey string `json:"subject_key"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.Content == "" || body.OperatorID == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "content and operator_id are required"})
	}
	if body.SubjectKey == "" {
		body.SubjectKey = "default"
	}

	id := eywa.GenerateID()
	echo := eywa.Echo{
		ID:           id,
		MemoryKey:    memoryKey,
		SubjectKey:   body.SubjectKey,
		Role:         eywa.RoleOperator,
		Content:      body.Content,
		IsUserFacing: true,
		Metadata:     map[string]any{"operator_id": body.OperatorID},
	}
	if err := h.echoRepo.Append(c.Context(), echo); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.Status(fiberlib.StatusCreated).JSON(fiberlib.Map{"message_id": id})
}

func (h *echoMgmtHandler) listEchoes(c *fiberlib.Ctx) error {
	memoryKey := c.Query("memory_key")
	if memoryKey == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "memory_key is required"})
	}
	beforeID := c.Query("before_id")
	limit := c.QueryInt("limit", resthttp.DefaultPageLimit)
	if limit > resthttp.MaxPageLimit {
		limit = resthttp.MaxPageLimit
	}

	echoes, err := h.queryRepo.FindByMemoryKeyBefore(c.Context(), memoryKey, beforeID, limit)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if echoes == nil {
		echoes = []*eywa.Echo{}
	}
	return c.JSON(fiberlib.Map{"items": echoes, "limit": limit})
}
