package http

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ParseWebhookPayload: supports application/json and application/x-www-form-urlencoded.
func ParseWebhookPayload(c *fiber.Ctx) (map[string]any, error) {
	ct := strings.ToLower(strings.TrimSpace(c.Get("Content-Type")))

	switch {
	case strings.HasPrefix(ct, "application/json"):
		return parseJSON(c.Body())

	case strings.HasPrefix(ct, "application/x-www-form-urlencoded"):
		return parseForm(c.Body())

	case ct == "":
		if payload, err := parseJSON(c.Body()); err == nil {
			return payload, nil
		}
		if payload, err := parseForm(c.Body()); err == nil {
			return payload, nil
		}
		return nil, fmt.Errorf("could not parse body: no Content-Type header and body is neither JSON nor form-encoded")

	default:
		return nil, fmt.Errorf("unsupported Content-Type %q — expected application/json or application/x-www-form-urlencoded", ct)
	}
}

func parseJSON(body []byte) (map[string]any, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return payload, nil
}

func parseForm(body []byte) (map[string]any, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("invalid form encoding: %w", err)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("form body contained no key-value pairs")
	}
	payload := make(map[string]any, len(values))
	for k, v := range values {
		payload[k] = v[len(v)-1]
	}
	return payload, nil
}

// QueryParam reads a query parameter with RFC 3986 decoding, preserving '+' as a literal character.
func QueryParam(c *fiber.Ctx, key string) string {
	raw := string(c.Request().URI().QueryString())
	for _, pair := range strings.Split(raw, "&") {
		k, v, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		if decodedKey, _ := url.QueryUnescape(k); decodedKey == key {
			decoded, err := url.PathUnescape(v)
			if err != nil {
				return v
			}
			return decoded
		}
	}
	return ""
}
