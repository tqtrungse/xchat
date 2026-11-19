/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package httpx

import (
	"errors"
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"xchat/services/iam/internal/application/device/register"

	"github.com/gin-gonic/gin"
	"github.com/mailru/easyjson/jwriter"
	"github.com/stretchr/testify/require"
)

// failingMarshaler is an easyjson.Marshaler whose encoding fails.
type failingMarshaler struct{ err error }

func (m failingMarshaler) MarshalEasyJSON(w *jwriter.Writer) {
	w.Error = m.err
}

// failingWriter is a gin response writer whose body write fails.
type failingWriter struct {
	gin.ResponseWriter
	err error
}

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRespondSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	err := RespondSuccess(c, nethttp.StatusCreated, &register.Response{DeviceID: "abc"})

	require.NoError(t, err)
	require.Equal(t, nethttp.StatusCreated, recorder.Code)
	require.JSONEq(t, `{"device_id":"abc"}`, recorder.Body.String())
	require.Equal(t, ContentTypeJSON, recorder.Header().Get("Content-Type"))
	require.Equal(t, fmt.Sprint(recorder.Body.Len()), recorder.Header().Get("Content-Length"))
	require.Empty(t, c.Errors)
}

func TestSetChallenge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name        string
		setRealm    bool
		realm       string
		code        string
		description string
		wantHeader  string
	}{
		{
			name:       "realm without error",
			setRealm:   true,
			realm:      "iam",
			wantHeader: `Bearer realm="iam"`,
		},
		{
			name:        "realm with error",
			setRealm:    true,
			realm:       "iam",
			code:        "invalid_token",
			description: "failed to verify token",
			wantHeader:  `Bearer realm="iam", error="invalid_token", error_description="failed to verify token"`,
		},
		{
			name:        "empty realm with error",
			setRealm:    true,
			realm:       "",
			code:        "invalid_token",
			description: "failed to verify token",
			wantHeader:  `Bearer error="invalid_token", error_description="failed to verify token"`,
		},
		{
			name:       "no realm and no error",
			setRealm:   false,
			wantHeader: "Bearer",
		},
		{
			name:        "realm with quote",
			setRealm:    true,
			realm:       `a"b`,
			code:        "invalid_request",
			description: "bad",
			wantHeader:  `Bearer realm="a\"b", error="invalid_request", error_description="bad"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			if tc.setRealm {
				SetRealm(c, tc.realm)
			}

			SetChallenge(c, tc.code, tc.description)

			require.Equal(t, tc.wantHeader, recorder.Header().Get("WWW-Authenticate"))
		})
	}
}

func TestRespondSuccessEncodeFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	cause := errors.New("secret value")

	err := RespondSuccess(c, nethttp.StatusCreated, failingMarshaler{err: cause})

	require.ErrorIs(t, err, ErrEncode)
	require.ErrorIs(t, err, cause)
	require.Equal(t, nethttp.StatusInternalServerError, recorder.Code)
	require.JSONEq(t, `{"error":"internal_error","error_description":"internal error"}`, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "secret", "the response must not contain the error")
	require.Empty(t, recorder.Header().Get("Content-Length"))
}

func TestRespondSuccessWriteFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	cause := errors.New("broken pipe")
	c.Writer = failingWriter{ResponseWriter: c.Writer, err: cause}

	err := RespondSuccess(c, nethttp.StatusCreated, &register.Response{DeviceID: "abc"})

	require.ErrorIs(t, err, cause, "the caller must see the write error")
	require.NotErrorIs(t, err, ErrEncode, "a failed write is not an encoding error")
	require.Len(t, c.Errors, 1)
	require.ErrorIs(t, c.Errors.Last().Err, cause)
}
