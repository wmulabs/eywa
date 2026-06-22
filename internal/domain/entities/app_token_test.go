package entities

import (
	"testing"
	"time"
)

func TestAppToken_IsActive(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name  string
		token AppToken
		want  bool
	}{
		{"no expiry, not revoked", AppToken{}, true},
		{"future expiry", AppToken{ExpiresAt: &future}, true},
		{"past expiry", AppToken{ExpiresAt: &past}, false},
		{"expires exactly now", AppToken{ExpiresAt: &now}, false},
		{"revoked", AppToken{RevokedAt: &past}, false},
		{"revoked overrides future expiry", AppToken{ExpiresAt: &future, RevokedAt: &past}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.token.IsActive(now); got != tc.want {
				t.Errorf("IsActive() = %v, want %v", got, tc.want)
			}
		})
	}
}
