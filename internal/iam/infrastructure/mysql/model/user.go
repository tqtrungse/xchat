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
	Username     string
	PasswordHash string
	PhoneHash    string
	PhoneTail    uint16
	State        user.State
	CreatedAt    time.Time `gorm:"->"`
	UpdatedAt    time.Time `gorm:"<-:update"`
}
