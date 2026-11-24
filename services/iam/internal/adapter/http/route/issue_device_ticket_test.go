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
	"xchat/services/iam/internal/application/device/issueticket"
	"xchat/services/iam/internal/domain/user"
	"xchat/services/internal/shared/hub"

	"xchat/pkg/jwt"
	"xchat/pkg/log"
	slicepool "xchat/pkg/pool/slice"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const validIssueDeviceTicketBody = `{"device_id":"1234"}`

// newTestTicketResponse makes a response. The ticket comes from slicepool, as
// the ticket of the real use case does.
func newTestTicketResponse() *issueticket.Response {
	ticketBytes := slicepool.Get(3)
	copy(ticketBytes, []byte{1, 2, 3})

	return &issueticket.Response{
		Ticket: ticketBytes,
		Hub: hub.Descriptor{
			Address:   "hub.example.com:443",
			Audience:  "hub",
			KeyID:     7,
			PublicKey: []byte{4, 5, 6},
			Protocol:  hub.Protocol,
			Version:   hub.WireVersion,
		},
	}
}

func TestMapIssueDeviceTicketError(t *testing.T) {
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
			err:           issueticket.ErrRequestInvalid,
			wantStatus:    nethttp.StatusBadRequest,
			wantCode:      "invalid_request",
			wantRejection: true,
		},
		{
			name:           "busy",
			err:            issueticket.ErrBusy,
			wantStatus:     nethttp.StatusServiceUnavailable,
			wantCode:       "busy",
			wantRetryAfter: retryAfterBusySeconds,
		},
		{
			name:       "device not found",
			err:        issueticket.ErrDeviceNotFound,
			wantStatus: nethttp.StatusNotFound,
			wantCode:   "device_not_found",
		},
		{
			name:       "device inactive",
			err:        issueticket.ErrDeviceInactive,
			wantStatus: nethttp.StatusForbidden,
			wantCode:   "device_inactive",
		},
		{
			name:       "internal",
			err:        issueticket.ErrInternal,
			wantStatus: nethttp.StatusInternalServerError,
			wantCode:   "internal_error",
		},
		{
			name:       "wrapped error is still mapped",
			err:        fmt.Errorf("issue: %w", issueticket.ErrDeviceInactive),
			wantStatus: nethttp.StatusForbidden,
			wantCode:   "device_inactive",
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

			got := mapIssueDeviceTicketError(test.err)
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

// issueDeviceTicketFixture is a handler that runs behind a middleware, which
// sets the claims as authenticate does.
type issueDeviceTicketFixture struct {
	engine     *gin.Engine
	usecase    *mocks.MockDeviceTicketIssuer
	rejections interface{ Count(rejection.Reason) uint64 }
	logger     *log.Logger
	logPath    string
}

// newIssueDeviceTicketFixture makes the fixture with testUsecaseTimeout. Each
// middleware runs after the one that sets the claims, and before the handler.
func newIssueDeviceTicketFixture(
	t *testing.T,
	claims *jwt.Claims,
	middleware ...gin.HandlerFunc,
) *issueDeviceTicketFixture {
	t.Helper()
	return newIssueDeviceTicketFixtureWithTimeout(t, testUsecaseTimeout, claims, middleware...)
}

// newIssueDeviceTicketFixtureWithTimeout makes the fixture with timeout as the
// deadline of the use case.
func newIssueDeviceTicketFixtureWithTimeout(
	t *testing.T,
	timeout time.Duration,
	claims *jwt.Claims,
	middleware ...gin.HandlerFunc,
) *issueDeviceTicketFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	usecase := mocks.NewMockDeviceTicketIssuer(gomock.NewController(t))
	logger, rejections, logPath := newTestDepsWithLog(t)
	handler := NewIssueDeviceTicketHandler(usecase, timeout, logger, rejections)

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

	return &issueDeviceTicketFixture{
		engine:     engine,
		usecase:    usecase,
		rejections: rejections,
		logger:     logger,
		logPath:    logPath,
	}
}

// expectIssueTicket sets the call that the use case must receive for the valid
// body of the tests. The call returns resp and err.
func (f *issueDeviceTicketFixture) expectIssueTicket(resp *issueticket.Response, err error) {
	f.usecase.EXPECT().
		IssueTicket(
			gomock.Not(gomock.Nil()),
			user.ID(42),
			issueticket.Request{DeviceID: "1234"},
		).
		Return(resp, err)
}

