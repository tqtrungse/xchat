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
	"time"

	"xchat/services/iam/internal/adapter/http/httpx"
	"xchat/services/iam/internal/adapter/http/rejection"
	"xchat/services/iam/internal/adapter/http/route/mocks"
	"xchat/services/iam/internal/application/device/register"
	"xchat/services/iam/internal/domain/user"

	"xchat/pkg/jwt"
	"xchat/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

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
			err:           register.ErrRequestInvalid,
			wantStatus:    nethttp.StatusBadRequest,
			wantCode:      "invalid_request",
			wantRejection: true,
		},
		{
			name:       "user locked",
			err:        register.ErrUserLocked,
			wantStatus: nethttp.StatusForbidden,
			wantCode:   "user_locked",
		},
		{
			name:       "max devices",
			err:        register.ErrUserReachMaxDevices,
			wantStatus: nethttp.StatusForbidden,
			wantCode:   "max_devices_reached",
		},
		{
			name:       "user exists",
			err:        register.ErrUserExists,
			wantStatus: nethttp.StatusConflict,
			wantCode:   "user_exists",
		},
		{
			name:       "device exists",
			err:        register.ErrDeviceExists,
			wantStatus: nethttp.StatusConflict,
			wantCode:   "device_exists",
		},
		{
			name:           "busy",
			err:            register.ErrBusy,
			wantStatus:     nethttp.StatusServiceUnavailable,
			wantCode:       "busy",
			wantRetryAfter: retryAfterBusySeconds,
		},
		{
			name:       "internal",
			err:        register.ErrInternal,
			wantStatus: nethttp.StatusInternalServerError,
			wantCode:   "internal_error",
		},
		{
			name:       "wrapped error is still mapped",
			err:        fmt.Errorf("register: %w", register.ErrUserLocked),
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
	usecase    *mocks.MockDeviceRegisterer
	rejections interface{ Count(rejection.Reason) uint64 }
	logger     *log.Logger
	logPath    string
}

// newRegisterDeviceFixture makes the fixture with testUsecaseTimeout. Each
// middleware runs after the one that sets the claims, and before the handler.
func newRegisterDeviceFixture(
	t *testing.T,
	claims *jwt.Claims,
	middleware ...gin.HandlerFunc,
) *registerDeviceFixture {
	t.Helper()
	return newRegisterDeviceFixtureWithTimeout(t, testUsecaseTimeout, claims, middleware...)
}

// newRegisterDeviceFixtureWithTimeout makes the fixture with timeout as the
// deadline of the use case.
func newRegisterDeviceFixtureWithTimeout(
	t *testing.T,
	timeout time.Duration,
	claims *jwt.Claims,
	middleware ...gin.HandlerFunc,
) *registerDeviceFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	usecase := mocks.NewMockDeviceRegisterer(gomock.NewController(t))
	logger, rejections, logPath := newTestDepsWithLog(t)
	handler := NewRegisterDeviceHandler(usecase, timeout, logger, rejections)

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

