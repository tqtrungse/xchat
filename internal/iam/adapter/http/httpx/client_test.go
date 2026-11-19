/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package httpx

import (
	"context"
	"errors"
	"fmt"
	"net"
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIsClientGone(t *testing.T) {
	t.Parallel()

	timeout := &net.OpError{Op: "write", Err: os.ErrDeadlineExceeded}
	require.True(t, timeout.Timeout(), "the test error must be a timeout")

	for _, test := range []struct {
		name            string
		err             error
		requestCanceled bool
		want            bool
	}{
		{name: "broken pipe", err: syscall.EPIPE, want: true},
		{name: "connection reset", err: syscall.ECONNRESET, want: true},
		{name: "connection aborted", err: syscall.ECONNABORTED, want: true},
		{name: "closed connection", err: net.ErrClosed, want: true},
		{name: "write timeout", err: timeout, want: true},
		{
			name: "wrapped system error",
			err:  fmt.Errorf("write: %w", &net.OpError{Op: "write", Err: os.NewSyscallError("write", syscall.EPIPE)}),
			want: true,
		},
		{
			name:            "unknown error and the request ended",
			err:             errors.New("http2: stream closed"),
			requestCanceled: true,
			want:            true,
		},
		{name: "unknown error", err: errors.New("boom"), want: false},
		{name: "network error without timeout", err: &net.OpError{Op: "write", Err: errors.New("boom")}, want: false},
		{name: "nil", err: nil, want: false},
		{name: "nil and the request ended", err: nil, requestCanceled: true, want: false},
		{
			name:            "encoding failure is never the client",
			err:             fmt.Errorf("%w: %w", ErrEncode, syscall.EPIPE),
			requestCanceled: true,
			want:            false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if test.requestCanceled {
				cancel()
			}

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(nethttp.MethodPost, "/", nil).WithContext(ctx)

			require.Equal(t, test.want, IsClientGone(c, test.err))
		})
	}
}
