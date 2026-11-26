/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package register

// Request is the body of POST /api/device/register.
type Request struct {
	// Username is the name that the user types.
	Username string `json:"username"`

	// Password is the SHA-256 digest of the password that the user types. The
	// client makes the digest, so the server does not receive the password.
	Password []byte `json:"password"`

	// PhoneHash is the HMAC-SHA256 of the phone number, truncated to 128 bits.
	// The client makes the hash.
	PhoneHash []byte `json:"phone_hash"`

	// PhoneTail is the value of the last 3 digits of the phone number. For
	// example, "032" gives 32, and "123" gives 123.
	PhoneTail uint16 `json:"phone_tail"`

	// DeviceName is the model and the OS name that the client reads from the
	// device.
	DeviceName string `json:"device_name"`

	// DevicePeerPub is the X25519 public key of the device.
	DevicePeerPub []byte `json:"device_peer_pub"`

	// DeviceIdentityPub is the Ed25519 public key of the device.
	DeviceIdentityPub []byte `json:"device_identity_pub"`
}

// Response is the body of the response after the use case registers the
// device.
type Response struct {
	// DeviceID is the ID of the new device as a decimal string.
	DeviceID string `json:"device_id"`
}
