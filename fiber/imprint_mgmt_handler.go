package fiber

import (
	"errors"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type imprintMgmtHandler struct {
	repo eywa.ImprintRepository
}

func newImprintMgmtHandler(repo eywa.ImprintRepository) *imprintMgmtHandler {
	return &imprintMgmtHandler{repo: repo}
}

func (h *imprintMgmtHandler) list(c *fiberlib.Ctx) error {
	opts := eywa.ImprintListOptions{
		UserKey:    c.Query("user_key"),
		SpiritName: c.Query("spirit_name"),
		Category:   c.Query("category"),
		Limit:      c.QueryInt("limit", 50),
		Offset:     c.QueryInt("offset", 0),
	}
	imprints, total, err := h.repo.List(c.Context(), opts)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(fiberlib.Map{
		"imprints": imprints,
		"total":    total,
		"limit":    opts.Limit,
		"offset":   opts.Offset,
	})
}

func (h *imprintMgmtHandler) delete(c *fiberlib.Ctx) error {
	id := c.Params("id")
	if err := h.repo.Delete(c.Context(), id); err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "imprint not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(fiberlib.Map{"success": true})
}
