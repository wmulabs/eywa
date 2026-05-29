package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"
)

// --- JWT validator wrong-method branches ---

func TestJWTValidator_HS256_WrongSigningMethod(t *testing.T) {
	// Token signed with RS256 passed to HS256 validator — triggers unexpected signing method.
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	token := signRS256(t, key, "user-1", "admin", time.Hour)

	v := NewJWTValidator([]byte("secret"))
	_, err := v.Validate(context.Background(), token)
	if err == nil {
		t.Fatal("expected error when RS256 token given to HS256 validator")
	}
}

func TestJWTValidatorRS256_WrongSigningMethod(t *testing.T) {
	// HS256 token passed to RS256 validator — triggers unexpected signing method.
	token := signHS256(t, []byte("secret"), "user-1", "admin", time.Hour)

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := NewJWTValidatorRS256(&key.PublicKey)
	_, err := v.Validate(context.Background(), token)
	if err == nil {
		t.Fatal("expected error when HS256 token given to RS256 validator")
	}
}

// --- OperatorAuth wrong-method branch ---

func TestOperatorAuth_Validate_WrongSigningMethod(t *testing.T) {
	// OperatorAuth uses HS256; passing an RS256 token should fail.
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	token := signRS256(t, key, "user-1", "admin", time.Hour)

	repo := testOperatorRepo(t, "pass")
	a := NewOperatorAuth(repo, []byte("secret"))
	_, err := a.Validate(context.Background(), token)
	if err == nil {
		t.Fatal("expected error when RS256 token given to HS256-based OperatorAuth")
	}
}

// --- JWKS fetchKeys with RSA key having invalid base64 (skipped) ---

func TestJWKSValidator_FetchKeys_InvalidRSAKey_Skipped(t *testing.T) {
	// A JWKS with a valid key and an RSA key with bad N (parse error, skipped).
	realKey := generateRSAKey(t)
	validJWK := publicKeyToJWK("good", &realKey.PublicKey)
	badJWK := jwksKey{Kid: "bad", Kty: "RSA", N: "not!valid!base64", E: "AQAB"}

	srv := newJWKSServer(t, badJWK, validJWK)
	defer srv.Close()

	token := signRS256WithKid(t, realKey, "good", "user-1", "admin", time.Hour)
	v := NewJWKSValidator(srv.URL, "")

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error (bad key should be skipped): %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("expected user-1, got %s", claims.Subject)
	}
}
