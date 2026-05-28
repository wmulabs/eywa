package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

type JWKSValidator struct {
	jwksURL   string
	audience  string
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	cacheTTL  time.Duration
}

// NewJWKSValidator validates RS256 JWTs using keys from a JWKS endpoint (Auth0, Firebase, Google IAP).
func NewJWKSValidator(jwksURL, audience string) ports.TokenValidator {
	return &JWKSValidator{
		jwksURL:  jwksURL,
		audience: audience,
		keys:     make(map[string]*rsa.PublicKey),
		cacheTTL: time.Hour,
	}
}

func (v *JWKSValidator) fetchKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("build jwks request: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	var jwks jwksResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1*1024*1024)).Decode(&jwks); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

func (v *JWKSValidator) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	stale := time.Since(v.fetchedAt) > v.cacheTTL || len(v.keys) == 0
	key := v.keys[kid]
	v.mu.RUnlock()

	if stale || key == nil {
		if err := v.fetchKeys(ctx); err != nil {
			return nil, err
		}
		v.mu.RLock()
		key = v.keys[kid]
		v.mu.RUnlock()
	}
	if key == nil {
		return nil, fmt.Errorf("key %q not found in JWKS", kid)
	}
	return key, nil
}

func (v *JWKSValidator) Validate(ctx context.Context, token string) (*ports.AuthClaims, error) {
	t, err := jwt.ParseWithClaims(token, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return v.publicKey(ctx, kid)
	})
	if err != nil || !t.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := t.Claims.(*jwtClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("missing subject: %w", err)
	}
	return &ports.AuthClaims{Subject: sub, Role: claims.Role}, nil
}

func rsaPublicKeyFromJWK(n, e string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
