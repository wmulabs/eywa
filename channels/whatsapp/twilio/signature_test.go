package twilio

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // mirrors Twilio's HMAC-SHA1 signing scheme for the test fixture.
	"encoding/base64"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

func twilioSign(token, fullURL string, params url.Values) string {
	var sb strings.Builder
	sb.WriteString(fullURL)
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range params[k] {
			sb.WriteString(k)
			sb.WriteString(v)
		}
	}
	mac := hmac.New(sha1.New, []byte(token))
	mac.Write([]byte(sb.String()))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func twilioReq(sig string, body string) eywa.VerifiableRequest {
	h := http.Header{}
	if sig != "" {
		h.Set("X-Twilio-Signature", sig)
	}
	return eywa.VerifiableRequest{
		Method: "POST",
		URL:    "https://app.example.com/api/v1/events/whatsapp",
		Header: h,
		Body:   []byte(body),
	}
}

func TestTwilioSignatureVerifier_Valid(t *testing.T) {
	token := "auth-token"
	params := url.Values{"From": {"+100"}, "Body": {"hi there"}}
	sig := twilioSign(token, "https://app.example.com/api/v1/events/whatsapp", params)

	v := NewSignatureVerifier(token)
	claims, err := v.Verify(context.Background(), twilioReq(sig, params.Encode()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "twilio" || claims.Role != "app" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestTwilioSignatureVerifier_Rejects(t *testing.T) {
	token := "auth-token"
	params := url.Values{"From": {"+100"}, "Body": {"hi"}}
	good := twilioSign(token, "https://app.example.com/api/v1/events/whatsapp", params)

	cases := []struct {
		name string
		req  eywa.VerifiableRequest
	}{
		{"missing header", twilioReq("", params.Encode())},
		{"wrong token", twilioReq(twilioSign("other", "https://app.example.com/api/v1/events/whatsapp", params), params.Encode())},
		{"tampered body", twilioReq(good, url.Values{"From": {"+999"}, "Body": {"hi"}}.Encode())},
	}
	v := NewSignatureVerifier(token)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v.Verify(context.Background(), tc.req); err == nil {
				t.Error("expected verification to fail")
			}
		})
	}
}
