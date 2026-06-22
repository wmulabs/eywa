package twilio

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // Twilio's request-signature scheme mandates HMAC-SHA1; not used to hash secrets.
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"

	eywa "github.com/wmulabs/eywa"
)

const twilioSignatureHeader = "X-Twilio-Signature"

// SignatureVerifier authenticates inbound Twilio webhooks by validating X-Twilio-Signature. Twilio
// signs the full request URL with the form parameters (sorted by key, concatenated key+value)
// appended, using HMAC-SHA1 keyed by the account Auth Token, base64-encoded.
//
// The URL must match exactly what Twilio signed (scheme, host, path, query). Behind a proxy/load
// balancer, ensure the app reconstructs the original external URL (X-Forwarded-Proto/Host) or the
// signature will not match.
type SignatureVerifier struct {
	authToken string
}

func NewSignatureVerifier(authToken string) eywa.RequestVerifier {
	return &SignatureVerifier{authToken: authToken}
}

func (v *SignatureVerifier) Verify(_ context.Context, req eywa.VerifiableRequest) (*eywa.AuthClaims, error) {
	provided := req.Header.Get(twilioSignatureHeader)
	if provided == "" {
		return nil, fmt.Errorf("missing %s", twilioSignatureHeader)
	}

	params, err := url.ParseQuery(string(req.Body))
	if err != nil {
		return nil, fmt.Errorf("parse twilio form body: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(req.URL)
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, val := range params[k] {
			sb.WriteString(k)
			sb.WriteString(val)
		}
	}

	mac := hmac.New(sha1.New, []byte(v.authToken))
	mac.Write([]byte(sb.String()))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		return nil, fmt.Errorf("twilio signature mismatch")
	}
	return &eywa.AuthClaims{Subject: "twilio", Role: "app"}, nil
}
