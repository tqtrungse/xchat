/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package register

type Request struct {
	Username          string `json:"username"`            // User will type username
	Password          []byte `json:"password"`            // User will type password -> Client sends SHA-256
	PhoneHash         []byte `json:"phone_hash"`          // Client sends HMAC-SHA256 truncated to 128 bits
	PhoneTail         uint16 `json:"phone_tail"`          // Last 3 digits: 032 -> 32, 123 -> 123
	DeviceName        string `json:"device_name"`         // Client gets model + OS name
	DevicePeerPub     []byte `json:"device_peer_pub"`     // Client sends x25519 public key
	DeviceIdentityPub []byte `json:"device_identity_pub"` // Client sends ed25519 public key
}

type Response struct {
	DeviceID string `json:"device_id"`
}
