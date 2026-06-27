package fiber

import (
	"net/http"
	"strings"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

// eventAuthMiddleware authenticates an inbound event request. It accepts the request if any configured
// bearer TokenValidator accepts the Authorization token, or any RequestVerifier accepts the full
// request (headers + raw body). On no match it rejects with 401.
func eventAuthMiddleware(validators []eywa.TokenValidator, verifiers []eywa.RequestVerifier) fiberlib.Handler {
	return func(c *fiberlib.Ctx) error {
		ctx := c.UserContext()

		if token, ok := bearerToken(c); ok {
			for _, v := range validators {
				if _, err := v.Validate(ctx, token); err == nil {
					return c.Next()
				}
			}
		}

		if len(verifiers) > 0 {
			req := eywa.VerifiableRequest{
				Method: c.Method(),
				URL:    c.Protocol() + "://" + c.Hostname() + c.OriginalURL(),
				Header: requestHeader(c),
				Body:   c.Body(),
			}
			for _, v := range verifiers {
				if _, err := v.Verify(ctx, req); err == nil {
					return c.Next()
				}
			}
		}

		return c.Status(fiberlib.StatusUnauthorized).JSON(fiberlib.Map{"error": "unauthorized event request"})
	}
}

func bearerToken(c *fiberlib.Ctx) (string, bool) {
	return strings.CutPrefix(c.Get("Authorization"), "Bearer ")
}

func requestHeader(c *fiberlib.Ctx) http.Header {
	h := http.Header{}
	for k, v := range c.Request().Header.All() {
		h.Add(string(k), string(v))
	}
	return h
}
