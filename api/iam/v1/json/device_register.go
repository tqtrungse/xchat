/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package json

type RegisterDeviceReq struct {
	Username          string `json:"username"`            // User will type username
	Password          string `json:"password"`            // User will type password -> Client hashes SHA-256
	PhoneHash         string `json:"phone_hash"`          // Client hashes HMAC-SHA256 truncated to 128 bits
	PhoneTail         uint16 `json:"phone_tail"`          // Last 3 digits: 032 -> 32, 123 -> 123
	DeviceName        string `json:"device_name"`         // Client gets model + OS name
	DeviceIdentityPub []byte `json:"device_identity_pub"` // Client generates ed25519 public key
	DevicePeerPub     []byte `json:"device_peer_pub"`     // Client generates x25519 public key
}

type RegisterDeviceResp struct {
	DeviceID string `json:"device_id"`
}
