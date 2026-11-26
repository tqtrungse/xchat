/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package user

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValidUsername(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		username string
		want     bool
	}{
		{
			name:     "lower case letters",
			username: "alice",
			want:     true,
		},
		{
			name:     "each allowed character",
			username: "a.b_c-d0123456789",
			want:     true,
		},
		{
			name:     "minimum length",
			username: strings.Repeat("a", minUsernameLength),
			want:     true,
		},
		{
			name:     "maximum length",
			username: strings.Repeat("a", maxUsernameLength),
			want:     true,
		},
		{
			name:     "empty",
			username: "",
			want:     false,
		},
		{
			name:     "too short",
			username: strings.Repeat("a", minUsernameLength-1),
			want:     false,
		},
		{
			name:     "too long",
			username: strings.Repeat("a", maxUsernameLength+1),
			want:     false,
		},
		{
			name:     "upper case letter",
			username: "Alice",
			want:     false,
		},
		{
			name:     "space",
			username: "al ice",
			want:     false,
		},
		{
			name:     "other ASCII symbol",
			username: "alice@home",
			want:     false,
		},
		{
			// The first letter is the Cyrillic U+0430. It looks like the Latin a.
			name:     "homoglyph",
			username: "\u0430lice",
			want:     false,
		},
		{
			name:     "letter with a diacritic",
			username: "an\u00e9",
			want:     false,
		},
		{
			// U+202E reverses the direction of the text that follows it.
			name:     "bidi control character",
			username: "alice\u202e",
			want:     false,
		},
		{
			name:     "zero-width space",
			username: "ali\u200bce",
			want:     false,
		},
		{
			name:     "NUL byte",
			username: "ali\x00ce",
			want:     false,
		},
		{
			name:     "new line",
			username: "alice\n",
			want:     false,
		},
		{
			name:     "invalid UTF-8",
			username: "ali\xffce",
			want:     false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.want, IsValidUsername(test.username))
		})
	}
}

func TestIsValidPhoneHash(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		phoneHash []byte
		want      bool
	}{
		{
			name:      "exact length",
			phoneHash: bytes.Repeat([]byte{0x01}, maxPhoneHashLength),
			want:      true,
		},
		{
			// A truncated HMAC always has the same length. A shorter value can
			// collide with the hash of another phone.
			name:      "one byte short",
			phoneHash: bytes.Repeat([]byte{0x01}, maxPhoneHashLength-1),
			want:      false,
		},
		{
			name:      "one byte",
			phoneHash: []byte{0x01},
			want:      false,
		},
		{
			name:      "nil",
			phoneHash: nil,
			want:      false,
		},
		{
			name:      "empty",
			phoneHash: []byte{},
			want:      false,
		},
		{
			name:      "one byte long",
			phoneHash: bytes.Repeat([]byte{0x01}, maxPhoneHashLength+1),
			want:      false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.want, IsValidPhoneHash(test.phoneHash))
		})
	}
}
