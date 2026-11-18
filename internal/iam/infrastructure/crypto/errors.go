/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package crypto

import "errors"

var (
	ErrX25519InvalidPeerPub      = errors.New("invalid X25519 public key length")
	ErrX25519NonCanonicalPeerPub = errors.New("non-canonical X25519 public key encoding")
	ErrX25519InvalidDerive       = errors.New("invalid X25519 public key: ECDH failed")
	ErrBcryptInvalidHash         = errors.New("invalid bcrypt hash")
	ErrBcryptPasswordTooLong     = errors.New("bcrypt password is longer than 72 bytes")
)
