/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package httpx

import "errors"

// The descriptions of these errors go to the client. They must be constants
// that never contain a value of the request.
var (
	ErrAuthVerify   = errors.New("failed to verify token")
	ErrBodyInvalid  = errors.New("invalid request body")
	ErrBodyTooLarge = errors.New("request body is too large")
	ErrEncode       = errors.New("failed to encode response")
	ErrInternal     = errors.New("internal error")
)
