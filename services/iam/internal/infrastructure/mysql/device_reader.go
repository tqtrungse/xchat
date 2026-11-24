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
	"xchat/services/iam/internal/infrastructure/mysql/model"

	"xchat/pkg/database/sql"

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
	val, err := sql.ExtractDB(dbCtx)
	if err != nil {
		return nil, err
	}

	var deviceRecord model.Device
	err = val.(*gorm.DB).
		WithContext(dbCtx).
		Model(&model.Device{}).
		Select([]string{
			"user_id",
			"state",
			"identity_pub",
		}).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select([]string{
				"id",
				"state",
			})
		}).
		Where("id = ?", deviceID).
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
		UserState:   deviceRecord.User.State,
		State:       deviceRecord.State,
		IdentityPub: deviceRecord.IdentityPub,
	}, nil
}
