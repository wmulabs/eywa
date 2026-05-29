package http

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

// ErrorResponse: ErrNotFound → 404, ErrConflict → 409, default → 500.
func ErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, eywa.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	case errors.Is(err, eywa.ErrConflict):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "internal server error",
		})
	}
}
