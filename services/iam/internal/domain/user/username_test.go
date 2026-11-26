/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package user

import (
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
			username: strings.Repeat("a", MinUsernameLength),
			want:     true,
		},
		{
			name:     "maximum length",
			username: strings.Repeat("a", MaxUsernameLength),
			want:     true,
		},
		{
			name:     "empty",
			username: "",
			want:     false,
		},
		{
			name:     "too short",
			username: strings.Repeat("a", MinUsernameLength-1),
			want:     false,
		},
		{
			name:     "too long",
			username: strings.Repeat("a", MaxUsernameLength+1),
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
