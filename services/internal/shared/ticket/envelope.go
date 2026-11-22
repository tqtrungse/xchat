/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package ticket

import slicepool "xchat/pkg/pool/slice"

const (
	SignedVersion    = byte(2)
	SignedHeaderSize = 4
	SigningDomain    = "xxx/iam/ticket-signature/v1\x00"
	MinSignedSize    = SignedHeaderSize + MinPayloadSize + 64
)

// SigningMessage prefixes the unsigned envelope with the ticket-signature domain
// and returns an owned pooled buffer that does not alias unsigned. The caller
// must clear the buffer and return it to slicepool after use.
func SigningMessage(unsigned []byte) []byte {
	domainLen := len(SigningDomain)
	message := slicepool.Get(domainLen + len(unsigned))
	copy(message, SigningDomain)
	copy(message[domainLen:], unsigned)
	return message
}
