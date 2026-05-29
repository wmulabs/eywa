package auth

import (
	"context"
	"crypto/rsa"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// jwtClaims is shared by JWTValidator and JWKSValidator.
type jwtClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type JWTValidator struct {
	keyFunc jwt.Keyfunc
}

func NewJWTValidator(secret []byte) ports.TokenValidator {
	return &JWTValidator{
		keyFunc: func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return secret, nil
		},
	}
}

func NewJWTValidatorRS256(pubKey *rsa.PublicKey) ports.TokenValidator {
	return &JWTValidator{
		keyFunc: func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return pubKey, nil
		},
	}
}

func (v *JWTValidator) Validate(_ context.Context, token string) (*ports.AuthClaims, error) {
	t, err := jwt.ParseWithClaims(token, &jwtClaims{}, v.keyFunc)
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
