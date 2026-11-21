/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"
	"errors"

	"xchat/services/iam/internal/application/device/issue_ticket"
	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/domain/user"
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
func (dr *DeviceReader) GetMeta(dbCtx context.Context, deviceID device.ID) (*issue_ticket.DeviceMeta, error) {
	val, err := sql.ExtractDB(dbCtx)
	if err != nil {
		return nil, err
	}

	type Record struct {
		UserID      user.ID
		State       device.State
		IdentityPub []byte
	}

	var record Record
	err = val.(*gorm.DB).
		WithContext(dbCtx).
		Model(&model.Device{}).
		Select([]string{
			"user_id",
			"state",
			"identity_pub",
		}).
		Where("id = ?", deviceID).
		Take(&record).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &issue_ticket.DeviceMeta{
		UserID:      record.UserID,
		State:       record.State,
		IdentityPub: record.IdentityPub,
	}, nil
}
