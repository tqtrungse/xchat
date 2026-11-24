/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package issueticket

import "errors"

// The use case returns these errors to the HTTP adapter.
var (
	ErrRequestInvalid = errors.New("invalid client request")
	ErrDeviceNotFound = errors.New("device not found")
	ErrDeviceInactive = errors.New("inactive device")
	ErrInternal       = errors.New("internal error")
	ErrBusy           = errors.New("retry again")
)
