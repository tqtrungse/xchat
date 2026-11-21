/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package hub

const (
	WireVersion      = uint8(2)
	MaxAudienceBytes = 128
)

// Descriptor contains public data only. Address is a client-reachable TCP
// endpoint of the service, not necessarily the address of an individual pod.
// PublicKey uses a slice so encoding/json decodes Base64 without padding or
// truncating malformed keys into a fixed-size array.
type Descriptor struct {
	Address   string `json:"address"`
	Audience  string `json:"audience"`
	KeyID     uint32 `json:"key_id"`
	PublicKey []byte `json:"public_key"`
	Protocol  string `json:"protocol"`
	Version   uint8  `json:"version"`
}
