package helpers

import (
	"io"
	"net/http"
	"strings"
	"testing"

	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
)

func TestFromHTTPStatus(t *testing.T) {
	cases := []struct {
		status      int
		body        string
		wantNil     bool
		wantInfra   bool
		bodyInMsg   bool
	}{
		// 2xx → nil
		{200, "", true, false, false},
		{201, "created", true, false, false},

		// 4xx → BusinessError
		{400, "bad request", false, false, true},
		{404, "not found", false, false, true},
		{422, "validation error", false, false, true},

		// rate limit / gateway errors → InfrastructureError
		{429, "too many requests", false, true, true},
		{502, "bad gateway", false, true, true},
		{503, "service unavailable", false, true, true},
		{504, "gateway timeout", false, true, true},

		// 5xx → InfrastructureError
		{500, "internal server error", false, true, true},
		{501, "not implemented", false, true, true},

		// empty body
		{400, "", false, false, false},

		// body truncated at 200 chars
		{400, strings.Repeat("x", 300), false, false, false},
	}

	for _, c := range cases {
		err := FromHTTPStatus(c.status, c.body)
		if c.wantNil {
			if err != nil {
				t.Errorf("status %d: expected nil, got %v", c.status, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("status %d: expected error, got nil", c.status)
			continue
		}
		if c.wantInfra && !domainerrors.IsInfrastructureError(err) {
			t.Errorf("status %d: expected InfrastructureError, got %v", c.status, err)
		}
		if !c.wantInfra && domainerrors.IsInfrastructureError(err) {
			t.Errorf("status %d: expected BusinessError, got %v", c.status, err)
		}
	}
}

func TestFromHTTPStatus_LongBodyTruncated(t *testing.T) {
	body := strings.Repeat("a", 300)
	err := FromHTTPStatus(400, body)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	// message should contain "..." (truncated) and not the full 300-char body
	if !strings.Contains(msg, "...") {
		t.Errorf("expected truncated message with '...', got: %s", msg)
	}
}

func TestFromHTTPResponse(t *testing.T) {
	// 2xx → nil
	resp200 := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("ok")),
	}
	if err := FromHTTPResponse(resp200); err != nil {
		t.Errorf("2xx should return nil, got %v", err)
	}

	// 4xx → BusinessError
	resp400 := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(strings.NewReader("bad input")),
	}
	err := FromHTTPResponse(resp400)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if domainerrors.IsInfrastructureError(err) {
		t.Error("400 should be BusinessError")
	}

	// 500 → InfrastructureError
	resp500 := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("server error")),
	}
	err = FromHTTPResponse(resp500)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !domainerrors.IsInfrastructureError(err) {
		t.Error("500 should be InfrastructureError")
	}
}
