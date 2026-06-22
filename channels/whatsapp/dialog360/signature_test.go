package dialog360

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

func metaSign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func metaReq(sig string, body []byte) eywa.VerifiableRequest {
	h := http.Header{}
	if sig != "" {
		h.Set("X-Hub-Signature-256", sig)
	}
	return eywa.VerifiableRequest{Method: "POST", URL: "https://x/api/v1/events/whatsapp", Header: h, Body: body}
}

func TestDialog360SignatureVerifier_Valid(t *testing.T) {
	secret, body := "app-secret", []byte(`{"entry":[]}`)
	v := NewSignatureVerifier(secret)

	claims, err := v.Verify(context.Background(), metaReq(metaSign(secret, body), body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "360dialog" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestDialog360SignatureVerifier_Rejects(t *testing.T) {
	secret, body := "app-secret", []byte(`{"entry":[]}`)
	v := NewSignatureVerifier(secret)

	cases := []struct {
		name string
		req  eywa.VerifiableRequest
	}{
		{"missing header", metaReq("", body)},
		{"no prefix", metaReq(hex.EncodeToString([]byte("x")), body)},
		{"bad hex", metaReq("sha256=zz", body)},
		{"wrong secret", metaReq(metaSign("other", body), body)},
		{"tampered body", metaReq(metaSign(secret, body), []byte("tampered"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v.Verify(context.Background(), tc.req); err == nil {
				t.Error("expected verification to fail")
			}
		})
	}
}
