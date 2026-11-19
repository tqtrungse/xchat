/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package route

import (
	"context"
	"errors"
	"fmt"
	"net"
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"

	"xchat/internal/iam/adapter/http/httpx"
	"xchat/internal/iam/application/registerdevice"
	"xchat/internal/iam/core/rejection"
	"xchat/internal/iam/domain/user"

	"xchat/pkg/jwt"
	"xchat/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// stubDeviceRegisterer is a DeviceRegisterer that records its call.
type stubDeviceRegisterer struct {
	resp *registerdevice.Response
	err  error

	called bool
	ctx    context.Context
	userID user.ID
	req    registerdevice.Request
}

func (s *stubDeviceRegisterer) Register(
	ctx context.Context,
	userID user.ID,
	req *registerdevice.Request,
) (*registerdevice.Response, error) {
	s.called = true
	s.ctx = ctx
	s.userID = userID
	s.req = *req
	return s.resp, s.err
}

func TestNewRegisterDeviceHandlerRequiresArguments(t *testing.T) {
	logger, rejections := newTestDeps(t)
	usecase := &stubDeviceRegisterer{}

	_, err := NewRegisterDeviceHandler(nil, logger, rejections)
	require.ErrorIs(t, err, errUsecaseRequired)

	_, err = NewRegisterDeviceHandler(usecase, nil, rejections)
	require.ErrorIs(t, err, errLoggerRequired)

	_, err = NewRegisterDeviceHandler(usecase, logger, nil)
	require.ErrorIs(t, err, errRejectionsRequired)

	handler, err := NewRegisterDeviceHandler(usecase, logger, rejections)
	require.NoError(t, err)
	require.NotNil(t, handler)
}

func TestMapRegisterDeviceError(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name           string
		err            error
		wantStatus     int
		wantCode       string
		wantRejection  bool
		wantUnexpected bool
		wantRetryAfter string
	}{
		{
			name:          "invalid request",
			err:           registerdevice.ErrRequestInvalid,
			wantStatus:    nethttp.StatusBadRequest,
			wantCode:      "invalid_request",
			wantRejection: true,
		},
		{
			name:       "user locked",
			err:        registerdevice.ErrUserLocked,
			wantStatus: nethttp.StatusForbidden,
			wantCode:   "user_locked",
		},
		{
			name:       "max devices",
			err:        registerdevice.ErrUserReachMaxDevices,
			wantStatus: nethttp.StatusForbidden,
			wantCode:   "max_devices_reached",
		},
		{
			name:       "user exists",
			err:        registerdevice.ErrUserExists,
			wantStatus: nethttp.StatusConflict,
			wantCode:   "user_exists",
		},
		{
			name:       "device exists",
			err:        registerdevice.ErrDeviceExists,
			wantStatus: nethttp.StatusConflict,
			wantCode:   "device_exists",
		},
		{
			name:           "busy",
			err:            registerdevice.ErrBusy,
			wantStatus:     nethttp.StatusServiceUnavailable,
			wantCode:       "busy",
			wantRetryAfter: retryAfterBusySeconds,
		},
		{
			name:       "internal",
			err:        registerdevice.ErrInternal,
			wantStatus: nethttp.StatusInternalServerError,
			wantCode:   "internal_error",
		},
		{
			name:       "wrapped error is still mapped",
			err:        fmt.Errorf("register: %w", registerdevice.ErrUserLocked),
			wantStatus: nethttp.StatusForbidden,
			wantCode:   "user_locked",
		},
		{
			name:           "unknown error",
			err:            errors.New("driver: connection reset"),
			wantStatus:     nethttp.StatusInternalServerError,
			wantCode:       "internal_error",
			wantUnexpected: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := mapRegisterDeviceError(test.err)
			require.Equal(t, test.wantStatus, got.status)
			require.Equal(t, test.wantCode, got.code)
			require.Equal(t, test.wantRejection, got.reason != nil)
			require.Equal(t, test.wantUnexpected, got.unexpected)
			require.Equal(t, test.wantRetryAfter, got.retryAfter)

			// The response must not leak the text of an unknown error.
			require.NotContains(t, got.description, "driver")
		})
	}
}

// validRegisterDeviceBody is a request body with every field set.
const validRegisterDeviceBody = `{
	"username": "alice",
	"password": "cGFzc3dvcmQ=",
	"phone_hash": "cGhvbmUtaGFzaA==",
	"phone_tail": 32,
	"device_name": "iPhone 15 iOS 18",
	"device_identity_pub": "aWRlbnRpdHk=",
	"device_peer_pub": "cGVlcg=="
}`

// registerDeviceFixture is a handler that runs behind a middleware, which sets
// the claims as authenticate does.
type registerDeviceFixture struct {
	engine     *gin.Engine
	usecase    *stubDeviceRegisterer
	rejections interface{ Count(rejection.Reason) uint64 }
	logger     *log.Logger
	logPath    string
}

