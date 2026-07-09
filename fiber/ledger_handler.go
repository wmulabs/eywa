package fiber

import (
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type ledgerHandler struct {
	repo eywa.LedgerRepository
}

func newLedgerHandler(repo eywa.LedgerRepository) *ledgerHandler {
	return &ledgerHandler{repo: repo}
}

func (h *ledgerHandler) listMonthUsage(c *fiberlib.Ctx) error {
	month := c.Query("month")
	if month == "" {
		month = time.Now().UTC().Format("2006-01")
	}

	entries, err := h.repo.ListMonthUsage(c.Context(), month)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if entries == nil {
		entries = []eywa.LedgerEntry{}
	}

	var totalTokens int64
	var totalCost float64
	for _, e := range entries {
		totalTokens += e.TokensUsed
		totalCost += e.EstCostUSD
	}

	return c.JSON(fiberlib.Map{
		"month":          month,
		"items":          entries,
		"total_tokens":   totalTokens,
		"total_cost_usd": totalCost,
	})
}

func (h *ledgerHandler) listBudgets(c *fiberlib.Ctx) error {
	budgets, err := h.repo.ListBudgets(c.Context())
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if budgets == nil {
		budgets = []eywa.TokenBudget{}
	}
	return c.JSON(fiberlib.Map{"items": budgets})
}

func (h *ledgerHandler) getBudget(c *fiberlib.Ctx) error {
	spiritName := c.Params("spiritName")
	budget, err := h.repo.GetBudget(c.Context(), spiritName)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	// The repository returns a zero-limit budget when none is configured (not an error).
	if budget.MonthlyTokenLimit == 0 {
		return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "budget not found"})
	}
	return c.JSON(budget)
}

func (h *ledgerHandler) setBudget(c *fiberlib.Ctx) error {
	spiritName := c.Params("spiritName")

	var budget eywa.TokenBudget
	if err := c.BodyParser(&budget); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if budget.SpiritName != "" && budget.SpiritName != spiritName {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "spirit_name in body must match URL parameter"})
	}
	budget.SpiritName = spiritName

	if budget.MonthlyTokenLimit <= 0 {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "monthly_token_limit must be positive"})
	}
	switch budget.OnExceed {
	case "block", "alert":
	case "downgrade":
		if budget.DowngradeModel.Provider == "" || budget.DowngradeModel.Model == "" {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "downgrade_model provider and model are required when on_exceed is downgrade"})
		}
	default:
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "on_exceed must be block, downgrade, or alert"})
	}
	if budget.AlertThreshold < 0 || budget.AlertThreshold > 1 {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "alert_threshold must be between 0 and 1"})
	}

	if err := h.repo.SetBudget(c.Context(), budget); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(budget)
}
