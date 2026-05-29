package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type TokenValidator interface {
	Validate(ctx context.Context, token string) (*AuthClaims, error)
}

type AuthClaims struct {
	Subject string
	Role    string
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