// newRegisterDeviceFixture makes the fixture. Each middleware runs after the
// one that sets the claims, and before the handler.
func newRegisterDeviceFixture(
	t *testing.T,
	claims *jwt.Claims,
	usecase *stubDeviceRegisterer,
	middleware ...gin.HandlerFunc,
) *registerDeviceFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	logger, rejections, logPath := newTestDepsWithLog(t)
	handler, err := NewRegisterDeviceHandler(usecase, logger, rejections)
	require.NoError(t, err)

	engine := gin.New()
	group := engine.Group("/api")
	group.Use(func(c *gin.Context) {
		if claims != nil {
			httpx.SetClaims(c, claims)
		}
		c.Next()
	})
	group.Use(middleware...)
	handler.Mount(group)

	return &registerDeviceFixture{
		engine:     engine,
		usecase:    usecase,
		rejections: rejections,
		logger:     logger,
		logPath:    logPath,
	}
}

// log gives the text that the logger wrote.
func (f *registerDeviceFixture) log(t *testing.T) string {
	t.Helper()

	require.NoError(t, f.logger.Sync())
	raw, err := os.ReadFile(f.logPath)
	require.NoError(t, err)
	return string(raw)
}

func (f *registerDeviceFixture) post(body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(nethttp.MethodPost, "/api/device/register", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	f.engine.ServeHTTP(recorder, request)
	return recorder
}

func (f *registerDeviceFixture) requireRejections(t *testing.T, want map[rejection.Reason]uint64) {
	t.Helper()

	for _, reason := range []rejection.Reason{
		rejection.AuthHeader,
		rejection.AuthToken,
		rejection.RequestBody,
		rejection.ResponseWrite,
	} {
		require.EqualValues(t, want[reason], f.rejections.Count(reason), "reason %d", reason)
	}
}

func TestRegisterDeviceSuccess(t *testing.T) {
	usecase := &stubDeviceRegisterer{resp: &registerdevice.Response{DeviceID: "dev-1"}}
	f := newRegisterDeviceFixture(t, &jwt.Claims{UserID: "42"}, usecase)

	recorder := f.post(validRegisterDeviceBody)

	require.Equal(t, nethttp.StatusCreated, recorder.Code)
	require.JSONEq(t, `{"device_id":"dev-1"}`, recorder.Body.String())
	require.Equal(t, httpx.ContentTypeJSON, recorder.Header().Get("Content-Type"))
	require.Equal(t, fmt.Sprint(recorder.Body.Len()), recorder.Header().Get("Content-Length"))

	require.True(t, usecase.called)
	require.Equal(t, user.ID(42), usecase.userID)
	require.NotNil(t, usecase.ctx)
	require.Equal(t, "alice", usecase.req.Username)
	require.Equal(t, uint16(32), usecase.req.PhoneTail)
	require.Equal(t, "iPhone 15 iOS 18", usecase.req.DeviceName)
	f.requireRejections(t, nil)
}

func TestRegisterDeviceRejectsRequest(t *testing.T) {
	for _, test := range []struct {
		name           string
		claims         *jwt.Claims
		body           string
		wantStatus     int
		wantJSON       string
		wantRejections map[rejection.Reason]uint64
	}{
		{
			name:       "no claims",
			claims:     nil,
			body:       validRegisterDeviceBody,
			wantStatus: nethttp.StatusUnauthorized,
			wantJSON:   `{"error":"invalid_token","error_description":"failed to verify token"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.AuthToken: 1,
			},
		},
		{
			name:       "empty user id",
			claims:     &jwt.Claims{},
			body:       validRegisterDeviceBody,
			wantStatus: nethttp.StatusUnauthorized,
			wantJSON:   `{"error":"invalid_token","error_description":"failed to verify token"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.AuthToken: 1,
			},
		},
		{
			name:       "user id is not a number",
			claims:     &jwt.Claims{UserID: "auth0|42"},
			body:       validRegisterDeviceBody,
			wantStatus: nethttp.StatusUnauthorized,
			wantJSON:   `{"error":"invalid_token","error_description":"failed to verify token"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.AuthToken: 1,
			},
		},
		{
			name:       "user id is zero",
			claims:     &jwt.Claims{UserID: "0"},
			body:       validRegisterDeviceBody,
			wantStatus: nethttp.StatusUnauthorized,
			wantJSON:   `{"error":"invalid_token","error_description":"failed to verify token"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.AuthToken: 1,
			},
		},
		{
			name:       "malformed json",
			claims:     &jwt.Claims{UserID: "42"},
			body:       `{"username":`,
			wantStatus: nethttp.StatusBadRequest,
			wantJSON:   `{"error":"invalid_request_body","error_description":"invalid request body"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.RequestBody: 1,
			},
		},
		{
			name:       "body is too large",
			claims:     &jwt.Claims{UserID: "42"},
			body:       `{"username":"` + strings.Repeat("a", maxRegisterDeviceBodyBytes) + `"}`,
			wantStatus: nethttp.StatusRequestEntityTooLarge,
			wantJSON:   `{"error":"request_too_large","error_description":"request body is too large"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.RequestBody: 1,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			usecase := &stubDeviceRegisterer{resp: &registerdevice.Response{DeviceID: "dev-1"}}
			f := newRegisterDeviceFixture(t, test.claims, usecase)

			recorder := f.post(test.body)

			require.Equal(t, test.wantStatus, recorder.Code)
			require.JSONEq(t, test.wantJSON, recorder.Body.String())
			require.False(t, usecase.called, "the use case must not run for this request")
			f.requireRejections(t, test.wantRejections)
		})
	}
}

func TestRegisterDeviceUsecaseErrors(t *testing.T) {
	for _, test := range []struct {
		name           string
		err            error
		wantStatus     int
		wantJSON       string
		wantRetryAfter string
		wantRejections map[rejection.Reason]uint64
	}{
		{
			name:       "invalid request",
			err:        registerdevice.ErrRequestInvalid,
			wantStatus: nethttp.StatusBadRequest,
			wantJSON:   `{"error":"invalid_request","error_description":"invalid client request"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.RequestBody: 1,
			},
		},
		{
			name:       "user locked",
			err:        registerdevice.ErrUserLocked,
			wantStatus: nethttp.StatusForbidden,
			wantJSON:   `{"error":"user_locked","error_description":"user is locked"}`,
		},
		{
			name:           "busy",
			err:            registerdevice.ErrBusy,
			wantStatus:     nethttp.StatusServiceUnavailable,
			wantJSON:       `{"error":"busy","error_description":"retry again"}`,
			wantRetryAfter: retryAfterBusySeconds,
		},
		{
			name:       "unknown error does not leak its text",
			err:        errors.New("driver: connection reset"),
			wantStatus: nethttp.StatusInternalServerError,
			wantJSON:   `{"error":"internal_error","error_description":"internal error"}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			usecase := &stubDeviceRegisterer{err: test.err}
			f := newRegisterDeviceFixture(t, &jwt.Claims{UserID: "42"}, usecase)

			recorder := f.post(validRegisterDeviceBody)

			require.Equal(t, test.wantStatus, recorder.Code)
			require.JSONEq(t, test.wantJSON, recorder.Body.String())
			require.Equal(t, test.wantRetryAfter, recorder.Header().Get("Retry-After"))
			require.True(t, usecase.called)
			f.requireRejections(t, test.wantRejections)
		})
	}
}

