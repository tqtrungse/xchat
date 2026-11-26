/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"
	"errors"

	"xchat/services/iam/internal/application/device/issueticket"
	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/domain/user"
	"xchat/services/iam/internal/infrastructure/mysql/model"

	"gorm.io/gorm"
)

type DeviceReader struct{}

// NewDeviceReader creates the MySQL reader used to get device.
func NewDeviceReader() DeviceReader {
	return DeviceReader{}
}

// GetMeta reads the user ID of the owner, the state, and the identity public
// key of the device. It returns nil if the device does not exist or is
// deleted. The soft delete of model.Device adds the deleted_at condition.
func (dr *DeviceReader) GetMeta(dbCtx context.Context, deviceID device.ID) (*issueticket.DeviceMeta, error) {
	db, err := gormDB(dbCtx)
	if err != nil {
		return nil, err
	}

	type record struct {
		UserID      user.ID
		UserState   user.State
		State       device.State
		IdentityPub []byte
	}

	// The users table also has the columns id and state. Each column must
	// have its table name, or MySQL rejects the query as ambiguous.
	var deviceRecord record
	err = db.
		Model(&model.Device{}).
		Select([]string{
			"users.id AS user_id",
			"users.state AS user_state",
			"devices.state",
			"devices.identity_pub",
		}).
		Joins("INNER JOIN users ON users.id = devices.user_id").
		Where("devices.id = ?", deviceID).
		Take(&deviceRecord).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &issueticket.DeviceMeta{
		UserID:      deviceRecord.UserID,
		UserState:   deviceRecord.UserState,
		State:       deviceRecord.State,
		IdentityPub: deviceRecord.IdentityPub,
	}, nil
}
