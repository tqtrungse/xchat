/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package password

import "errors"

var (
	// ErrMismatch means the password does not match the stored hash.
	ErrMismatch = errors.New("password does not match")

	// ErrBusy means the hasher has no free capacity before the context ends.
	// The caller can try again later.
	ErrBusy = errors.New("password hasher is busy")
)
