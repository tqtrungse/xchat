/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package bootstrap

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"xchat/services/internal/shared/hub"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// prettyDescriptor gives a descriptor in the layout of the protocol vectors:
// one key on each line. extra goes before the closing brace.
func prettyDescriptor(t *testing.T, extra string) string {
	t.Helper()

	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)

	return fmt.Sprintf(`{
		  "address": "hub.example.com:9000",
		  "audience": "hub.chat",
		  "key_id": 7,
		  "protocol": "Noise_NK_25519_ChaChaPoly_SHA256",
		  "public_key": %q,
		  "version": %d%s
		}
	`,
		base64.StdEncoding.EncodeToString(privateKey.PublicKey().Bytes()), hub.WireVersion, extra)
}

func TestRejectDuplicateKeys(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "descriptor without duplicate",
			data:    prettyDescriptor(t, ""),
			wantErr: false,
		},
		{
			name:    "duplicate on another line",
			data:    prettyDescriptor(t, ",\n  \"key_id\": 9"),
			wantErr: true,
		},
		{
			name:    "duplicate on the same line",
			data:    `{"key_id":7,"version":2,"key_id":9}`,
			wantErr: true,
		},
		{
			name:    "duplicate with an escaped name",
			data:    `{"key_id":7,"key_id":9}`,
			wantErr: true,
		},
		{
			name:    "duplicate after a nested object",
			data:    `{"a":{"b":1},"a":2}`,
			wantErr: true,
		},
		{
			name:    "duplicate after a nested array",
			data:    `{"a":[1,{"a":1}],"a":2}`,
			wantErr: true,
		},
		{
			name:    "duplicate in a nested object",
			data:    `{"a":{"b":1,"b":2}}`,
			wantErr: true,
		},
		{
			name:    "same key in two objects",
			data:    `{"a":{"x":1},"b":{"x":2}}`,
			wantErr: false,
		},
		{
			name:    "same key in objects of an array",
			data:    `[{"x":1},{"x":2}]`,
			wantErr: false,
		},
		{
			name:    "value equal to a later key",
			data:    `{"a":"b","b":1}`,
			wantErr: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := rejectDuplicateKeys([]byte(tc.data))
			if tc.wantErr {
				require.ErrorIs(t, err, errDuplicateKey)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRejectDuplicateKeysHidesKey(t *testing.T) {
	err := rejectDuplicateKeys([]byte(`{"SECRET":1,"SECRET":2}`))
	require.ErrorIs(t, err, errDuplicateKey)
	require.NotContains(t, err.Error(), "SECRET")
}

func TestLoadHubEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    func(t *testing.T) string
		wantErr error
	}{
		{
			name:    "valid descriptor",
			data:    func(t *testing.T) string { return prettyDescriptor(t, "") },
			wantErr: nil,
		},
		{
			name:    "duplicate key",
			data:    func(t *testing.T) string { return prettyDescriptor(t, ",\n  \"key_id\": 9") },
			wantErr: errDuplicateKey,
		},
		{
			name:    "unknown field",
			data:    func(t *testing.T) string { return prettyDescriptor(t, ",\n  \"private_key\": \"x\"") },
			wantErr: hub.ErrUnknownField,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hub.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.data(t)), 0o600))

			loader := viper.New()
			loader.Set("HUB_NOISE_DESCRIPTOR_FILE", path)

			descriptor, err := loadHubEndpoint(loader)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Nil(t, descriptor)
				return
			}
			require.NoError(t, err)
			require.Equal(t, uint32(7), descriptor.KeyID)
			require.Equal(t, "Noise_NK_25519_ChaChaPoly_SHA256", descriptor.Protocol)
		})
	}

	t.Run("missing path", func(t *testing.T) {
		descriptor, err := loadHubEndpoint(viper.New())
		require.Error(t, err)
		require.Nil(t, descriptor)
	})
}
