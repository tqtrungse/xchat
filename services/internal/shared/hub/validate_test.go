/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package hub

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The helpers validDescriptor and newHubPublicKey are in parse_test.go.

func TestDescriptorValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(descriptor *Descriptor)
		wantErr bool
	}{
		{
			name:    "valid",
			change:  func(*Descriptor) {},
			wantErr: false,
		},
		{
			name:    "address without port",
			change:  func(descriptor *Descriptor) { descriptor.Address = "hub.example" },
			wantErr: true,
		},
		{
			name:    "address without host",
			change:  func(descriptor *Descriptor) { descriptor.Address = ":7443" },
			wantErr: true,
		},
		{
			name:    "port zero",
			change:  func(descriptor *Descriptor) { descriptor.Address = "hub.example:0" },
			wantErr: true,
		},
		{
			name:    "port too large",
			change:  func(descriptor *Descriptor) { descriptor.Address = "hub.example:65536" },
			wantErr: true,
		},
		{
			name:    "port not a number",
			change:  func(descriptor *Descriptor) { descriptor.Address = "hub.example:https" },
			wantErr: true,
		},
		{
			name:    "IPv6 address",
			change:  func(descriptor *Descriptor) { descriptor.Address = "[2001:db8::1]:7443" },
			wantErr: false,
		},
		{
			name:    "IPv4 address",
			change:  func(descriptor *Descriptor) { descriptor.Address = "192.0.2.10:9000" },
			wantErr: false,
		},
		{
			name:    "hostname with trailing dot",
			change:  func(descriptor *Descriptor) { descriptor.Address = "hub.example.:9000" },
			wantErr: false,
		},
		{
			name:    "IPv6 address without brackets",
			change:  func(descriptor *Descriptor) { descriptor.Address = "2001:db8::1:9000" },
			wantErr: true,
		},
		{
			name:    "empty address",
			change:  func(descriptor *Descriptor) { descriptor.Address = "" },
			wantErr: true,
		},
		{
			name: "address too long",
			change: func(descriptor *Descriptor) {
				descriptor.Address = strings.Repeat("a.", MaxAddressBytes/2) + "example:9000"
			},
			wantErr: true,
		},
		{
			name:    "hostname label starts with hyphen",
			change:  func(descriptor *Descriptor) { descriptor.Address = "-bad.example:9000" },
			wantErr: true,
		},
		{
			name:    "hostname label ends with hyphen",
			change:  func(descriptor *Descriptor) { descriptor.Address = "bad-.example:9000" },
			wantErr: true,
		},
		{
			name:    "hostname with underscore",
			change:  func(descriptor *Descriptor) { descriptor.Address = "hub_1.example:9000" },
			wantErr: true,
		},
		{
			name:    "empty hostname label",
			change:  func(descriptor *Descriptor) { descriptor.Address = "hub..example:9000" },
			wantErr: true,
		},
		{
			name: "hostname label of 63 bytes",
			change: func(descriptor *Descriptor) {
				descriptor.Address = strings.Repeat("a", 63) + ".example:9000"
			},
			wantErr: false,
		},
		{
			name: "hostname label of 64 bytes",
			change: func(descriptor *Descriptor) {
				descriptor.Address = strings.Repeat("a", 64) + ".example:9000"
			},
			wantErr: true,
		},
		{
			name:    "wrong protocol",
			change:  func(descriptor *Descriptor) { descriptor.Protocol = "Noise_XX_25519_ChaChaPoly_SHA256" },
			wantErr: true,
		},
		{
			name:    "empty protocol",
			change:  func(descriptor *Descriptor) { descriptor.Protocol = "" },
			wantErr: true,
		},
		{
			name:    "empty audience",
			change:  func(descriptor *Descriptor) { descriptor.Audience = "" },
			wantErr: true,
		},
		{
			name:    "zero key ID",
			change:  func(descriptor *Descriptor) { descriptor.KeyID = 0 },
			wantErr: true,
		},
		{
			name:    "short public key",
			change:  func(descriptor *Descriptor) { descriptor.PublicKey = descriptor.PublicKey[:31] },
			wantErr: true,
		},
		{
			name: "low-order public key",
			change: func(descriptor *Descriptor) {
				descriptor.PublicKey = make([]byte, 32)
			},
			wantErr: true,
		},
		{
			name:    "wrong version",
			change:  func(descriptor *Descriptor) { descriptor.Version = WireVersion + 1 },
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			descriptor := validDescriptor(t)
			tc.change(&descriptor)

			err := descriptor.Validate()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func validDescriptor(t *testing.T) Descriptor {
	t.Helper()

	return Descriptor{
		Address:   "hub.example:7443",
		Audience:  "xchat-hub",
		KeyID:     7,
		PublicKey: newHubPublicKey(t),
		Protocol:  Protocol,
		Version:   WireVersion,
	}
}
