package http

import "github.com/gofiber/fiber/v2"

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

func ParsePagination(c *fiber.Ctx) (limit, offset int) {
	limit = c.QueryInt("limit", DefaultPageLimit)
	offset = c.QueryInt("offset", 0)
	if limit <= 0 {
		limit = DefaultPageLimit
	} else if limit > MaxPageLimit {
		limit = MaxPageLimit
	}
	if offset < 0 {
		offset = 0
	}
	return
}
