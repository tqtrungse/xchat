/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

// Package tickettest supplies deterministic public test keys, never production secrets.
package tickettest

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"time"

	"xchat/services/internal/shared/crypto"
	"xchat/services/internal/shared/hub"
	"xchat/services/internal/shared/ticket"
)

// IdentityPrivate returns a newly allocated, deterministic Ed25519 device identity for tests.
func IdentityPrivate() ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize))
}

// SigningSeed returns a newly allocated, deterministic Ed25519 ticket-signing seed for tests.
func SigningSeed() []byte {
	return bytes.Repeat([]byte{0x22}, ed25519.SeedSize)
}

// HubPrivate returns a newly allocated, deterministic X25519 private key for tests.
func HubPrivate() []byte {
	return bytes.Repeat([]byte{0x33}, 32)
}

// HubPublic derives the X25519 public key matching the deterministic HubPrivate fixture.
func HubPublic() [crypto.X25519PublicKeySize]byte {
	p, _ := ecdh.X25519().NewPrivateKey(HubPrivate())
	return [crypto.X25519PublicKeySize]byte(p.PublicKey().Bytes())
}

// New returns public ticket claims for the test device and Hub, valid for 15 seconds
// from the current time. The claims are not signed.
func New() *ticket.Info {
	now := time.Now().Unix()
	info := &ticket.Info{
		DeviceID:       42,
		IssuedAt:       now,
		ExpiresAt:      now + 15,
		HubAudience:    "hub.chat",
		HubKeyID:       7,
		HubPublicKey:   HubPublic(),
		HubWireVersion: hub.WireVersion,
	}
	info.TicketID[0] = 1
	copy(info.IdentityPub[:], IdentityPrivate().Public().(ed25519.PublicKey))
	return info
}
