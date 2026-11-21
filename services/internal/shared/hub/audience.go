/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package hub

import "fmt"

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
