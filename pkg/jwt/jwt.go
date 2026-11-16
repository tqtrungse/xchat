/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package jwt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrIssuerRequired      = errors.New("JWT issuer is required")
	ErrAudienceRequired    = errors.New("JWT audience is required")
	ErrJwksUrlRequired     = errors.New("JWT JWKS URL is required")
	ErrTokenInvalid        = errors.New("invalid token")
	ErrVerifierUnavailable = errors.New("token verifier is unavailable")
)

const clockSkew = 30 * time.Second

type AuthConfig struct {
	Issuer   string
	Audience string
	JWKSURL  string
}

type auth struct {
	jwks keyfunc.Keyfunc
	cfg  AuthConfig
}

func NewAuth(ctx context.Context, cfg AuthConfig) (Auth, error) {
	// These checks do not depend on the config layer. They keep the
	// invariant of this package: an Auth has each OIDC setting.
	if cfg.Issuer == "" {
		return nil, ErrIssuerRequired
	}
	if cfg.Audience == "" {
		return nil, ErrAudienceRequired
	}
	if cfg.JWKSURL == "" {
		return nil, ErrJwksUrlRequired
	}

	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{cfg.JWKSURL})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	return &auth{
		jwks: jwks,
		cfg:  cfg,
	}, nil
}

func (v *auth) Verify(token string) (*Claims, error) {
	claims := new(Claims)
	tk, err := jwt.ParseWithClaims(
		token,
		claims,
		v.jwks.Keyfunc,
		// Force signature algorithm to prevent algorithm confusion attacks.
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.cfg.Issuer),
		jwt.WithAudience(v.cfg.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(clockSkew),
	)
	if err != nil {
		// A keyfunc failure with no key in the storage means that the JWKS is
		// not available. All other failures mean that the token is not valid.
		if errors.Is(err, jwt.ErrTokenUnverifiable) && !v.Ready() {
			return nil, fmt.Errorf("%w: %w", ErrVerifierUnavailable, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrTokenInvalid, err)
	}
	if !tk.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

// Ready reports whether the verifier has at least one verification key. A
// deployment can call Ready before it sends traffic to this service.
func (v *auth) Ready() bool {
	keys, err := v.jwks.Storage().KeyReadAll(context.Background())
	if err != nil {
		return false
	}
	return len(keys) > 0
}
