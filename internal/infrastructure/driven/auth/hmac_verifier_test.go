package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

func signHMAC(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "." + string(body)))
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func hmacRequest(sig string, body []byte) ports.VerifiableRequest {
	h := http.Header{}
	if sig != "" {
		h.Set("X-Eywa-Signature", sig)
	}
	return ports.VerifiableRequest{Method: "POST", URL: "https://x/api/v1/events/chat", Header: h, Body: body}
}

func TestHMACVerifier_Valid(t *testing.T) {
	secret, body := "s3cr3t", []byte(`{"user":"u"}`)
	v := NewHMACVerifier(secret)

	claims, err := v.Verify(context.Background(), hmacRequest(signHMAC(secret, time.Now().Unix(), body), body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Role != appTokenRole {
		t.Errorf("role = %q, want %q", claims.Role, appTokenRole)
	}
}

func TestHMACVerifier_Rejects(t *testing.T) {
	secret, body := "s3cr3t", []byte(`{"user":"u"}`)
	now := time.Now().Unix()
	v := NewHMACVerifier(secret)

	cases := []struct {
		name string
		req  ports.VerifiableRequest
	}{
		{"missing header", hmacRequest("", body)},
		{"malformed header", hmacRequest("garbage", body)},
		{"missing v1", hmacRequest("t=123", body)},
		{"bad hex", hmacRequest("t=123,v1=zz", body)},
		{"bad timestamp", hmacRequest("t=abc,v1=aa", body)},
		{"wrong secret", hmacRequest(signHMAC("other", now, body), body)},
		{"tampered body", hmacRequest(signHMAC(secret, now, body), []byte("tampered"))},
		{"stale timestamp", hmacRequest(signHMAC(secret, now-3600, body), body)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v.Verify(context.Background(), tc.req); err == nil {
				t.Error("expected verification to fail")
			}
		})
	}
}

func TestHMACVerifier_Options(t *testing.T) {
	secret, body := "s3cr3t", []byte("x")
	v := NewHMACVerifier(secret, WithHMACHeader("X-Sig"), WithHMACTolerance(time.Hour))

	// Custom header + a timestamp 30m old still inside the 1h tolerance.
	ts := time.Now().Add(-30 * time.Minute).Unix()
	h := http.Header{}
	h.Set("X-Sig", signHMAC(secret, ts, body))
	req := ports.VerifiableRequest{Header: h, Body: body}

	if _, err := v.Verify(context.Background(), req); err != nil {
		t.Errorf("unexpected error with custom header/tolerance: %v", err)
	}
}
