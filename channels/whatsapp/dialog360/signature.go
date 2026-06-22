package dialog360

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"

	eywa "github.com/wmulabs/eywa"
)

const metaSignatureHeader = "X-Hub-Signature-256"

// SignatureVerifier authenticates inbound WhatsApp webhooks by validating X-Hub-Signature-256 — the
// Meta Cloud API scheme: HMAC-SHA256 of the raw request body keyed by the app secret, hex-encoded and
// prefixed with "sha256=". Use it with 360dialog/BSP setups that forward the Meta signature.
type SignatureVerifier struct {
	appSecret []byte
}

func NewSignatureVerifier(appSecret string) eywa.RequestVerifier {
	return &SignatureVerifier{appSecret: []byte(appSecret)}
}

func (v *SignatureVerifier) Verify(_ context.Context, req eywa.VerifiableRequest) (*eywa.AuthClaims, error) {
	provided := req.Header.Get(metaSignatureHeader)
	if provided == "" {
		return nil, fmt.Errorf("missing %s", metaSignatureHeader)
	}
	hexSig, ok := strings.CutPrefix(provided, "sha256=")
	if !ok {
		return nil, fmt.Errorf("%s must be prefixed with sha256=", metaSignatureHeader)
	}
	sig, err := hex.DecodeString(hexSig)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	mac := hmac.New(sha256.New, v.appSecret)
	mac.Write(req.Body)
	if subtle.ConstantTimeCompare(mac.Sum(nil), sig) != 1 {
		return nil, fmt.Errorf("whatsapp signature mismatch")
	}
	return &eywa.AuthClaims{Subject: "360dialog", Role: "app"}, nil
}
