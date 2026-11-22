/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package hub

import "errors"

// These errors do not contain a name or a value of the descriptor, because a
// wrong file can contain private data.
var (
	// ErrUnknownField means that the descriptor has a field that Descriptor
	// does not have. An example is a private key.
	ErrUnknownField = errors.New("hub descriptor has an unknown field")
)
