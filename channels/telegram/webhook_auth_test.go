package telegram

import (
	"context"
	"net/http"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

func reqWithToken(token string) eywa.VerifiableRequest {
	h := http.Header{}
	if token != "" {
		h.Set("X-Telegram-Bot-Api-Secret-Token", token)
	}
	return eywa.VerifiableRequest{Header: h}
}

func TestSecretTokenVerifier(t *testing.T) {
	v := NewSecretTokenVerifier("s3cr3t")

	claims, err := v.Verify(context.Background(), reqWithToken("s3cr3t"))
	if err != nil || claims == nil || claims.Subject != "telegram" {
		t.Errorf("valid token must pass: claims=%v err=%v", claims, err)
	}

	if _, err := v.Verify(context.Background(), reqWithToken("wrong")); err == nil {
		t.Error("wrong token must be rejected")
	}
	if _, err := v.Verify(context.Background(), reqWithToken("")); err == nil {
		t.Error("missing token must be rejected")
	}
}
