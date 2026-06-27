package dialog360

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

func basicReq(authHeader string) eywa.VerifiableRequest {
	h := http.Header{}
	if authHeader != "" {
		h.Set("Authorization", authHeader)
	}
	return eywa.VerifiableRequest{Method: "POST", URL: "https://x/api/v1/events/whatsapp", Header: h, Body: []byte(`{}`)}
}

func basicHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func TestBasicAuthVerifier_Valid(t *testing.T) {
	v := NewBasicAuthVerifier("hook", "s3cr3t")
	claims, err := v.Verify(context.Background(), basicReq(basicHeader("hook", "s3cr3t")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "360dialog" || claims.Role != "app" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestBasicAuthVerifier_Rejects(t *testing.T) {
	v := NewBasicAuthVerifier("hook", "s3cr3t")
	cases := []struct {
		name   string
		header string
	}{
		{"missing", ""},
		{"wrong password", basicHeader("hook", "wrong")},
		{"wrong user", basicHeader("other", "s3cr3t")},
		{"bearer instead of basic", "Bearer s3cr3t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v.Verify(context.Background(), basicReq(tc.header)); err == nil {
				t.Error("expected verification to fail")
			}
		})
	}
}
