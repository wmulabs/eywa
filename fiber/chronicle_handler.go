package fiber

import (
	"fmt"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type chronicleHandler struct {
	repo eywa.ChronicleQueryRepository
}

func newChronicleHandler(repo eywa.ChronicleQueryRepository) *chronicleHandler {
	return &chronicleHandler{repo: repo}
}

func (h *chronicleHandler) list(c *fiberlib.Ctx) error {
	opts := eywa.ChronicleListOptions{
		SpiritName:    c.Query("spirit_name"),
		MemoryKey:     c.Query("memory_key"),
		HasError:      c.QueryBool("has_error"),
		MinIterations: c.QueryInt("min_iterations"),
		Page:          c.QueryInt("page", 1),
		Limit:         c.QueryInt("limit", resthttp.DefaultPageLimit),
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

	items, total, err := h.repo.List(c.Context(), opts)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if items == nil {
		items = []*eywa.Chronicle{}
	}
	return c.JSON(fiberlib.Map{"items": items, "total": total, "page": opts.Page, "limit": opts.Limit})
}

func (h *chronicleHandler) findByID(c *fiberlib.Ctx) error {
	item, err := h.repo.FindByID(c.Context(), c.Params("id"))
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(item)
}

func (h *chronicleHandler) aggregateTokens(c *fiberlib.Ctx) error {
	from, to, err := parseDateRange(c)
	if err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": err.Error()})
	}
	granularity := c.Query("granularity", "day")
	result, err := h.repo.AggregateTokens(c.Context(), c.Query("spirit_name"), from, to, granularity)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if result == nil {
		result = []eywa.TokenSeries{}
	}
	return c.JSON(fiberlib.Map{"data": result})
}

func (h *chronicleHandler) aggregateActions(c *fiberlib.Ctx) error {
	from, to, err := parseDateRange(c)
	if err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": err.Error()})
	}
	result, err := h.repo.AggregateActions(c.Context(), c.Query("spirit_name"), from, to)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if result == nil {
		result = []eywa.ActionStats{}
	}
	return c.JSON(fiberlib.Map{"data": result})
}

func (h *chronicleHandler) aggregateSpirits(c *fiberlib.Ctx) error {
	from, to, err := parseDateRange(c)
	if err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": err.Error()})
	}
	result, err := h.repo.AggregateSpirits(c.Context(), from, to)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if result == nil {
		result = []eywa.SpiritStats{}
	}
	return c.JSON(fiberlib.Map{"data": result})
}

// parseDateRange extracts date_from and date_to from query params (RFC3339).
func parseDateRange(c *fiberlib.Ctx) (from, to time.Time, err error) {
	fromStr := c.Query("date_from")
	toStr := c.Query("date_to")
	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("date_from and date_to are required")
	}
	from, err = time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date_from: %w", err)
	}
	to, err = time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date_to: %w", err)
	}
	return from, to, nil
}
