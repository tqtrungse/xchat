/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package ticket

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateIdentity(t *testing.T) {
	t.Parallel()

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	decodeHex := func(t *testing.T, value string) []byte {
		t.Helper()

		decoded, err := hex.DecodeString(value)
		require.NoError(t, err)
		return decoded
	}

	for _, test := range []struct {
		name    string
		pub     []byte
		wantErr error
	}{
		{
			name: "valid",
			pub:  publicKey,
		},
		{
			name:    "nil",
			pub:     nil,
			wantErr: ErrIdentityPubInvalid,
		},
		{
			name:    "short",
			pub:     make([]byte, ed25519.PublicKeySize-1),
			wantErr: ErrIdentityPubInvalid,
		},
		{
			name:    "long",
			pub:     make([]byte, ed25519.PublicKeySize+1),
			wantErr: ErrIdentityPubInvalid,
		},
		{
			name:    "identity point",
			pub:     decodeHex(t, "0100000000000000000000000000000000000000000000000000000000000000"),
			wantErr: ErrIdentityPubInvalid,
		},
		{
			name:    "order two point",
			pub:     decodeHex(t, "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"),
			wantErr: ErrIdentityPubInvalid,
		},
		{
			name:    "non-canonical encoding",
			pub:     decodeHex(t, "edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"),
			wantErr: ErrIdentityPubInvalid,
		},
		{
			name:    "invalid encoding",
			pub:     bytes.Repeat([]byte{0xff}, ed25519.PublicKeySize),
			wantErr: ErrIdentityPubInvalid,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			validationErr := ValidateIdentity(test.pub)
			if test.wantErr == nil {
				require.NoError(t, validationErr)
				return
			}
			require.ErrorIs(t, validationErr, test.wantErr)
		})
	}
}
