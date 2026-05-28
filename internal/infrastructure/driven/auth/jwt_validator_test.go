package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func signHS256(t *testing.T, secret []byte, subject, role string, ttl time.Duration) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	signed, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("sign hs256: %v", err)
	}
	return signed
}

func signRS256(t *testing.T, key *rsa.PrivateKey, subject, role string, ttl time.Duration) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign rs256: %v", err)
	}
	return signed
}

func TestJWTValidator_HS256_ValidToken(t *testing.T) {
	secret := []byte("test-secret")
	v := NewJWTValidator(secret)
	token := signHS256(t, secret, "user-1", "admin", time.Hour)

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("want subject user-1, got %s", claims.Subject)
	}
	if claims.Role != "admin" {
		t.Errorf("want role admin, got %s", claims.Role)
	}
}

func TestJWTValidator_HS256_WrongSecret(t *testing.T) {
	v := NewJWTValidator([]byte("secret-A"))
	token := signHS256(t, []byte("secret-B"), "user-1", "admin", time.Hour)

	if _, err := v.Validate(context.Background(), token); err == nil {
		t.Error("expected error with wrong secret")
	}
}

func TestJWTValidator_HS256_ExpiredToken(t *testing.T) {
	secret := []byte("secret")
	v := NewJWTValidator(secret)
	token := signHS256(t, secret, "user-1", "admin", -time.Hour)

	if _, err := v.Validate(context.Background(), token); err == nil {
		t.Error("expected error for expired token")
	}
}

func TestJWTValidatorRS256_ValidToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	v := NewJWTValidatorRS256(&key.PublicKey)
	token := signRS256(t, key, "user-2", "operator", time.Hour)

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "user-2" {
		t.Errorf("want subject user-2, got %s", claims.Subject)
	}
	if claims.Role != "operator" {
		t.Errorf("want role operator, got %s", claims.Role)
	}
}

func TestJWTValidatorRS256_WrongKey(t *testing.T) {
	keyA, _ := rsa.GenerateKey(rand.Reader, 2048)
	keyB, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := NewJWTValidatorRS256(&keyB.PublicKey)
	token := signRS256(t, keyA, "user-2", "operator", time.Hour)

	if _, err := v.Validate(context.Background(), token); err == nil {
		t.Error("expected error with wrong public key")
	}
}

func TestJWTValidator_HS256_WrongAlgorithm_ReturnsError(t *testing.T) {
	// Token signed with RS256, but validator expects HS256 → unexpected signing method
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &jwtClaims{
		Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, _ := tok.SignedString(key)
	v := NewJWTValidator([]byte("secret"))
	_, err := v.Validate(context.Background(), signed)
	if err == nil {
		t.Error("expected error for RS256 token given to HS256 validator")
	}
}
