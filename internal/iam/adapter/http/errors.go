/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package http

import "errors"

var (
	errAuthHeaderNotFound = errors.New("missing or invalid Authorization header")
	errAuthUnavailable    = errors.New("token verification is unavailable")
	errRejectionsRequired = errors.New("rejection stats are required")
	errLoggerRequired     = errors.New("logger is required")
	errRouteNil           = errors.New("route is nil")
)
