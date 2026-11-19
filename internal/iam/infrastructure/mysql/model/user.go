/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package model

import (
	"time"

	"xchat/internal/iam/domain/user"
)

type User struct {
	ID           user.ID `gorm:"primaryKey;autoIncrement:false"`
	ExternalID   string
	Username     string
	PasswordHash []byte
	PhoneHash    []byte
	PhoneTail    uint16
	State        user.State
	CreatedAt    time.Time `gorm:"->"`
	UpdatedAt    time.Time `gorm:"<-:update"`
}
