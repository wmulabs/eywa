// Package slack is an inbound/outbound channel adapter for Slack's Events API. It implements
// eywa.Receptor (event_callback -> Pulse), eywa.Voice (chat.postMessage), a RequestVerifier for Slack's
// signed requests, and a helper for the one-time url_verification handshake.
package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	eywa "github.com/wmulabs/eywa"
)

const maxClockSkew = 5 * time.Minute

// SignatureVerifier authenticates inbound Slack requests with Slack's signing scheme: an HMAC-SHA256
// over "v0:<timestamp>:<rawBody>" keyed by the signing secret, compared constant-time to the
// X-Slack-Signature header. Requests older than maxClockSkew are rejected to block replay.
type SignatureVerifier struct {
	signingSecret []byte
	now           func() time.Time // injectable for tests
}

func NewSignatureVerifier(signingSecret string) eywa.RequestVerifier {
	return &SignatureVerifier{signingSecret: []byte(signingSecret), now: time.Now}
}

func (v *SignatureVerifier) Verify(_ context.Context, req eywa.VerifiableRequest) (*eywa.AuthClaims, error) {
	ts := req.Header.Get("X-Slack-Request-Timestamp")
	sig := req.Header.Get("X-Slack-Signature")
	if ts == "" || sig == "" {
		return nil, fmt.Errorf("slack: missing signature headers")
	}

	tsUnix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("slack: invalid timestamp: %w", err)
	}
	if delta := v.now().Sub(time.Unix(tsUnix, 0)); delta > maxClockSkew || delta < -maxClockSkew {
		return nil, fmt.Errorf("slack: stale request timestamp")
	}

	mac := hmac.New(sha256.New, v.signingSecret)
	mac.Write([]byte("v0:" + ts + ":"))
	mac.Write(req.Body)
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) != 1 {
		return nil, fmt.Errorf("slack: signature mismatch")
	}
	return &eywa.AuthClaims{Subject: "slack", Role: "app"}, nil
}
