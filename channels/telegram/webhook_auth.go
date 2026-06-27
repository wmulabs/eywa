package telegram

import (
	"context"
	"crypto/subtle"
	"fmt"

	eywa "github.com/wmulabs/eywa"
)

// SecretTokenVerifier authenticates inbound Telegram webhooks via the secret token Telegram echoes in
// the X-Telegram-Bot-Api-Secret-Token header (configured when registering the webhook with setWebhook).
type SecretTokenVerifier struct {
	expected string
}

func NewSecretTokenVerifier(secret string) eywa.RequestVerifier {
	return &SecretTokenVerifier{expected: secret}
}

func (v *SecretTokenVerifier) Verify(_ context.Context, req eywa.VerifiableRequest) (*eywa.AuthClaims, error) {
	got := req.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(v.expected)) != 1 {
		return nil, fmt.Errorf("telegram: secret token mismatch")
	}
	return &eywa.AuthClaims{Subject: "telegram", Role: "app"}, nil
}