// expectRegister sets the call that the use case must receive for the valid
// body of the tests. The call returns resp and err. check can inspect the
// request. It runs before the handler clears the buffers of the request.
func (f *registerDeviceFixture) expectRegister(
	check func(req *register.Request),
	resp *register.Response,
	err error,
) {
	f.usecase.EXPECT().
		Register(gomock.Not(gomock.Nil()), user.ID(42), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ user.ID, req *register.Request) (*register.Response, error) {
			if check != nil {
				check(req)
			}
			return resp, err
		})
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
	f := newRegisterDeviceFixture(t, &jwt.Claims{UserID: "42"})
	f.expectRegister(
		func(req *register.Request) {
			require.Equal(t, "alice", req.Username)
			require.Equal(t, uint16(32), req.PhoneTail)
			require.Equal(t, "iPhone 15 iOS 18", req.DeviceName)
		},
		&register.Response{DeviceID: "dev-1"},
		nil,
	)

	recorder := f.post(validRegisterDeviceBody)

	require.Equal(t, nethttp.StatusCreated, recorder.Code)
	require.JSONEq(t, `{"device_id":"dev-1"}`, recorder.Body.String())
	require.Equal(t, httpx.ContentTypeJSON, recorder.Header().Get("Content-Type"))
	require.Equal(t, fmt.Sprint(recorder.Body.Len()), recorder.Header().Get("Content-Length"))

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
			// The fixture has no expected call. The mock fails the test if the
			// handler calls the use case for this request.
			f := newRegisterDeviceFixture(t, test.claims)

			recorder := f.post(test.body)

			require.Equal(t, test.wantStatus, recorder.Code)
			require.JSONEq(t, test.wantJSON, recorder.Body.String())
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
			err:        register.ErrRequestInvalid,
			wantStatus: nethttp.StatusBadRequest,
			wantJSON:   `{"error":"invalid_request","error_description":"invalid client request"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.RequestBody: 1,
			},
		},
		{
			name:       "user locked",
			err:        register.ErrUserLocked,
			wantStatus: nethttp.StatusForbidden,
			wantJSON:   `{"error":"user_locked","error_description":"user is locked"}`,
		},
		{
			name:           "busy",
			err:            register.ErrBusy,
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
			f := newRegisterDeviceFixture(t, &jwt.Claims{UserID: "42"})
			f.expectRegister(nil, nil, test.err)

			recorder := f.post(validRegisterDeviceBody)

			require.Equal(t, test.wantStatus, recorder.Code)
			require.JSONEq(t, test.wantJSON, recorder.Body.String())
			require.Equal(t, test.wantRetryAfter, recorder.Header().Get("Retry-After"))
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
			f := newRegisterDeviceFixture(t, &jwt.Claims{UserID: "42"}, failWrites(test.err))
			f.expectRegister(nil, &register.Response{DeviceID: "dev-1"}, nil)

			f.post(validRegisterDeviceBody)

			f.requireRejections(t, test.wantRejections)

			logged := f.log(t)
			require.Contains(t, logged, test.wantLog)
			require.NotContains(t, logged, test.wantNoLog)
			require.Contains(t, logged, `"user_id":42`)
		})
	}
}

// TestRegisterDeviceUsecaseDeadline checks that the use case gets a context
// with the deadline of the handler. The handler must cancel the context after
// the use case returns.
func TestRegisterDeviceUsecaseDeadline(t *testing.T) {
	for _, test := range []struct {
		name         string
		timeout      time.Duration
		wantDeadline bool
	}{
		{
			name:         "positive timeout sets a deadline",
			timeout:      testUsecaseTimeout,
			wantDeadline: true,
		},
		{
			name:         "negative timeout disables the deadline",
			timeout:      -time.Second,
			wantDeadline: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRegisterDeviceFixtureWithTimeout(t, test.timeout, &jwt.Claims{UserID: "42"})

			var usecaseCtx context.Context
			f.usecase.EXPECT().
				Register(gomock.Any(), user.ID(42), gomock.Any()).
				DoAndReturn(func(ctx context.Context, _ user.ID, _ *register.Request) (*register.Response, error) {
					usecaseCtx = ctx
					deadline, hasDeadline := ctx.Deadline()
					require.Equal(t, test.wantDeadline, hasDeadline)
					if test.wantDeadline {
						remaining := time.Until(deadline)
						require.Positive(t, remaining)
						require.LessOrEqual(t, remaining, test.timeout)
					}
					return nil, register.ErrBusy
				})

			recorder := f.post(validRegisterDeviceBody)

			require.Equal(t, nethttp.StatusServiceUnavailable, recorder.Code)
			require.Equal(t, retryAfterBusySeconds, recorder.Header().Get("Retry-After"))
			require.NotNil(t, usecaseCtx)
			if test.wantDeadline {
				require.ErrorIs(t, usecaseCtx.Err(), context.Canceled, "the handler must cancel the context")
			}
		})
	}
}
