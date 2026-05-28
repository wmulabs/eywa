package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"golang.org/x/crypto/bcrypt"
)

type OperatorAuth struct {
	repo     ports.OperatorRepository
	secret   []byte
	tokenTTL time.Duration
}

func NewOperatorAuth(repo ports.OperatorRepository, secret []byte) *OperatorAuth {
	return &OperatorAuth{repo: repo, secret: secret, tokenTTL: 8 * time.Hour}
}

func (a *OperatorAuth) WithTokenTTL(ttl time.Duration) *OperatorAuth {
	a.tokenTTL = ttl
	return a
}

type operatorClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (a *OperatorAuth) Login(ctx context.Context, email, password string) (token string, expiresAt time.Time, err error) {
	op, err := a.repo.FindByEmail(ctx, email)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}
	if !op.IsActive {
		// Return the same error as invalid credentials to prevent user enumeration.
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(op.PasswordHash), []byte(password)); err != nil {
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}

	expiresAt = time.Now().Add(a.tokenTTL)
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, operatorClaims{
		Role: op.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   op.ID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	signed, err := t.SignedString(a.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, expiresAt, nil
}

func (a *OperatorAuth) Validate(_ context.Context, token string) (*ports.AuthClaims, error) {
	t, err := jwt.ParseWithClaims(token, &operatorClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	})
	if err != nil || !t.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := t.Claims.(*operatorClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	return &ports.AuthClaims{Subject: claims.Subject, Role: claims.Role}, nil
}

func (a *OperatorAuth) CreateOperator(ctx context.Context, op *entities.Operator) error {
	return a.repo.Create(ctx, op)
}

func (a *OperatorAuth) ListOperators(ctx context.Context, page, limit int) ([]*entities.Operator, int64, error) {
	return a.repo.List(ctx, page, limit)
}

func (a *OperatorAuth) FindOperatorByID(ctx context.Context, id string) (*entities.Operator, error) {
	return a.repo.FindByID(ctx, id)
}

func (a *OperatorAuth) UpdateOperator(ctx context.Context, op *entities.Operator) error {
	return a.repo.Update(ctx, op)
}

func (a *OperatorAuth) DeactivateOperator(ctx context.Context, id string) error {
	return a.repo.Deactivate(ctx, id)
}

func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}
