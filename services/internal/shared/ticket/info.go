/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package ticket

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"time"

	"xchat/services/internal/shared/crypto"
	"xchat/services/internal/shared/device"
	"xchat/services/internal/shared/hub"
)

const (
	MaxTTL = 15 * time.Second
	IDSize = 16
)

// Info contains only public authorization claims signed by IAM.
type Info struct {
	TicketID       [IDSize]byte
	DeviceID       device.ID
	IssuedAt       int64
	ExpiresAt      int64
	IdentityPub    [ed25519.PublicKeySize]byte
	HubAudience    string
	HubKeyID       uint32
	HubPublicKey   [crypto.X25519PublicKeySize]byte
	HubWireVersion uint8
}

// ValidateClaims checks identifiers, the claimed lifetime, hub audience, and wire
// version without elliptic-curve operations. It does not check current expiry.
func (i *Info) ValidateClaims() error {
	if i == nil ||
		i.TicketID == [IDSize]byte{} ||
		i.DeviceID == 0 ||
		i.IssuedAt <= 0 ||
		i.ExpiresAt <= i.IssuedAt ||
		i.ExpiresAt-i.IssuedAt > int64(MaxTTL/time.Second) ||
		i.HubKeyID == 0 ||
		i.HubWireVersion != hub.WireVersion {
		return ErrTicketInvalid
	}

	if hub.ValidateAudience(i.HubAudience) != nil {
		return ErrTicketInvalid
	}
	return nil
}

// Validate checks the claims, device identity key, and Hub X25519 key for callers
// without a trusted Hub identity. It does not authenticate the ticket or check
// expiry against the current time.
func (i *Info) Validate() error {
	err := i.ValidateClaims()
	if err != nil {
		return err
	}

	err = ValidateIdentity(i.IdentityPub[:])
	if err != nil {
		return err
	}

	pub, err := ecdh.X25519().NewPublicKey(i.HubPublicKey[:])
	if err != nil {
		return ErrTicketInvalid
	}

	var (
		probeKey [crypto.X25519PublicKeySize]byte
		probe    *ecdh.PrivateKey
	)

	probe, err = ecdh.X25519().NewPrivateKey(probeKey[:])
	if err != nil {
		return ErrTicketInvalid
	}

	_, err = probe.ECDH(pub)
	if err != nil {
		return ErrTicketInvalid
	}
	return nil
}

// Clear resets all ticket claims and is safe to call on a nil receiver.
func (i *Info) Clear() {
	if i != nil {
		*i = Info{}
	}
}
