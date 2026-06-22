package dialog360

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	eywa "github.com/wmulabs/eywa"
)

// BasicAuthVerifier authenticates inbound 360dialog webhooks via HTTP Basic Auth — the mechanism
// 360dialog documents for securing the webhook URL (it does NOT forward Meta's X-Hub-Signature-256,
// since the Meta app secret belongs to 360dialog as the BSP). Configure the same user/password in the
// 360dialog webhook settings; they arrive as "Authorization: Basic base64(user:pass)".
type BasicAuthVerifier struct {
	expected string
}

func NewBasicAuthVerifier(username, password string) eywa.RequestVerifier {
	cred := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return &BasicAuthVerifier{expected: "Basic " + cred}
}

func (v *BasicAuthVerifier) Verify(_ context.Context, req eywa.VerifiableRequest) (*eywa.AuthClaims, error) {
	got := req.Header.Get("Authorization")
	if subtle.ConstantTimeCompare([]byte(got), []byte(v.expected)) != 1 {
		return nil, fmt.Errorf("360dialog basic auth mismatch")
	}
	return &eywa.AuthClaims{Subject: "360dialog", Role: "app"}, nil
}
