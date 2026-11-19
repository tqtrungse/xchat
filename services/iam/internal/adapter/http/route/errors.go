/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package route

import "errors"

var (
	errUsecaseRequired    = errors.New("use case is required")
	errLoggerRequired     = errors.New("logger is required")
	errRejectionsRequired = errors.New("rejection stats are required")
)
