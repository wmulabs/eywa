package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

func publicKeyToJWK(kid string, pub *rsa.PublicKey) jwksKey {
	nBytes := pub.N.Bytes()
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	return jwksKey{
		Kid: kid,
		Kty: "RSA",
		N:   base64.RawURLEncoding.EncodeToString(nBytes),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}
}

func signRS256WithKid(t *testing.T, key *rsa.PrivateKey, kid, subject, role string, ttl time.Duration) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign rs256: %v", err)
	}
	return signed
}

func newJWKSServer(t *testing.T, keys ...jwksKey) *httptest.Server {
	t.Helper()
	resp := jwksResponse{Keys: keys}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// --- rsaPublicKeyFromJWK ---

func TestRsaPublicKeyFromJWK_Valid(t *testing.T) {
	key := generateRSAKey(t)
	jwk := publicKeyToJWK("k1", &key.PublicKey)
	pub, err := rsaPublicKeyFromJWK(jwk.N, jwk.E)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub.N.Cmp(key.N) != 0 {
		t.Error("recovered key N does not match")
	}
	if pub.E != key.E {
		t.Errorf("recovered key E %d != %d", pub.E, key.E)
	}
}

func TestRsaPublicKeyFromJWK_InvalidN(t *testing.T) {
	_, err := rsaPublicKeyFromJWK("not!base64url!", "AQAB")
	if err == nil {
		t.Fatal("expected error for invalid N")
	}
}

func TestRsaPublicKeyFromJWK_InvalidE(t *testing.T) {
	key := generateRSAKey(t)
	jwk := publicKeyToJWK("k1", &key.PublicKey)
	_, err := rsaPublicKeyFromJWK(jwk.N, "not!base64url!")
	if err == nil {
		t.Fatal("expected error for invalid E")
	}
}

// --- JWKSValidator ---

func TestJWKSValidator_Validate_Success(t *testing.T) {
	key := generateRSAKey(t)
	jwk := publicKeyToJWK("kid-1", &key.PublicKey)
	srv := newJWKSServer(t, jwk)
	defer srv.Close()

	token := signRS256WithKid(t, key, "kid-1", "user-42", "admin", time.Hour)
	v := NewJWKSValidator(srv.URL, "")

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "user-42" {
		t.Errorf("expected user-42, got %s", claims.Subject)
	}
	if claims.Role != "admin" {
		t.Errorf("expected admin, got %s", claims.Role)
	}
}

func TestJWKSValidator_Validate_KeyNotFound(t *testing.T) {
	key := generateRSAKey(t)
	jwk := publicKeyToJWK("kid-1", &key.PublicKey)
	srv := newJWKSServer(t, jwk)
	defer srv.Close()

	// sign with kid-2 which is not in the JWKS
	token := signRS256WithKid(t, key, "kid-2", "user-42", "admin", time.Hour)
	v := NewJWKSValidator(srv.URL, "")

	_, err := v.Validate(context.Background(), token)
	if err == nil {
		t.Fatal("expected error when kid not found")
	}
}

func TestJWKSValidator_Validate_ServerDown(t *testing.T) {
	v := NewJWKSValidator("http://127.0.0.1:1", "")
	_, err := v.Validate(context.Background(), "a.b.c")
	if err == nil {
		t.Fatal("expected error when JWKS server is unreachable")
	}
}

func TestJWKSValidator_Validate_WrongSigningMethod(t *testing.T) {
	srv := newJWKSServer(t) // no keys needed
	defer srv.Close()

	// HS256 token — JWKSValidator expects RS256
	token := signHS256(t, []byte("secret"), "user-1", "admin", time.Hour)
	v := NewJWKSValidator(srv.URL, "")

	_, err := v.Validate(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for wrong signing method")
	}
}

func TestJWKSValidator_Validate_NonRSAKeyInJWKS_Ignored(t *testing.T) {
	key := generateRSAKey(t)
	// Include a non-RSA key alongside the valid one
	nonRSAKey := jwksKey{Kid: "ec-1", Kty: "EC", N: "", E: ""}
	validKey := publicKeyToJWK("rsa-1", &key.PublicKey)
	srv := newJWKSServer(t, nonRSAKey, validKey)
	defer srv.Close()

	token := signRS256WithKid(t, key, "rsa-1", "user-x", "user", time.Hour)
	v := NewJWKSValidator(srv.URL, "")

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "user-x" {
		t.Errorf("expected user-x, got %s", claims.Subject)
	}
}

func TestJWKSValidator_Validate_CachesKeys(t *testing.T) {
	key := generateRSAKey(t)
	jwk := publicKeyToJWK("kid-1", &key.PublicKey)
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwksResponse{Keys: []jwksKey{jwk}})
	}))
	defer srv.Close()

	v := NewJWKSValidator(srv.URL, "")

	token := signRS256WithKid(t, key, "kid-1", "user-1", "admin", time.Hour)
	for i := 0; i < 3; i++ {
		if _, err := v.Validate(context.Background(), token); err != nil {
			t.Fatalf("validate %d: %v", i, err)
		}
	}
	if callCount > 1 {
		t.Errorf("expected keys to be cached, but JWKS was fetched %d times", callCount)
	}
}

func TestJWKSValidator_FetchKeys_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	v := NewJWKSValidator(srv.URL, "")
	_, err := v.Validate(context.Background(), "any.token.here")
	if err == nil {
		t.Fatal("expected error for invalid JSON JWKS response")
	}
}
