package cloudtasks

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/idtoken"
)

// NewCloudTasksOIDCMiddleware verifies Google-signed OIDC tokens from Cloud Tasks.
// Empty audience → no-op middleware (OIDC not configured on the queue).
func NewCloudTasksOIDCMiddleware(audience string) fiber.Handler {
	if audience == "" {
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authorization token"})
		}
		token := strings.TrimPrefix(auth, "Bearer ")

		if _, err := idtoken.Validate(c.Context(), token, audience); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid authorization token"})
		}

		return c.Next()
	}
}
