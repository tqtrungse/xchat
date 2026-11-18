/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package crypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"math/big"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// littleEndian gives the 32-byte little-endian encoding of v.
func littleEndian(v *big.Int) []byte {
	encoded := v.FillBytes(make([]byte, x25519PublicKeySize))
	slices.Reverse(encoded)
	return encoded
}

func TestX25519ValidatePeerPub(t *testing.T) {
	t.Parallel()

	privateKey, validationErr := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, validationErr)
	validPeerPub := privateKey.PublicKey().Bytes()

	// A valid key with the top bit set is a second encoding of the same key.
	highBitPeerPub := slices.Clone(validPeerPub)
	highBitPeerPub[x25519PublicKeySize-1] |= 0x80

	lowOrderZero := make([]byte, x25519PublicKeySize)
	lowOrderOne := make([]byte, x25519PublicKeySize)
	lowOrderOne[0] = 1

	// p, p+1, and 2^255-1 are above the modulus. Each one is a non-canonical
	// encoding of 0, 1, and 2^255-1-p.
	prime := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	primeMinusOne := new(big.Int).Sub(prime, big.NewInt(1))
	primePlusOne := new(big.Int).Add(prime, big.NewInt(1))
	maxEncodable := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(1))

	// Any 32-byte string is a valid X25519 u-coordinate. Only the low-order
	// points fail, because their shared secret is all zeros.
	arbitraryPeerPub := make([]byte, x25519PublicKeySize)
	for idx := range arbitraryPeerPub {
		arbitraryPeerPub[idx] = byte(idx + 2)
	}

	for _, test := range []struct {
		name    string
		pub     []byte
		wantErr error
	}{
		{
			name: "valid",
			pub:  validPeerPub,
		},
		{
			name: "arbitrary coordinate",
			pub:  arbitraryPeerPub,
		},
		{
			name:    "nil",
			pub:     nil,
			wantErr: ErrX25519InvalidPeerPub,
		},
		{
			name:    "short",
			pub:     make([]byte, x25519PublicKeySize-1),
			wantErr: ErrX25519InvalidPeerPub,
		},
		{
			name:    "long",
			pub:     make([]byte, x25519PublicKeySize+1),
			wantErr: ErrX25519InvalidPeerPub,
		},
		{
			name:    "low order zero",
			pub:     lowOrderZero,
			wantErr: ErrX25519InvalidDerive,
		},
		{
			name:    "low order one",
			pub:     lowOrderOne,
			wantErr: ErrX25519InvalidDerive,
		},
		{
			name:    "low order minus one is canonical",
			pub:     littleEndian(primeMinusOne),
			wantErr: ErrX25519InvalidDerive,
		},
		{
			name:    "top bit set on valid key",
			pub:     highBitPeerPub,
			wantErr: ErrX25519NonCanonicalPeerPub,
		},
		{
			name:    "prime encodes zero",
			pub:     littleEndian(prime),
			wantErr: ErrX25519NonCanonicalPeerPub,
		},
		{
			name:    "prime plus one encodes one",
			pub:     littleEndian(primePlusOne),
			wantErr: ErrX25519NonCanonicalPeerPub,
		},
		{
			name:    "largest 255-bit value",
			pub:     littleEndian(maxEncodable),
			wantErr: ErrX25519NonCanonicalPeerPub,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			validator := NewX25519()
			validationErr = validator.ValidatePeerPub(test.pub)
			if test.wantErr == nil {
				require.NoError(t, validationErr)
				return
			}
			require.ErrorIs(t, validationErr, test.wantErr)
		})
	}
}
