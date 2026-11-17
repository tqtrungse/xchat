/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"

	"xchat/internal/iam/infrastructure/mysql/model"
	"xchat/internal/shared/device"

	"xchat/pkg/database/sql"

	"gorm.io/gorm"
)

type DeviceWriter struct{}

// NewDeviceWriter creates the MySQL writer used to create/update/delete device.
func NewDeviceWriter() DeviceWriter {
	return DeviceWriter{}
}

func (dw *DeviceWriter) Create(dbCtx context.Context, device device.Info) error {
	val, err := sql.ExtractDB(dbCtx)
	if err != nil {
		return err
	}

	deviceModel := &model.Device{
		ID:          device.ID,
		UserID:      device.UserID,
		Name:        device.Name,
		State:       device.State,
		IdentityPub: device.IdentityPub,
		PeerPub:     device.PeerPub,
	}
	return val.(*gorm.DB).
		WithContext(dbCtx).
		Create(deviceModel).
		Error
}