// log gives the text that the logger wrote.
func (f *issueDeviceTicketFixture) log(t *testing.T) string {
	t.Helper()

	require.NoError(t, f.logger.Sync())
	raw, err := os.ReadFile(f.logPath)
	require.NoError(t, err)
	return string(raw)
}

func (f *issueDeviceTicketFixture) post(body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(nethttp.MethodPost, "/api/device/ticket", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	f.engine.ServeHTTP(recorder, request)
	return recorder
}

func (f *issueDeviceTicketFixture) requireRejections(t *testing.T, want map[rejection.Reason]uint64) {
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

func TestIssueDeviceTicketSuccess(t *testing.T) {
	resp := newTestTicketResponse()
	ticketBytes := resp.Ticket
	f := newIssueDeviceTicketFixture(t, &jwt.Claims{UserID: "42"})
	f.expectIssueTicket(resp, nil)

	recorder := f.post(validIssueDeviceTicketBody)

	require.Equal(t, nethttp.StatusOK, recorder.Code)
	require.JSONEq(
		t,
		`{
			"ticket": "AQID",
			"hub": {
				"address": "hub.example.com:443",
				"audience": "hub",
				"key_id": 7,
				"public_key": "BAUG",
				"protocol": "Noise_NK_25519_ChaChaPoly_SHA256",
				"version": 2
			}
		}`,
		recorder.Body.String(),
	)
	require.Equal(t, httpx.ContentTypeJSON, recorder.Header().Get("Content-Type"))
	require.Equal(t, fmt.Sprint(recorder.Body.Len()), recorder.Header().Get("Content-Length"))

	f.requireRejections(t, nil)

	// The handler must clear the ticket after it writes the response.
	require.Equal(t, []byte{0, 0, 0}, ticketBytes)
	// The handler must not change the hub public key. The use case shares it.
	require.Equal(t, []byte{4, 5, 6}, resp.Hub.PublicKey)
}

func TestIssueDeviceTicketRejectsRequest(t *testing.T) {
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
			body:       validIssueDeviceTicketBody,
			wantStatus: nethttp.StatusUnauthorized,
			wantJSON:   `{"error":"invalid_token","error_description":"failed to verify token"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.AuthToken: 1,
			},
		},
		{
			name:       "empty user id",
			claims:     &jwt.Claims{},
			body:       validIssueDeviceTicketBody,
			wantStatus: nethttp.StatusUnauthorized,
			wantJSON:   `{"error":"invalid_token","error_description":"failed to verify token"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.AuthToken: 1,
			},
		},
		{
			name:       "user id is zero",
			claims:     &jwt.Claims{UserID: "0"},
			body:       validIssueDeviceTicketBody,
			wantStatus: nethttp.StatusUnauthorized,
			wantJSON:   `{"error":"invalid_token","error_description":"failed to verify token"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.AuthToken: 1,
			},
		},
		{
			name:       "malformed json",
			claims:     &jwt.Claims{UserID: "42"},
			body:       `{"device_id":`,
			wantStatus: nethttp.StatusBadRequest,
			wantJSON:   `{"error":"invalid_request_body","error_description":"invalid request body"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.RequestBody: 1,
			},
		},
		{
			name:       "body is too large",
			claims:     &jwt.Claims{UserID: "42"},
			body:       `{"device_id":"` + strings.Repeat("1", maxIssueDeviceTicketBodyBytes) + `"}`,
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
			f := newIssueDeviceTicketFixture(t, test.claims)

			recorder := f.post(test.body)

			require.Equal(t, test.wantStatus, recorder.Code)
			require.JSONEq(t, test.wantJSON, recorder.Body.String())
			f.requireRejections(t, test.wantRejections)
		})
	}
}

