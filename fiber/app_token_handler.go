package fiber

import (
	"time"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

// appTokenHandler manages revocable app tokens used to authenticate inbound event requests.
type appTokenHandler struct {
	repo eywa.AppTokenRepository
}

func newAppTokenHandler(repo eywa.AppTokenRepository) *appTokenHandler {
	return &appTokenHandler{repo: repo}
}

type createAppTokenRequest struct {
	Name string `json:"name"`
	// TTLSeconds bounds the token's lifetime; 0 (or omitted) means it never expires.
	TTLSeconds int64 `json:"ttl_seconds"`
}

// create mints a token and returns its plaintext secret exactly once.
func (h *appTokenHandler) create(c *fiber.Ctx) error {
	var req createAppTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	secret, token, err := eywa.MintAppToken(req.Name, time.Duration(req.TTLSeconds)*time.Second)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to mint token"})
	}
	if err := h.repo.Create(c.Context(), token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to persist token"})
	}

	// The plaintext secret is returned only here — it is never stored or shown again.
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         token.ID,
		"name":       token.Name,
		"token":      secret,
		"expires_at": token.ExpiresAt,
	})
}

func (h *appTokenHandler) list(c *fiber.Ctx) error {
	tokens, err := h.repo.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tokens"})
	}
	// AppToken.Hash is json:"-", so the secret hash is never serialized.
	return c.JSON(fiber.Map{"items": tokens})
}

func (h *appTokenHandler) revoke(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.repo.Revoke(c.Context(), id); err != nil {
		if err == eywa.ErrNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke token"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
