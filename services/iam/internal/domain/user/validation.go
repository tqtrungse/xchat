/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package user

const (
	minUsernameLength  = 3
	maxUsernameLength  = 64
	maxPhoneHashLength = 16
)

// IsValidUsername reports whether username has the canonical form. The form has
// minUsernameLength to maxUsernameLength characters from a-z, 0-9, '.', '_',
// and '-'.
//
// The form has no upper case letter, so each name has one spelling. The users
// table compares the bytes, and the register use case does too. The form has
// only ASCII, so it has no homoglyph, no control character, and no invalid
// UTF-8. Each byte of a non-ASCII character in UTF-8 is 0x80 or more, so the
// check rejects it.
func IsValidUsername(username string) bool {
	if len(username) < minUsernameLength || len(username) > maxUsernameLength {
		return false
	}
	for idx := 0; idx < len(username); idx++ {
		character := username[idx]
		switch {
		case 'a' <= character && character <= 'z',
			'0' <= character && character <= '9',
			character == '.',
			character == '_',
			character == '-':
		default:
			return false
		}
	}
	return true
}

// IsValidPhoneHash reports whether phoneHash has exactly maxPhoneHashLength
// bytes. The client sends HMAC-SHA256 truncated to 128 bits.
func IsValidPhoneHash(phoneHash []byte) bool {
	return len(phoneHash) == maxPhoneHashLength
}
