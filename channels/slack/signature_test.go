package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"

	eywa "github.com/wmulabs/eywa"
	"testing"
	"time"
)

func signed(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + ts + ":"))
	mac.Write(body)
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func verifierAt(secret string, now time.Time) *SignatureVerifier {
	return &SignatureVerifier{signingSecret: []byte(secret), now: func() time.Time { return now }}
}

func req(ts, sig string, body []byte) eywa.VerifiableRequest {
	h := http.Header{}
	if ts != "" {
		h.Set("X-Slack-Request-Timestamp", ts)
	}
	if sig != "" {
		h.Set("X-Slack-Signature", sig)
	}
	return eywa.VerifiableRequest{Header: h, Body: body}
}

func TestNewSignatureVerifier(t *testing.T) {
	v := NewSignatureVerifier("secret")
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"type":"event_callback"}`)
	if _, err := v.Verify(context.Background(), req(ts, signed("secret", ts, body), body)); err != nil {
		t.Fatalf("constructor-built verifier must validate a fresh request: %v", err)
	}
}

func TestSignatureVerifier_Valid(t *testing.T) {
	now := time.Unix(1700000000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"type":"event_callback"}`)
	v := verifierAt("secret", now)

	claims, err := v.Verify(context.Background(), req(ts, signed("secret", ts, body), body))
	if err != nil || claims == nil || claims.Subject != "slack" {
		t.Fatalf("valid signature must pass: claims=%v err=%v", claims, err)
	}
}

func TestSignatureVerifier_Rejections(t *testing.T) {
	now := time.Unix(1700000000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"x":1}`)
	good := signed("secret", ts, body)

	cases := []struct {
		name string
		req  eywa.VerifiableRequest
		v    *SignatureVerifier
	}{
		{"missing headers", req("", "", body), verifierAt("secret", now)},
		{"bad timestamp", req("notanumber", good, body), verifierAt("secret", now)},
		{"stale", req(ts, good, body), verifierAt("secret", now.Add(10*time.Minute))},
		{"wrong secret", req(ts, signed("other", ts, body), body), verifierAt("secret", now)},
		{"tampered body", req(ts, good, []byte(`{"x":2}`)), verifierAt("secret", now)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.v.Verify(context.Background(), c.req); err == nil {
				t.Errorf("expected rejection for %s", c.name)
			}
		})
	}
}
