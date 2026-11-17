/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package user

import "time"

type Info struct {
	ID           ID
	Username     string
	PasswordHash string
	PhoneHash    string
	PhoneTail    uint16
	State        State
	NumDevices   uint8
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
