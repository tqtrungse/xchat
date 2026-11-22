/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package hub

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// secretMarker is a text that each bad input contains. An error must not
// contain it, because a wrong file can contain private data.
const secretMarker = "SECRET"

func newHubPublicKey(t *testing.T) []byte {
	t.Helper()

	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	return privateKey.PublicKey().Bytes()
}

// descriptorJSON gives the JSON of descriptor. extra goes before the closing
// brace, so a test can add a member.
func descriptorJSON(descriptor Descriptor, extra string) []byte {
	return fmt.Appendf(
		nil,
		`{"address":%q,"audience":%q,"key_id":%d,"public_key":%q,"protocol":%q,"version":%d%s}`,
		descriptor.Address,
		descriptor.Audience,
		descriptor.KeyID,
		base64.StdEncoding.EncodeToString(descriptor.PublicKey),
		descriptor.Protocol,
		descriptor.Version,
		extra,
	)
}

func TestParseValid(t *testing.T) {
	want := validDescriptor(t)

	got, err := Parse(descriptorJSON(want, ""))
	require.NoError(t, err)
	require.Equal(t, &want, got)
}

func TestParseRejectsLargeData(t *testing.T) {
	data := make([]byte, MaxDescriptorBytes+1)

	got, err := Parse(data)
	require.Error(t, err)
	require.Nil(t, got)
}

// TestParseLastDuplicateWins records the behavior for a duplicate field. The
// decoder does not reject it. The last value replaces the first value.
func TestParseLastDuplicateWins(t *testing.T) {
	descriptor := validDescriptor(t)

	got, err := Parse(descriptorJSON(descriptor, `,"key_id":9`))
	require.NoError(t, err)
	require.Equal(t, uint32(9), got.KeyID)
}

func TestParseHidesInput(t *testing.T) {
	descriptor := validDescriptor(t)
	valid := string(descriptorJSON(descriptor, ""))

	for _, tc := range []struct {
		name    string
		data    string
		wantErr error
	}{
		{
			name:    "unknown field",
			data:    string(descriptorJSON(descriptor, `,"private_key":"SECRET-KEY"`)),
			wantErr: ErrUnknownField,
		},
		{
			name: "unknown field with secret name",
			data: string(descriptorJSON(descriptor, `,"SECRET":1`)),
			// The name of the field must not go into the error either.
			wantErr: ErrUnknownField,
		},
		{
			name: "syntax error",
			data: `{"address":"SECRET`,
		},
		{
			name: "wrong type",
			data: strings.Replace(valid, `"key_id":7`, `"key_id":"SECRET"`, 1),
		},
		{
			name: "number overflow",
			data: strings.Replace(valid, `"key_id":7`, `"key_id":99999999999999999999`, 1),
		},
		{
			name: "invalid Base64",
			data: strings.Replace(
				valid,
				base64.StdEncoding.EncodeToString(descriptor.PublicKey),
				"!!SECRET!!",
				1,
			),
		},
		{
			name: "trailing data",
			data: valid + ` "SECRET"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse([]byte(tc.data))
			require.Error(t, err)
			require.Nil(t, got)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			}
			require.NotContains(t, err.Error(), secretMarker)
			require.NotContains(t, err.Error(), "private_key")
			require.NotContains(t, err.Error(), "99999")
		})
	}
}

func TestParseValidates(t *testing.T) {
	descriptor := validDescriptor(t)
	descriptor.KeyID = 0

	got, err := Parse(descriptorJSON(descriptor, ""))
	require.ErrorContains(t, err, "validate hub descriptor")
	require.Nil(t, got)
}
