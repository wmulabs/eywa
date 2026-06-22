package auth

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type fakeAppTokenRepo struct {
	byHash map[string]*entities.AppToken
}

func newFakeAppTokenRepo(tokens ...*entities.AppToken) *fakeAppTokenRepo {
	r := &fakeAppTokenRepo{byHash: map[string]*entities.AppToken{}}
	for _, t := range tokens {
		r.byHash[string(t.Hash)] = t
	}
	return r
}

func (r *fakeAppTokenRepo) Create(_ context.Context, t *entities.AppToken) error {
	r.byHash[string(t.Hash)] = t
	return nil
}
func (r *fakeAppTokenRepo) FindByHash(_ context.Context, hash []byte) (*entities.AppToken, error) {
	if t, ok := r.byHash[string(hash)]; ok {
		return t, nil
	}
	return nil, errors.New("not found")
}
func (r *fakeAppTokenRepo) List(_ context.Context) ([]*entities.AppToken, error) { return nil, nil }
func (r *fakeAppTokenRepo) Revoke(_ context.Context, _ string) error             { return nil }

func TestMintAppToken(t *testing.T) {
	secret, token, err := MintAppToken("ci", time.Hour)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if secret == "" || token.ID == "" {
		t.Fatal("expected non-empty secret and id")
	}
	if !bytes.Equal(token.Hash, HashAppToken(secret)) {
		t.Error("stored hash must match the secret's hash")
	}
	if token.ExpiresAt == nil {
		t.Error("expected ExpiresAt set for a positive ttl")
	}
	if token.Role != appTokenRole {
		t.Errorf("role = %q, want %q", token.Role, appTokenRole)
	}

	_, infinite, _ := MintAppToken("infinite", 0)
	if infinite.ExpiresAt != nil {
		t.Error("expected nil ExpiresAt for a zero ttl (never expires)")
	}
}

func TestAppTokenValidator_Validate(t *testing.T) {
	secret, token, _ := MintAppToken("ci", time.Hour)
	v := NewAppTokenValidator(newFakeAppTokenRepo(token))

	claims, err := v.Validate(context.Background(), secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Role != appTokenRole || claims.Subject != "apptoken:"+token.ID {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestAppTokenValidator_Rejects(t *testing.T) {
	expiredSecret, expired, _ := MintAppToken("expired", time.Hour)
	past := time.Now().Add(-2 * time.Hour).UTC()
	expired.ExpiresAt = &past

	revokedSecret, revoked, _ := MintAppToken("revoked", time.Hour)
	now := time.Now().UTC()
	revoked.RevokedAt = &now

	repo := newFakeAppTokenRepo(expired, revoked)
	v := NewAppTokenValidator(repo)

	cases := []struct {
		name  string
		token string
	}{
		{"unknown token", "does-not-exist"},
		{"expired token", expiredSecret},
		{"revoked token", revokedSecret},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v.Validate(context.Background(), tc.token); err == nil {
				t.Error("expected validation to fail")
			}
		})
	}
}
