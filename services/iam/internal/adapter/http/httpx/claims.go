/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package httpx

import (
	"strconv"

	"xchat/pkg/jwt"

	"github.com/gin-gonic/gin"
)

type contextKey uint8

const (
	claimsContextKey contextKey = iota
	realmContextKey
)

// SetClaims attaches the claims of the verified token to the request. The
// authentication middleware calls it.
func SetClaims(c *gin.Context, claims *jwt.Claims) {
	c.Set(claimsContextKey, claims)
}

// ClaimsFromContext gives the claims that the authentication middleware
// attached to the request. The second result is false if the middleware did not
// run.
//
// Do not read the claims with c.Value. For a key that is not a string, c.Value
// reads the context of the request, and not the values of c.Set.
func ClaimsFromContext(c *gin.Context) (*jwt.Claims, bool) {
	value, exists := c.Get(claimsContextKey)
	if !exists {
		return nil, false
	}
	claims, exists := value.(*jwt.Claims)
	return claims, exists
}

// AuthenticatedUserID gives the numeric ID of the user in the claims that the
// authentication middleware attached to the request. ok is false if the claims
// are missing, or if the ID is empty, is not a number, or is zero. A handler
// answers that case with RespondInvalidToken.
//
// The ID is parsed as a number so that alternate spellings such as "42" and
// "042" are the same user. Otherwise, they could create separate quotas.
func AuthenticatedUserID(c *gin.Context) (userID uint64, ok bool) {
	claims, ok := ClaimsFromContext(c)
	if !ok || claims == nil || claims.UserID == "" {
		return 0, false
	}

	userID, err := strconv.ParseUint(claims.UserID, 10, 64)
	if err != nil || userID == 0 {
		return 0, false
	}
	return userID, true
}
