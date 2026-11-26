/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package register

import "errors"

// The use case returns these errors to the HTTP adapter.
var (
	ErrRequestInvalid      = errors.New("invalid client request")
	ErrUserLocked          = errors.New("user is locked")
	ErrUserReachMaxDevices = errors.New("user reach max devices")
	ErrUserExists          = errors.New("user is already registered or username/phone is already in use")
	ErrDeviceExists        = errors.New("device is already registered or peer-pub/identity-pub is already in use")
	ErrInternal            = errors.New("internal error")
	ErrBusy                = errors.New("retry again")
)

// The PasswordHasher port returns these errors to the use case. They are not
// errors of the use case. mapPasswordError converts them.
var (
	// ErrPasswordMismatch means that the password does not match the stored
	// hash.
	ErrPasswordMismatch = errors.New("password does not match")

	// ErrPasswordHasherBusy means that the hasher has no free capacity before
	// the context ends. The caller can try again later.
	ErrPasswordHasherBusy = errors.New("password hasher is busy")

	// ErrPasswordTooLong means that the password is longer than the hasher
	// accepts. It is an error of the client.
	ErrPasswordTooLong = errors.New("password is too long")
)
