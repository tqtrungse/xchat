/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package issueticket

import (
	"context"

	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/domain/user"
	"xchat/services/internal/shared/ticket"
)

type DeviceMeta struct {
	UserID      user.ID
	UserState   user.State
	State       device.State
	IdentityPub []byte
}

type DeviceReader interface {
	// GetMeta loads the device owner, state, and identity key from the
	// database context, returning nil, nil when no device is found.
	GetMeta(dbCtx context.Context, deviceID device.ID) (*DeviceMeta, error)
}

type TicketSigner interface {
	// Sign validates and signs the ticket claims, transferring ownership of the
	// returned buffer to the caller on success. The caller must clear it and
	// return it to slicepool after use. On error, no buffer is returned.
	Sign(info *ticket.Info) ([]byte, error)
}
