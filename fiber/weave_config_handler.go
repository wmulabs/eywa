package fiber

import (
	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type weaveConfigHandler struct {
	repo        eywa.WeaveConfigRepository
	configCache *eywa.ConfigCache // nil if not set
}

func newWeaveConfigHandler(repo eywa.WeaveConfigRepository, cache *eywa.ConfigCache) *weaveConfigHandler {
	return &weaveConfigHandler{repo: repo, configCache: cache}
}

func (h *weaveConfigHandler) get(c *fiberlib.Ctx) error {
	cfg, err := h.repo.Find(c.Context())
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(cfg)
}

func (h *weaveConfigHandler) save(c *fiberlib.Ctx) error {
	var cfg eywa.WeaveConfig
	if err := c.BodyParser(&cfg); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if err := cfg.Validate(); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": err.Error()})
	}
	if err := h.repo.Save(c.Context(), &cfg); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(&cfg)
}

func (h *weaveConfigHandler) reload(c *fiberlib.Ctx) error {
	if h.configCache != nil {
		if err := h.configCache.ForceReload(c.Context()); err != nil {
			return resthttp.ErrorResponse(c, err)
		}
	}
	return c.JSON(fiberlib.Map{"status": "reloaded"})
}
