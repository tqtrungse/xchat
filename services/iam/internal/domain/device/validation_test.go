/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package device

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValidName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		deviceName string
		want       bool
	}{
		{
			// The client sends the model and the OS name.
			name:       "model and OS name",
			deviceName: "iPhone 15 iOS 18",
			want:       true,
		},
		{
			name:       "punctuation",
			deviceName: "Pixel 8 Pro (Android 15), user's phone",
			want:       true,
		},
		{
			name:       "Vietnamese letters",
			deviceName: "\u0110i\u1ec7n tho\u1ea1i c\u1ee7a t\u00f4i",
			want:       true,
		},
		{
			name:       "letters of a non-Latin script",
			deviceName: "\u30b9\u30de\u30fc\u30c8\u30d5\u30a9\u30f3",
			want:       true,
		},
		{
			// U+1F4F1 is outside the Basic Multilingual Plane. utf8mb4 stores it.
			name:       "emoji",
			deviceName: "Phone \U0001f4f1",
			want:       true,
		},
		{
			name:       "minimum length",
			deviceName: "d",
			want:       true,
		},
		{
			name:       "maximum length",
			deviceName: strings.Repeat("d", maxNameLength),
			want:       true,
		},
		{
			// The limit counts bytes, not characters. Each U+00E9 has 2 bytes in
			// UTF-8, so this name has 64 characters and 128 bytes.
			name:       "maximum length in bytes of multibyte characters",
			deviceName: strings.Repeat("\u00e9", maxNameLength/2),
			want:       true,
		},
		{
			name:       "empty",
			deviceName: "",
			want:       false,
		},
		{
			name:       "spaces only",
			deviceName: "   ",
			want:       false,
		},
		{
			// U+00A0 is a space, but not a control or a format character.
			name:       "no-break space only",
			deviceName: "\u00a0",
			want:       false,
		},
		{
			// The check rejects a blank name. It does not trim a valid name.
			name:       "leading and trailing spaces",
			deviceName: " iPhone 15 ",
			want:       true,
		},
		{
			name:       "too long",
			deviceName: strings.Repeat("d", maxNameLength+1),
			want:       false,
		},
		{
			name:       "one byte over the limit with multibyte characters",
			deviceName: strings.Repeat("\u00e9", maxNameLength/2) + "d",
			want:       false,
		},
		{
			// MySQL rejects this value in a utf8mb4 column with error 1366.
			name:       "invalid UTF-8",
			deviceName: "Phone \xff",
			want:       false,
		},
		{
			name:       "truncated multibyte character",
			deviceName: "Phone \xc3",
			want:       false,
		},
		{
			// U+202E reverses the direction of the text that follows it.
			name:       "bidi override",
			deviceName: "Phone \u202e",
			want:       false,
		},
		{
			name:       "zero-width space",
			deviceName: "Ph\u200bone",
			want:       false,
		},
		{
			name:       "byte order mark",
			deviceName: "\ufeffPhone",
			want:       false,
		},
		{
			name:       "new line",
			deviceName: "Phone\n",
			want:       false,
		},
		{
			name:       "tab",
			deviceName: "Phone\t15",
			want:       false,
		},
		{
			name:       "NUL byte",
			deviceName: "Phone\x00",
			want:       false,
		},
		{
			// U+0085 is a C1 control character.
			name:       "C1 control character",
			deviceName: "Phone\u0085",
			want:       false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.want, IsValidName(test.deviceName))
		})
	}
}
