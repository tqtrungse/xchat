/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package model

import (
	"time"

	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/domain/user"

	"gorm.io/gorm"
)

type Device struct {
	ID          device.ID `gorm:"primaryKey;autoIncrement:false"`
	UserID      user.ID
	Name        string
	State       device.State
	PeerPub     []byte
	IdentityPub []byte
	CreatedAt   time.Time      `gorm:"->"`
	UpdatedAt   time.Time      `gorm:"<-:update"`
	DeletedAt   gorm.DeletedAt `gorm:"->"`
	SeenAt      time.Time      `gorm:"<-:update"`
}
