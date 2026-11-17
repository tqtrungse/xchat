/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package jwt

import "errors"

var (
	ErrIssuerRequired      = errors.New("JWT issuer is required")
	ErrIssuerInsecure      = errors.New("JWT issuer is insecure")
	ErrAudienceRequired    = errors.New("JWT audience is required")
	ErrJWKSURLRequired     = errors.New("JWT JWKS URL is required")
	ErrJWKSURLInsecure     = errors.New("JWT JWKS URL is insecure")
	ErrLoggerRequired      = errors.New("logger is required")
	ErrTokenInvalid        = errors.New("invalid token")
	ErrVerifierUnavailable = errors.New("token verifier is unavailable")
)
