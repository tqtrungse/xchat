/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package ticket

import (
	"bytes"
	"crypto/ed25519"

	"filippo.io/edwards25519"
)

var identityPoint = edwards25519.NewIdentityPoint()

// ValidateIdentity checks the Ed25519 public-key size and canonical point encoding,
// rejecting malformed and small-order points. It does not verify a signature.
func ValidateIdentity(pub []byte) error {
	if len(pub) != ed25519.PublicKeySize {
		return ErrIdentityPubInvalid
	}

	var p edwards25519.Point
	_, err := p.SetBytes(pub)
	if err != nil || !bytes.Equal(p.Bytes(), pub) {
		return ErrIdentityPubInvalid
	}

	p.MultByCofactor(&p)
	if p.Equal(identityPoint) == 1 {
		return ErrIdentityPubInvalid
	}
	return nil
}
