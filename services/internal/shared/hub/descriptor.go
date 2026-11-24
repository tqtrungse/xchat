/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package hub

import (
	"fmt"
	"regexp"
)

const (
	Protocol         = "Noise_NK_25519_ChaChaPoly_SHA256"
	WireVersion      = uint8(1)
	MaxAudienceBytes = 128
	MaxAddressBytes  = 255
)

var hostnameLabelPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

// Descriptor contains public data only. Address is a client-reachable TCP
// endpoint of the service, not necessarily the address of an individual pod.
// PublicKey uses a slice so the JSON decoder decodes Base64 without padding or
// truncating malformed keys into a fixed-size array.
type Descriptor struct {
	Address   string `json:"address"`
	Audience  string `json:"audience"`
	KeyID     uint32 `json:"key_id"`
	PublicKey []byte `json:"public_key"`
	Protocol  string `json:"protocol"`
	Version   uint8  `json:"version"`
}

// Validate checks each field that a ticket and a client need.
// The error does not contain a value of the descriptor.
func (d *Descriptor) Validate() error {
	if d.Protocol != Protocol || d.Version != WireVersion {
		return fmt.Errorf("unsupported hub Noise protocol or wire version")
	}

	if d.KeyID == 0 {
		return fmt.Errorf("hub key ID must be nonzero")
	}

	err := validateAddress(d.Address)
	if err != nil {
		return err
	}

	err = ValidateAudience(d.Audience)
	if err != nil {
		return err
	}

	return validatePublicKey(d.PublicKey)
}
