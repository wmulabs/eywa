package fiber

import (
	"errors"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type httpToolHandler struct {
	repo       eywa.HTTPToolRepository
	testerFunc func(tool eywa.HTTPTool, args map[string]any) (*eywa.HTTPToolTestResult, error)
}

func newHTTPToolHandler(repo eywa.HTTPToolRepository) *httpToolHandler {
	return &httpToolHandler{repo: repo}
}

func (h *httpToolHandler) list(c *fiberlib.Ctx) error {
	tools, err := h.repo.List(c.Context())
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if tools == nil {
		tools = []eywa.HTTPTool{}
	}
	return c.JSON(fiberlib.Map{"items": tools})
}

func (h *httpToolHandler) create(c *fiberlib.Ctx) error {
	var tool eywa.HTTPTool
	if err := c.BodyParser(&tool); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if tool.ID == "" {
		tool.ID = eywa.GenerateID()
	}
	if err := h.repo.Save(c.Context(), tool); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.Status(fiberlib.StatusCreated).JSON(tool)
}

func (h *httpToolHandler) getByID(c *fiberlib.Ctx) error {
	id := c.Params("id")
	tool, err := h.repo.FindByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "http tool not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(tool)
}

func (h *httpToolHandler) update(c *fiberlib.Ctx) error {
	id := c.Params("id")
	var tool eywa.HTTPTool
	if err := c.BodyParser(&tool); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	tool.ID = id
	if err := h.repo.Update(c.Context(), tool); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(tool)
}

func (h *httpToolHandler) delete(c *fiberlib.Ctx) error {
	id := c.Params("id")
	if err := h.repo.Delete(c.Context(), id); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.SendStatus(fiberlib.StatusNoContent)
}

func (h *httpToolHandler) test(c *fiberlib.Ctx) error {
	id := c.Params("id")
	tool, err := h.repo.FindByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "http tool not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}

	var body struct {
		Args map[string]any `json:"args"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.Args == nil {
		body.Args = map[string]any{}
	}

	var result *eywa.HTTPToolTestResult
	if h.testerFunc != nil {
		result, err = h.testerFunc(*tool, body.Args)
	} else {
		executor := eywa.NewHTTPToolExecutor(*tool)
		result, err = executor.Test(c.Context(), body.Args)
	}
	if err != nil {
		return c.Status(fiberlib.StatusBadGateway).JSON(fiberlib.Map{"error": err.Error()})
	}

	return c.JSON(fiberlib.Map{
		"status_code":  result.StatusCode,
		"response":     result.Response,
		"resolved_url": result.ResolvedURL,
		"latency_ms":   result.LatencyMS,
	})
}
