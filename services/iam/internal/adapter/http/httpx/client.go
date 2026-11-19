/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package httpx

import (
	"errors"
	"net"
	"syscall"

	"github.com/gin-gonic/gin"
)

// IsClientGone reports whether err, which a write of the response returned,
// comes from the client side of the connection and not from the server. The
// client closed the connection, reset it, canceled the request, or was too slow
// for the write timeout of the server.
//
// IsClientGone returns true if:
//   - the context of the request ended. The server cancels it when the client
//     closes the connection, and when an HTTP/2 client cancels the stream. The
//     HTTP/2 server reports those cases with errors that it does not export,
//     so errors.Is cannot match them.
//   - err wraps EPIPE, ECONNRESET, ECONNABORTED, or net.ErrClosed. ErrClosed
//     also covers the connections that Server.Close closes after an aborted
//     shutdown.
//   - err is a network error that timed out, for example the write timeout.
//
// IsClientGone returns false for nil and for an ErrEncode error. An encoding
// failure is a bug of the server, even if the client is gone too.
func IsClientGone(c *gin.Context, err error) bool {
	if err == nil || errors.Is(err, ErrEncode) {
		return false
	}

	if c.Request.Context().Err() != nil {
		return true
	}

	var netErr net.Error
	return errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, net.ErrClosed) ||
		(errors.As(err, &netErr) && netErr.Timeout())
}
