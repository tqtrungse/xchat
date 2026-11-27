/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package device

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxNameLength = 128

// IsValidName reports whether name has 1 to maxNameLength bytes of valid UTF-8,
// has at least one non-space character, and has no control or format character.
//
// The client sends the model and the OS name, for example "iPhone 15 iOS 18".
// Thus the name can have upper case letters, spaces, and letters of each
// language. The client shows the name. A bidi or zero-width character can
// change how the name looks, so the check rejects each format character. MySQL
// rejects invalid UTF-8 in a utf8mb4 column, so the check rejects it first.
func IsValidName(name string) bool {
	if len(name) == 0 || len(name) > maxNameLength || strings.TrimSpace(name) == "" || !utf8.ValidString(name) {
		return false
	}
	for _, character := range name {
		if unicode.IsControl(character) || unicode.Is(unicode.Cf, character) {
			return false
		}
	}
	return true
}
