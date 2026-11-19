/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package user

import "errors"

var (
	// ErrAlreadyExists means that a user with the same ID exists.
	ErrAlreadyExists = errors.New("user already exists")

	// ErrUniqueConflict means that another user has the same username or
	// phone hash.
	ErrUniqueConflict = errors.New("user unique value conflicts with another user")
)
