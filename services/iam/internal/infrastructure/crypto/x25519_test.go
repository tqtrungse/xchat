/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package crypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"slices"
	"testing"

	"xchat/services/internal/shared/crypto"

	"github.com/stretchr/testify/require"
)

// littleEndian gives the 32-byte little-endian encoding of v.
func littleEndian(v *big.Int) []byte {
	encoded := v.FillBytes(make([]byte, crypto.X25519PublicKeySize))
	slices.Reverse(encoded)
	return encoded
}

// mustDecodeHex decodes a little-endian u-coordinate in hex.
func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	require.NoError(t, err)
	require.Len(t, decoded, crypto.X25519PublicKeySize)
	return decoded
}

// x25519TestPoints gives the canonical points that the tests use. Each value is
// a 32-byte u-coordinate.
type x25519TestPoints struct {
	valid     []byte
	arbitrary []byte
	// lowOrder holds each canonical u-coordinate of a point of order 1, 2, 4,
	// or 8 on the curve or on its twist.
	lowOrder map[string][]byte
}

func newX25519TestPoints(t *testing.T) x25519TestPoints {
	t.Helper()

	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)

	// Any 32-byte string is a valid X25519 u-coordinate. Only the low-order
	// points fail, because their shared secret is all zeros.
	arbitrary := make([]byte, crypto.X25519PublicKeySize)
	for idx := range arbitrary {
		arbitrary[idx] = byte(idx + 2)
	}

	prime := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	primeMinusOne := new(big.Int).Sub(prime, big.NewInt(1))

	return x25519TestPoints{
		valid:     privateKey.PublicKey().Bytes(),
		arbitrary: arbitrary,
		lowOrder: map[string][]byte{
			"zero":      make([]byte, crypto.X25519PublicKeySize),
			"one":       littleEndian(big.NewInt(1)),
			"minus one": littleEndian(primeMinusOne),
			// The two points of order 8. libsodium rejects the same values.
			"order 8 (first)":  mustDecodeHex(t, "e0eb7a7c3b41b8ae1656e3faf19fc46ada098deb9c32b1fd866205165f49b800"),
			"order 8 (second)": mustDecodeHex(t, "5f9c95bca3508c24b1d0b1559c83ef5b04445cc4581c8e86d8224eddd09f1157"),
		},
	}
}

func TestX25519ValidatePeerPub(t *testing.T) {
	t.Parallel()

	points := newX25519TestPoints(t)

	// A valid key with the top bit set is a second encoding of the same key.
	highBitPeerPub := slices.Clone(points.valid)
	highBitPeerPub[crypto.X25519PublicKeySize-1] |= 0x80

	// p, p+1, and 2^255-1 are above the modulus. Each one is a non-canonical
	// encoding of 0, 1, and 2^255-1-p.
	prime := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	primePlusOne := new(big.Int).Add(prime, big.NewInt(1))
	maxEncodable := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(1))

	type testCase struct {
		name    string
		pub     []byte
		wantErr error
	}
	testCases := []testCase{
		{
			name: "valid",
			pub:  points.valid,
		},
		{
			name: "arbitrary coordinate",
			pub:  points.arbitrary,
		},
		{
			name:    "nil",
			pub:     nil,
			wantErr: ErrX25519InvalidPeerPub,
		},
		{
			name:    "short",
			pub:     make([]byte, crypto.X25519PublicKeySize-1),
			wantErr: ErrX25519InvalidPeerPub,
		},
		{
			name:    "long",
			pub:     make([]byte, crypto.X25519PublicKeySize+1),
			wantErr: ErrX25519InvalidPeerPub,
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
	}
	for name, pub := range points.lowOrder {
		testCases = append(testCases, testCase{
			name:    "low order " + name,
			pub:     pub,
			wantErr: ErrX25519InvalidDerive,
		})
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			validator := NewX25519()
			err := validator.ValidatePeerPub(test.pub)
			if test.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, test.wantErr)
		})
	}
}

// TestX25519FixedScalarMatchesRandomScalars checks the reason for
// testPrivateKey. For each point, an ECDH with testPrivateKey and an ECDH with a
// random scalar give an error in the same cases. Thus the fixed scalar
// finds each low-order point that a random scalar finds.
func TestX25519FixedScalarMatchesRandomScalars(t *testing.T) {
	t.Parallel()

	require.NotNil(t, testPrivateKey)

	points := newX25519TestPoints(t)
	pubs := map[string][]byte{
		"valid":     points.valid,
		"arbitrary": points.arbitrary,
	}
	for name, pub := range points.lowOrder {
		pubs["low order "+name] = pub
	}

	const randomScalars = 16
	for name, pub := range pubs {
		peerPublicKey, err := ecdh.X25519().NewPublicKey(pub)
		require.NoError(t, err, name)

		_, err = testPrivateKey.ECDH(peerPublicKey)
		fixedFails := err != nil

		for range randomScalars {
			privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
			require.NoError(t, err, name)

			_, err = privateKey.ECDH(peerPublicKey)
			require.Equal(t, fixedFails, err != nil, name)
		}
	}
}
