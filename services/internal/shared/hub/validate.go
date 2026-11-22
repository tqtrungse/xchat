/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package hub

import (
	"crypto/ecdh"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"xchat/services/internal/shared/crypto"
)

// ValidateAudience requires a nonempty, bounded audience made of printable ASCII without spaces.
func ValidateAudience(audience string) error {
	if len(audience) == 0 || len(audience) > MaxAudienceBytes {
		return fmt.Errorf("hub audience must contain 1..%d bytes", MaxAudienceBytes)
	}

	for i := range len(audience) {
		if audience[i] < 0x21 || audience[i] > 0x7e {
			return fmt.Errorf("hub audience must use printable ASCII without spaces")
		}
	}
	return nil
}

// validateAddress checks a bounded host:port endpoint with a nonzero numeric port
// and an IP address or valid hostname. IPv6 addresses must be bracketed.
func validateAddress(address string) error {
	if len(address) == 0 || len(address) > MaxAddressBytes {
		return fmt.Errorf("hub address must contain 1..%d bytes", MaxAddressBytes)
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return fmt.Errorf("hub address must be host:port (IPv6 requires brackets)")
	}

	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return fmt.Errorf("hub address port must be in 1..65535")
	}
	if net.ParseIP(host) != nil {
		return nil
	}

	for label := range strings.SplitSeq(strings.TrimSuffix(host, "."), ".") {
		if !hostnameLabelPattern.MatchString(label) {
			return fmt.Errorf("invalid hub address hostname")
		}
	}
	return nil
}

// validatePublicKey checks that publicKey is an X25519 public key. It rejects
// a low-order point, because the shared secret with that point is zero. The
// check is the same as the hub key check of ticket.Info.Validate.
func validatePublicKey(publicKey []byte) error {
	if len(publicKey) != crypto.X25519PublicKeySize {
		return fmt.Errorf("hub public key must contain %d bytes", crypto.X25519PublicKeySize)
	}

	key, err := ecdh.X25519().NewPublicKey(publicKey)
	if err != nil {
		return errors.New("hub public key is not valid")
	}

	// The clamped zero scalar is a multiple of the cofactor 8. Thus ECDH with
	// a low-order point gives zero, and ECDH returns an error.
	var probeSeed [crypto.X25519PublicKeySize]byte
	probe, err := ecdh.X25519().NewPrivateKey(probeSeed[:])
	if err != nil {
		return errors.New("hub public key is not valid")
	}
	_, err = probe.ECDH(key)
	if err != nil {
		return errors.New("hub public key is a low-order point")
	}
	return nil
}
