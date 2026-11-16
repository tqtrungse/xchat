/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package jwt

import "github.com/golang-jwt/jwt/v5"

type Claims struct {
	jwt.RegisteredClaims
	Scope string `json:"scope"`
}

type Auth interface {
	Verify(token string) (*Claims, error)
	Ready() bool
}