// failingWriter is a gin response writer whose body write fails.
type failingWriter struct {
	gin.ResponseWriter
	err error
}

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func failWrites(err error) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer = failingWriter{ResponseWriter: c.Writer, err: err}
		c.Next()
	}
}

func TestRegisterDeviceResponseWriteFailure(t *testing.T) {
	for _, test := range []struct {
		name           string
		err            error
		wantRejections map[rejection.Reason]uint64
		wantLog        string
		wantNoLog      string
	}{
		{
			name: "client reset the connection",
			err:  &net.OpError{Op: "write", Err: os.NewSyscallError("write", syscall.ECONNRESET)},
			wantRejections: map[rejection.Reason]uint64{
				rejection.ResponseWrite: 1,
			},
			wantLog:   `"msg":"client left before the response of register device"`,
			wantNoLog: `failed to respond to register device`,
		},
		{
			name: "broken pipe",
			err:  syscall.EPIPE,
			wantRejections: map[rejection.Reason]uint64{
				rejection.ResponseWrite: 1,
			},
			wantLog:   `"device_id":"dev-1"`,
			wantNoLog: `failed to respond to register device`,
		},
		{
			name:      "write fails for another reason",
			err:       errors.New("disk on fire"),
			wantLog:   `"msg":"failed to respond to register device"`,
			wantNoLog: `client left before`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			usecase := &stubDeviceRegisterer{resp: &registerdevice.Response{DeviceID: "dev-1"}}
			f := newRegisterDeviceFixture(t, &jwt.Claims{UserID: "42"}, usecase, failWrites(test.err))

			f.post(validRegisterDeviceBody)

			require.True(t, usecase.called, "the use case ran: the device exists")
			f.requireRejections(t, test.wantRejections)

			logged := f.log(t)
			require.Contains(t, logged, test.wantLog)
			require.NotContains(t, logged, test.wantNoLog)
			require.Contains(t, logged, `"user_id":42`)
		})
	}
}
