/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package registerdevice

import "errors"

var (
	ErrRequestInvalid      = errors.New("invalid client request")
	ErrUserLocked          = errors.New("user is locked")
	ErrUserReachMaxDevices = errors.New("user reach max devices")
	ErrUserExists          = errors.New("user is already registered or username/phone is already in use")
	ErrUserNotFound        = errors.New("user not found")
	ErrDeviceExists        = errors.New("device is already registered or peer-pub/identity-pub is already in use")
	ErrInternal            = errors.New("internal error")
	ErrBusy                = errors.New("retry again")
)
