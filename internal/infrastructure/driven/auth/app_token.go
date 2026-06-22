package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// appTokenRole is the role granted to a request authenticated by an app token. App tokens authenticate
// machine callers (event ingestion); they are not "admin" and so cannot reach admin-gated routes.
const appTokenRole = "app"

// HashAppToken returns the SHA-256 hash stored for a token secret. The plaintext is never persisted.
func HashAppToken(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// GenerateAppTokenSecret returns a high-entropy URL-safe secret for a new app token.
func GenerateAppTokenSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate app token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// MintAppToken creates a new app token record and its one-time plaintext secret. A ttl of zero (or
// negative) produces a token that never expires. The returned secret is the only time the plaintext is
// available — store only the returned *AppToken.
func MintAppToken(name string, ttl time.Duration) (secret string, token *entities.AppToken, err error) {
	secret, err = GenerateAppTokenSecret()
	if err != nil {
		return "", nil, err
	}

	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return "", nil, fmt.Errorf("generate app token id: %w", err)
	}

	now := time.Now().UTC()
	token = &entities.AppToken{
		ID:        hex.EncodeToString(id),
		Name:      name,
		Hash:      HashAppToken(secret),
		Role:      appTokenRole,
		CreatedAt: now,
	}
	if ttl > 0 {
		expiresAt := now.Add(ttl)
		token.ExpiresAt = &expiresAt
	}
	return secret, token, nil
}

// AppTokenValidator authenticates a bearer token against the AppTokenRepository, honoring expiry and
// revocation at request time (no redeploy needed to revoke).
type AppTokenValidator struct {
	repo ports.AppTokenRepository
}

func NewAppTokenValidator(repo ports.AppTokenRepository) ports.TokenValidator {
	return &AppTokenValidator{repo: repo}
}

func (v *AppTokenValidator) Validate(ctx context.Context, token string) (*ports.AuthClaims, error) {
	rec, err := v.repo.FindByHash(ctx, HashAppToken(token))
	if err != nil {
		return nil, fmt.Errorf("invalid app token")
	}
	if !rec.IsActive(time.Now().UTC()) {
		return nil, fmt.Errorf("app token expired or revoked")
	}
	role := rec.Role
	if role == "" {
		role = appTokenRole
	}
	return &ports.AuthClaims{Subject: "apptoken:" + rec.ID, Role: role}, nil
}
