/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 *  SPDX-License-Identifier: Apache-2.0
 */

package user

import "time"

const maxDevices = 5

type Info struct {
	ID           ID
	ExternalID   string
	Username     string
	PasswordHash []byte
	PhoneHash    []byte
	PhoneTail    uint16
	State        State
	NumDevices   uint8
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (i Info) ReachMaxDevices() bool {
	return i.NumDevices >= maxDevices
}
