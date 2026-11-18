/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"
	"errors"
	"time"

	"xchat/internal/iam/domain/user"
	"xchat/internal/iam/infrastructure/mysql/model"

	"xchat/pkg/database/sql"

	"gorm.io/gorm"
)

type UserReader struct{}

// NewUserReader creates the MySQL reader used to get/list user.
func NewUserReader() UserReader {
	return UserReader{}
}

func (ur *UserReader) Get(dbCtx context.Context, userID user.ID) (*user.Info, error) {
	val, err := sql.ExtractDB(dbCtx)
	if err != nil {
		return nil, err
	}

	type Record struct {
		ID           user.ID
		ExternalID   string
		Username     string
		PasswordHash []byte
		PhoneHash    []byte
		PhoneTail    uint16
		State        user.State
		NumDevices   uint8
		CreatedAt    time.Time
		UpdatedAt    time.Time
	}

	var record Record
	err = val.(*gorm.DB).
		WithContext(dbCtx).
		Model(&model.User{}).
		Select([]string{
			"id",
			"external_id",
			"username",
			"password_hash",
			"phone_hash",
			"phone_tail",
			"state",
			"COUNT(*) AS num_devices",
			"created_at",
			"updated_at",
		}).
		Joins("INNER JOIN devices ON users.id = devices.user_id").
		Where("id = ?", userID).
		Group("users.id").
		Take(&record).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &user.Info{
		ID:           record.ID,
		ExternalID:   record.ExternalID,
		Username:     record.Username,
		PasswordHash: record.PasswordHash,
		PhoneHash:    record.PhoneHash,
		PhoneTail:    record.PhoneTail,
		State:        record.State,
		NumDevices:   record.NumDevices,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}, nil
}
