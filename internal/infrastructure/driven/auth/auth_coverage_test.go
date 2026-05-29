package auth

import (
	"context"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestJWKSValidator_FetchKeys_HTTPError covers the http.Get failure branch (lines 50-52).
func TestJWKSValidator_FetchKeys_HTTPError(t *testing.T) {
	v := &JWKSValidator{
		jwksURL: "http://127.0.0.1:1", // unreachable
		keys:    make(map[string]*rsa.PublicKey),
	}
	if err := v.fetchKeys(context.Background()); err == nil {
		t.Fatal("expected error from unreachable JWKS server")
	}
}

// TestJWKSValidator_FetchKeys_InvalidJSON covers the json.Decode failure branch (lines 56-58).
func TestJWKSValidator_FetchKeys_InvalidJSON_Direct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json!!!"))
	}))
	defer srv.Close()

	v := &JWKSValidator{
		jwksURL: srv.URL,
		keys:    make(map[string]*rsa.PublicKey),
	}
	if err := v.fetchKeys(context.Background()); err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

// TestJWKSValidator_PublicKey_FetchError covers the fetchKeys error branch in publicKey (line 86-88).
func TestJWKSValidator_PublicKey_FetchError(t *testing.T) {
	v := &JWKSValidator{
		jwksURL: "http://127.0.0.1:1", // unreachable
		keys:    make(map[string]*rsa.PublicKey),
	}
	// keys is empty + no cacheTTL set → stale → will call fetchKeys → error
	_, err := v.publicKey(context.Background(), "any-kid")
	if err == nil {
		t.Fatal("expected error when fetch fails")
	}
}
