package ports

import (
	"context"
	"net/http"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type TokenValidator interface {
	Validate(ctx context.Context, token string) (*AuthClaims, error)
}

type AuthClaims struct {
	Subject string
	Role    string
}

// VerifiableRequest is the transport-agnostic view of an inbound request that a RequestVerifier
// inspects. URL is the full reconstructed request URL (scheme://host/path?query); Body is the raw,
// unparsed request body — both needed for signature schemes (HMAC over the body, provider webhooks).
type VerifiableRequest struct {
	Method string
	URL    string
	Header http.Header
	Body   []byte
}

// RequestVerifier authenticates an inbound event request from its full content (headers + raw body),
// enabling signature-based schemes (HMAC, provider webhook signatures) that a bearer-only
// TokenValidator cannot express. Return non-nil claims on success, an error to reject.
type RequestVerifier interface {
	Verify(ctx context.Context, req VerifiableRequest) (*AuthClaims, error)
}

// AppTokenRepository persists revocable app tokens used to authenticate inbound event requests.
// FindByHash returns the matching token (active or not) or a not-found error; callers check IsActive.
type AppTokenRepository interface {
	Create(ctx context.Context, token *entities.AppToken) error
	FindByHash(ctx context.Context, hash []byte) (*entities.AppToken, error)
	List(ctx context.Context) ([]*entities.AppToken, error)
	Revoke(ctx context.Context, id string) error
}

type OperatorRepository interface {
	Create(ctx context.Context, op *entities.Operator) error
	FindByEmail(ctx context.Context, email string) (*entities.Operator, error)
	FindByID(ctx context.Context, id string) (*entities.Operator, error)
	List(ctx context.Context, page, limit int) ([]*entities.Operator, int64, error)
	Update(ctx context.Context, op *entities.Operator) error
	Deactivate(ctx context.Context, id string) error
}

// Role identifies the permission level granted to an authenticated operator or API key.
// Use the predefined constants RoleAdmin and RoleOperator rather than bare string literals.
type Role = string

const (
	// RoleAdmin is required for administrative endpoints such as operator CRUD (/api/v1/operators).
	// Authenticated principals without this role receive 403 on admin-gated routes.
	RoleAdmin Role = "admin"
	// RoleOperator is the default role assigned to operators authenticated via OperatorAuth.
	// It satisfies authentication for non-admin management endpoints but does not grant access to admin-gated routes.
	RoleOperator Role = "operator"
)
