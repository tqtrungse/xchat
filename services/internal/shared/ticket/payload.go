/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package ticket

import (
	"crypto/ed25519"
	"encoding/binary"

	"xchat/services/internal/shared/crypto"

	slicepool "xchat/pkg/pool/slice"
)

const (
	MinPayloadSize = 112

	// The 111 fixed bytes are:
	// TicketID (16),
	// DeviceID (8),
	// IssuedAt (8),
	// ExpiresAt (8),
	// IdentityPub (32),
	// HubAudience length (2),
	// HubKeyID (4),
	// HubPublicKey (32),
	// HubWireVersion (1)
	fixedPayloadSize = IDSize + 8 + 8 + 8 + ed25519.PublicKeySize + 2 + 4 + crypto.X25519PublicKeySize + 1
)

// MarshalPayload fully validates the claims and encodes the binary ticket payload
// into an owned pooled buffer. The caller must clear it and return it to
// slicepool after use. It does not sign the payload.
func MarshalPayload(info *Info) ([]byte, error) {
	err := info.Validate()
	if err != nil {
		return nil, err
	}

	hubAudienceLen := len(info.HubAudience)
	buf := slicepool.Get(fixedPayloadSize + hubAudienceLen)
	buf = buf[:74]
	copy(buf, info.TicketID[:])
	binary.LittleEndian.PutUint64(buf[IDSize:], info.DeviceID.Uint64())
	binary.LittleEndian.PutUint64(buf[24:], uint64(info.IssuedAt))
	binary.LittleEndian.PutUint64(buf[32:], uint64(info.ExpiresAt))
	copy(buf[40:], info.IdentityPub[:])
	binary.LittleEndian.PutUint16(buf[72:], uint16(hubAudienceLen))
	buf = append(buf, info.HubAudience...)
	buf = binary.LittleEndian.AppendUint32(buf, info.HubKeyID)
	buf = append(buf, info.HubPublicKey[:]...)
	buf = append(buf, info.HubWireVersion)
	return buf, nil
}
