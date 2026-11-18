/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package crypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"fmt"
)

const x25519PublicKeySize = 32

// x25519Prime is the field modulus p = 2^255 - 19, in little-endian byte order.
var x25519Prime = [x25519PublicKeySize]byte{
	0xed, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f,
}

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
// The check runs a test ECDH with a new private key. A low-order point gives an
// all-zero shared secret, and ECDH reports that as an error. The private key
// does not change the result: RFC 7748 clamps the scalar to a multiple of 8,
// and a low-order point gives an all-zero secret for every such scalar. The
// private key is not stored, so each call is independent of the others.
func (x *X25519) ValidatePeerPub(publicKey []byte) error {
	if len(publicKey) != x25519PublicKeySize {
		return ErrX25519InvalidPeerPub
	}
	if !isCanonical(publicKey) {
		return ErrX25519NonCanonicalPeerPub
	}

	peerPublicKey, err := ecdh.X25519().NewPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrX25519InvalidPeerPub, err)
	}

	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate X25519 test key: %w", err)
	}

	_, err = privateKey.ECDH(peerPublicKey)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrX25519InvalidDerive, err)
	}

	return nil
}

// isCanonical reports whether u is the canonical encoding of a field element.
// Decoding masks the top bit of the last byte, so a key with that bit set is a
// second encoding of a key with the bit clear. A value of p or more is another
// encoding too: it stands for the field element that equals the value minus p.
// isCanonical rejects both. len(u) must be x25519PublicKeySize.
func isCanonical(publicKey []byte) bool {
	if publicKey[x25519PublicKeySize-1]&0x80 != 0 {
		return false
	}
	// Compare from the most significant byte. The first byte that differs
	// decides the order, and u equal to p is not canonical.
	for i := x25519PublicKeySize - 1; i >= 0; i-- {
		if publicKey[i] != x25519Prime[i] {
			return publicKey[i] < x25519Prime[i]
		}
	}
	return false
}
