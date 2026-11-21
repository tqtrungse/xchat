/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package issue_ticket

import "xchat/services/internal/shared/ticket"

type TicketSigner interface {
	// Sign validates and signs the ticket claims, transferring ownership of the
	// returned buffer to the caller on success. The caller must clear it and
	// return it to slicepool after use. On error, no buffer is returned.
	Sign(info *ticket.Info) ([]byte, error)
}
