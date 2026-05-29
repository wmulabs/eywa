package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

type apiKeyEntry struct {
	hash    []byte
	role    string
	subject string
}

type APIKeyValidator struct {
	entries []apiKeyEntry
}

func NewAPIKeyValidator(keys map[string]string) ports.TokenValidator {
	entries := make([]apiKeyEntry, 0, len(keys))
	for k, role := range keys {
		sum := sha256.Sum256([]byte(k))
		entries = append(entries, apiKeyEntry{
			hash:    sum[:],
			role:    role,
			subject: apiKeyID(k),
		})
	}
	return &APIKeyValidator{entries: entries}
}

func (v *APIKeyValidator) Validate(_ context.Context, token string) (*ports.AuthClaims, error) {
	incomingSum := sha256.Sum256([]byte(token))
	incomingHash := incomingSum[:]
	for _, entry := range v.entries {
		if subtle.ConstantTimeCompare(incomingHash, entry.hash) == 1 {
			return &ports.AuthClaims{Subject: entry.subject, Role: entry.role}, nil
		}
	}
	return nil, fmt.Errorf("invalid api key")
}

func apiKeyID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("apikey:%x", sum[:4])
}
