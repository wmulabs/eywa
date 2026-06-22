package entities

import "time"

// AppToken is a long-lived, revocable credential a calling app presents to authenticate inbound event
// requests. Only the SHA-256 hash of the secret is stored — the plaintext is shown once at creation.
// ExpiresAt nil means the token never expires; RevokedAt nil means it is still active.
type AppToken struct {
	ID        string     `bson:"_id" json:"id"`
	Name      string     `bson:"name" json:"name"`
	Hash      []byte     `bson:"hash" json:"-"`
	Role      string     `bson:"role,omitempty" json:"role,omitempty"`
	CreatedAt time.Time  `bson:"created_at" json:"created_at"`
	ExpiresAt *time.Time `bson:"expires_at,omitempty" json:"expires_at,omitempty"`
	RevokedAt *time.Time `bson:"revoked_at,omitempty" json:"revoked_at,omitempty"`
}

// IsActive reports whether the token may authenticate a request at now: not revoked and not past its
// expiry (a nil ExpiresAt never expires).
func (t *AppToken) IsActive(now time.Time) bool {
	if t.RevokedAt != nil {
		return false
	}
	if t.ExpiresAt != nil && !now.Before(*t.ExpiresAt) {
		return false
	}
	return true
}
