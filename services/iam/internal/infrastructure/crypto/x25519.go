/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package crypto

import (
	"bytes"
	"crypto/ecdh"
	"fmt"

	"xchat/services/internal/shared/crypto"
)

// x25519Prime is the field modulus p = 2^255 - 19, in little-endian byte order.
var x25519Prime = [crypto.X25519PublicKeySize]byte{
	0xed, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f,
}

// testPrivateKey is the scalar of the test ECDH. Each clamped scalar gives
// the same result, so a fixed scalar is sufficient. It is not a secret:
// ValidatePeerPub discards the shared secret.
var testPrivateKey, _ = ecdh.X25519().NewPrivateKey(bytes.Repeat([]byte{0x42}, 32))

type X25519 struct{}

func NewX25519() X25519 {
	return X25519{}
}

// ValidatePeerPub checks that publicKey is a usable X25519 public key. Each
// failure wraps one of the package errors, so a caller can use errors.Is.
//
// The key must be in the canonical encoding. Two encodings of one key would be
// two different byte strings for the same identity, so the check rejects the
// others.
//
// The check runs a test ECDH with testPrivateKey. A low-order point gives an
// all-zero shared secret, and ECDH reports that as an error. The scalar does
// not change the result: RFC 7748 clamps each scalar to a multiple of 8, and a
// low-order point gives an all-zero secret for every such scalar. Any other
// point gives an all-zero secret for no clamped scalar.
func (x *X25519) ValidatePeerPub(publicKey []byte) error {
	if len(publicKey) != crypto.X25519PublicKeySize {
		return ErrX25519InvalidPeerPub
	}
	if !isCanonical(publicKey) {
		return ErrX25519NonCanonicalPeerPub
	}

	peerPublicKey, err := ecdh.X25519().NewPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrX25519InvalidPeerPub, err)
	}

	_, err = testPrivateKey.ECDH(peerPublicKey)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrX25519InvalidDerive, err)
	}

	return nil
}

// isCanonical reports whether u is the canonical encoding of a field element.
// Decoding masks the top bit of the last byte, so a key with that bit set is a
// second encoding of a key with the bit clear. A value of p or more is another
// encoding too: it stands for the field element that equals the value minus p.
// isCanonical rejects both. len(u) must be crypto.X25519PublicKeySize.
func isCanonical(publicKey []byte) bool {
	if publicKey[crypto.X25519PublicKeySize-1]&0x80 != 0 {
		return false
	}
	// Compare from the most significant byte. The first byte that differs
	// decides the order, and u equal to p is not canonical.
	for i := crypto.X25519PublicKeySize - 1; i >= 0; i-- {
		if publicKey[i] != x25519Prime[i] {
			return publicKey[i] < x25519Prime[i]
		}
	}
	return false
}
