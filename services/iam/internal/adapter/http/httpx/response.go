/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jwriter"
)

// ContentTypeJSON is the Content-Type of each JSON response. It is the value
// that gin writes for AbortWithStatusJSON, so every response of the server has
// the same header.
const ContentTypeJSON = "application/json; charset=utf-8"

// SetRealm attaches the realm of the WWW-Authenticate header to the request.
// The authentication middleware calls it before the next handler. Thus a
// handler does not need the server config to write an authentication error.
func SetRealm(c *gin.Context, realm string) {
	c.Set(realmContextKey, realm)
}

// realmFromContext gives the realm that SetRealm attached to the request. It
// gives an empty string if SetRealm did not run.
func realmFromContext(c *gin.Context) string {
	value, exists := c.Get(realmContextKey)
	if !exists {
		return ""
	}
	realm, _ := value.(string)
	return realm
}

// SetChallenge writes the WWW-Authenticate header with the Bearer scheme. The
// header contains the realm that SetRealm attached. If the realm is empty, the
// header has no realm attribute. If code is empty, the header has no error
// attributes. RFC 6750 section 3 requires this for a request without
// credentials. The description must be a constant, because it goes into the
// header.
func SetChallenge(c *gin.Context, code string, description string) {
	attributes := make([]string, 0, 3)
	realm := realmFromContext(c)
	if realm != "" {
		attributes = append(attributes, fmt.Sprintf("realm=%q", realm))
	}
	if code != "" {
		attributes = append(
			attributes,
			fmt.Sprintf("error=%q", code),
			fmt.Sprintf("error_description=%q", description),
		)
	}
	if len(attributes) == 0 {
		c.Header("WWW-Authenticate", "Bearer")
		return
	}
	c.Header("WWW-Authenticate", "Bearer "+strings.Join(attributes, ", "))
}

// RespondError writes an error response. The description goes to the client,
// so it must be a constant.
func RespondError(c *gin.Context, status int, code string, description string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error":             code,
		"error_description": description,
	})
}

// RespondAuthError writes an authentication error. The WWW-Authenticate
// header contains the realm, the error code, and the error description. See
// SetChallenge. The description must be a constant, because it goes into the
// header.
func RespondAuthError(c *gin.Context, status int, code string, description string) {
	SetChallenge(c, code, description)
	RespondError(c, status, code, description)
}

// RespondInvalidToken writes the 401 response for a token whose claims a
// handler cannot use.
func RespondInvalidToken(c *gin.Context) {
	RespondAuthError(c, http.StatusUnauthorized, "invalid_token", ErrAuthVerify.Error())
}

// RespondSuccess writes v as the JSON body of the response with the given
// status. v must not be nil.
//
// RespondSuccess encodes v before it writes the status. After the status line
// goes out, a failure can no longer change the status.
//
// If the encoding fails, RespondSuccess writes a 500 response and returns an
// error that wraps ErrEncode. The error can hold values of v, so the response
// never contains it.
//
// DumpTo writes the buffer chunks to the response and releases them, so the
// body is not copied to one slice first. If the write fails, the status is
// already sent and RespondSuccess cannot answer again. It returns the write
// error, and it adds that error to c.Errors, so the access log counts it. The
// caller uses IsClientGone to tell a client that left from a failure of the
// server.
func RespondSuccess(c *gin.Context, status int, v easyjson.Marshaler) error {
	var jw jwriter.Writer
	v.MarshalEasyJSON(&jw)
	if jw.Error != nil {
		RespondError(c, http.StatusInternalServerError, "internal_error", ErrInternal.Error())
		return fmt.Errorf("%w: %w", ErrEncode, jw.Error)
	}

	c.Header("Content-Type", ContentTypeJSON)
	c.Header("Content-Length", strconv.Itoa(jw.Size()))
	c.Status(status)

	_, err := jw.DumpTo(c.Writer)
	if err != nil {
		_ = c.Error(err)
	}
	return err
}
