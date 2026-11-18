/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package registerdevice

import (
	"context"

	"xchat/internal/iam/domain/user"
	"xchat/internal/shared/device"
)

type KeyDeriver interface {
	ValidatePeerPub(peerPub []byte) error
}

type PasswordHasher interface {
	Hash(ctx context.Context, password []byte) ([]byte, error)
	Verify(ctx context.Context, password []byte, hash []byte) error
}

type UserReader interface {
	// Get reads the user and the number of devices of the user. It returns nil
	// if the user does not exist. If locking is true, Get locks the user row
	// until the transaction ends.
	Get(dbCtx context.Context, userID user.ID, locking bool) (*user.Info, error)
}

type UserWriter interface {
	Create(dbCtx context.Context, user user.Info) error
}

type DeviceWriter interface {
	Create(dbCtx context.Context, device device.Info) error
}
