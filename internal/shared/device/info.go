/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package device

import (
	"time"

	"xchat/internal/iam/domain/user"
)

type Info struct {
	ID          ID
	UserID      user.ID
	Name        string
	State       State
	PeerPub     []byte
	IdentityPub []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   time.Time
	SeenAt      time.Time
}
