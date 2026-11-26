/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"
	"fmt"
	"strings"

	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/infrastructure/mysql/model"

	"xchat/pkg/database/sql"
)

// deviceUniqueIndexPrefix is the prefix of the name of each unique index of the
// devices table: unique_index_devices_on_peer_pub and
// unique_index_devices_on_identity_pub.
const deviceUniqueIndexPrefix = "unique_index_devices_on_"

type DeviceWriter struct{}

// NewDeviceWriter creates the MySQL writer used to create/update/delete device.
func NewDeviceWriter() DeviceWriter {
	return DeviceWriter{}
}

func (dw *DeviceWriter) Create(dbCtx context.Context, info device.Info) error {
	db, err := gormDB(dbCtx)
	if err != nil {
		return err
	}

	deviceModel := &model.Device{
		ID:          info.ID,
		UserID:      info.UserID,
		Name:        info.Name,
		State:       info.State,
		IdentityPub: info.IdentityPub,
		PeerPub:     info.PeerPub,
	}
	err = db.Create(deviceModel).Error
	if err == nil {
		return nil
	}

	key, duplicate := sql.DuplicateKey(err)
	if !duplicate {
		return err
	}

	// A duplicate value in the unique index of the peer public key or of the
	// identity public key shows an existing device. The ID comes from the
	// generator, so a duplicate primary key is a bug. Return it as it is, and
	// the caller treats it as a server error.
	if strings.HasPrefix(key, deviceUniqueIndexPrefix) {
		return fmt.Errorf("%w: %w", device.ErrAlreadyExists, err)
	}
	return err
}
