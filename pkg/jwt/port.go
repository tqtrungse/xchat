/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package jwt

import "github.com/golang-jwt/jwt/v5"

// StdClaims holds the standard claims that this package knows. Code that needs
// only the standard claims gets a *StdClaims. Thus, that code does not need a
// type parameter for the private claims of each service. Examples are a
// middleware that logs sub and scope, and a scope check that all services use.
// For an ExtClaims value, give &claims.StdClaims.
type StdClaims struct {
	jwt.RegisteredClaims
	Scope string `json:"scope"`
}

// ExtClaims holds the claims of a verified token. The identity provider
// puts the private claims of a service in the object "ext". Verify decodes that
// object into Extension. If the token has no "ext" claim, Extension is the zero
// value of T. Thus, the service must check its required private claims.
type ExtClaims[T any] struct {
	StdClaims
	Extension T `json:"ext"`
}

// Auth verifies tokens and decodes their claims into an ExtClaims.
type Auth[T any] interface {
	Verify(token string) (*ExtClaims[T], error)
	Ready() bool
}