func TestIssueDeviceTicketUsecaseErrors(t *testing.T) {
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
			err:        issueticket.ErrRequestInvalid,
			wantStatus: nethttp.StatusBadRequest,
			wantJSON:   `{"error":"invalid_request","error_description":"invalid client request"}`,
			wantRejections: map[rejection.Reason]uint64{
				rejection.RequestBody: 1,
			},
		},
		{
			name:           "busy",
			err:            issueticket.ErrBusy,
			wantStatus:     nethttp.StatusServiceUnavailable,
			wantJSON:       `{"error":"busy","error_description":"retry again"}`,
			wantRetryAfter: retryAfterBusySeconds,
		},
		{
			name:       "device not found",
			err:        issueticket.ErrDeviceNotFound,
			wantStatus: nethttp.StatusNotFound,
			wantJSON:   `{"error":"device_not_found","error_description":"device not found"}`,
		},
		{
			name:       "device inactive",
			err:        issueticket.ErrDeviceInactive,
			wantStatus: nethttp.StatusForbidden,
			wantJSON:   `{"error":"device_inactive","error_description":"inactive device"}`,
		},
		{
			name:       "unknown error does not leak its text",
			err:        errors.New("driver: connection reset"),
			wantStatus: nethttp.StatusInternalServerError,
			wantJSON:   `{"error":"internal_error","error_description":"internal error"}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newIssueDeviceTicketFixture(t, &jwt.Claims{UserID: "42"})
			f.expectIssueTicket(nil, test.err)

			recorder := f.post(validIssueDeviceTicketBody)

			require.Equal(t, test.wantStatus, recorder.Code)
			require.JSONEq(t, test.wantJSON, recorder.Body.String())
			require.Equal(t, test.wantRetryAfter, recorder.Header().Get("Retry-After"))
			f.requireRejections(t, test.wantRejections)
		})
	}
}

// TestIssueDeviceTicketUsecaseDeadline checks that the use case gets a context
// with the deadline of the handler. The handler must cancel the context after
// the use case returns.
func TestIssueDeviceTicketUsecaseDeadline(t *testing.T) {
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
			f := newIssueDeviceTicketFixtureWithTimeout(t, test.timeout, &jwt.Claims{UserID: "42"})

			var usecaseCtx context.Context
			f.usecase.EXPECT().
				IssueTicket(gomock.Any(), user.ID(42), issueticket.Request{DeviceID: "1234"}).
				DoAndReturn(func(ctx context.Context, _ user.ID, _ issueticket.Request) (*issueticket.Response, error) {
					usecaseCtx = ctx
					deadline, hasDeadline := ctx.Deadline()
					require.Equal(t, test.wantDeadline, hasDeadline)
					if test.wantDeadline {
						remaining := time.Until(deadline)
						require.Positive(t, remaining)
						require.LessOrEqual(t, remaining, test.timeout)
					}
					return nil, issueticket.ErrBusy
				})

			recorder := f.post(validIssueDeviceTicketBody)

			require.Equal(t, nethttp.StatusServiceUnavailable, recorder.Code)
			require.NotNil(t, usecaseCtx)
			if test.wantDeadline {
				require.ErrorIs(t, usecaseCtx.Err(), context.Canceled, "the handler must cancel the context")
			}
		})
	}
}

func TestIssueDeviceTicketNilResponse(t *testing.T) {
	f := newIssueDeviceTicketFixture(t, &jwt.Claims{UserID: "42"})
	f.expectIssueTicket(nil, nil)

	recorder := f.post(validIssueDeviceTicketBody)

	require.Equal(t, nethttp.StatusInternalServerError, recorder.Code)
	require.JSONEq(
		t,
		`{"error":"internal_error","error_description":"internal error"}`,
		recorder.Body.String(),
	)
	require.Contains(t, f.log(t), "issue device ticket returned no response")
}

func TestIssueDeviceTicketResponseWriteFailure(t *testing.T) {
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
			wantNoLog: "failed to respond to issue device ticket",
		},
		{
			name:      "write failed for another reason",
			err:       errors.New("disk full"),
			wantLog:   "failed to respond to issue device ticket",
			wantNoLog: "client left",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			resp := newTestTicketResponse()
			ticketBytes := resp.Ticket
			f := newIssueDeviceTicketFixture(t, &jwt.Claims{UserID: "42"}, failWrites(test.err))
			f.expectIssueTicket(resp, nil)

			f.post(validIssueDeviceTicketBody)

			f.requireRejections(t, test.wantRejections)
			logged := f.log(t)
			if test.wantLog != "" {
				require.Contains(t, logged, test.wantLog)
				require.Contains(t, logged, `"user_id":42`)
			}
			require.NotContains(t, logged, test.wantNoLog)

			// The handler must clear the ticket when the write fails.
			require.Equal(t, []byte{0, 0, 0}, ticketBytes)
		})
	}
}
