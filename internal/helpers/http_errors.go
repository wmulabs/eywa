package helpers

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/errors"
)

// FromHTTPStatus classifies a status code and body as BusinessError (4xx) or InfrastructureError
// (5xx, 429, 502/503/504). Returns nil for 2xx. Use this when you already hold the status code
// and body separately (e.g. after API-specific JSON parsing).
func FromHTTPStatus(statusCode int, body string) *errors.ActionError {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	msg := fmt.Sprintf("HTTP %d", statusCode)
	if body != "" {
		if len(body) > 200 {
			body = body[:200] + "..."
		}
		msg = fmt.Sprintf("HTTP %d: %s", statusCode, strings.TrimSpace(body))
	}

	switch {
	case statusCode == 429, statusCode == 502, statusCode == 503, statusCode == 504:
		return errors.NewInfrastructureError(msg)
	case statusCode >= 500:
		return errors.NewInfrastructureError(msg)
	default:
		return errors.NewBusinessError(msg)
	}
}

// maxErrorBodyBytes caps how many bytes are read from a non-2xx response body.
// FromHTTPStatus truncates the body to 200 chars anyway; 4 KiB is a generous ceiling.
const maxErrorBodyBytes = 4 * 1024 // 4 KiB

// FromHTTPResponse classifies a non-2xx *http.Response as BusinessError or InfrastructureError,
// reading the body only on error so the caller can still decode it on success.
// Returns nil for 2xx without consuming the body.
func FromHTTPResponse(resp *http.Response) *errors.ActionError {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	return FromHTTPStatus(resp.StatusCode, string(body))
}
