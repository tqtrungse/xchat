/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"xchat/internal/iam/core/rejection"
	"xchat/pkg/jwt"

	"github.com/gin-gonic/gin"
)

// authenticate requires a Bearer token, verifies it, and attaches its claims to
// the request context before invoking the next handler.
func (s *Server) authenticate(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		_ = c.Error(errAuthHeaderNotFound)
		s.rejections.Record(rejection.AuthHeader, errAuthHeaderNotFound)
		// RFC 6750 section 3: do not send the error attribute when the
		// request has no credentials.
		c.Header("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q`, bearerRealm))
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":             "invalid_request",
			"error_description": errAuthHeaderNotFound.Error(),
		})
		return
	}

	scheme, token, found := strings.Cut(authHeader, " ")
	token = strings.TrimLeft(token, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		_ = c.Error(errAuthHeaderNotFound)
		s.rejections.Record(rejection.AuthHeader, errAuthHeaderNotFound)
		respondAuthError(c, http.StatusUnauthorized, "invalid_request", errAuthHeaderNotFound.Error())
		return
	}

	claims, err := s.auth.Verify(token)
	if err != nil {
		_ = c.Error(err)
		if errors.Is(err, jwt.ErrVerifierUnavailable) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":             "server_error",
				"error_description": errAuthUnavailable.Error(),
			})
			return
		}
		s.rejections.Record(rejection.AuthToken, err)
		respondAuthError(c, http.StatusUnauthorized, "invalid_token", errAuthVerify.Error())
		return
	}

	// Attach claims to the context.
	c.Set(claimsContextKey, claims)
	c.Next()
}

// respondAuthError writes an authentication error. The WWW-Authenticate
// header contains the error code and the error description. The description
// must be a constant, because it goes into the header.
func respondAuthError(c *gin.Context, status int, code string, description string) {
	c.Header("WWW-Authenticate", fmt.Sprintf(
		`Bearer realm=%q, error=%q, error_description=%q`,
		bearerRealm,
		code,
		description,
	))
	c.AbortWithStatusJSON(status, gin.H{
		"error":             code,
		"error_description": description,
	})
}
