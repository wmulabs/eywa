package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

type authContextKey struct{}

// AuthMiddleware tries each validator in order. First success sets claims and calls Next.
// All validators must fail for the request to be rejected with 401.
func AuthMiddleware(validators ...eywa.TokenValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		bearer := c.Get("Authorization")
		if bearer == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization header",
			})
		}

		token, found := strings.CutPrefix(bearer, "Bearer ")
		if !found {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "authorization header must use Bearer scheme",
			})
		}

		for _, v := range validators {
			claims, err := v.Validate(c.Context(), token)
			if err == nil {
				c.Locals(authContextKey{}, claims)
				return c.Next()
			}
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or expired token",
		})
	}
}

// RequireRole rejects requests where the authenticated role is not in the allowed list.
// Must be placed after AuthMiddleware in the handler chain.
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		claims := ClaimsFromCtx(c)
		if claims == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing auth claims",
			})
		}
		if _, ok := allowed[claims.Role]; !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "insufficient permissions",
			})
		}
		return c.Next()
	}
}

// ClaimsFromCtx retrieves the authenticated claims set by AuthMiddleware.
// Returns nil if the request was not authenticated.
func ClaimsFromCtx(c *fiber.Ctx) *eywa.AuthClaims {
	claims, _ := c.Locals(authContextKey{}).(*eywa.AuthClaims)
	return claims
}
