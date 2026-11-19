/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"
	"errors"
	"time"

	"xchat/services/iam/internal/domain/user"
	"xchat/services/iam/internal/infrastructure/mysql/model"

	"xchat/pkg/database/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserReader struct{}

// NewUserReader creates the MySQL reader used to get/list user.
func NewUserReader() UserReader {
	return UserReader{}
}

func (ur *UserReader) Get(dbCtx context.Context, userID user.ID, locking bool) (*user.Info, error) {
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

	db := val.(*gorm.DB)
	if locking {
		db = db.
			WithContext(dbCtx).
			Clauses(clause.Locking{
				Strength: clause.LockingStrengthUpdate,
			})
	}

	var record Record
	err = db.
		WithContext(dbCtx).
		Model(&model.User{}).
		Select([]string{
			"users.id",
			"users.external_id",
			"users.username",
			"users.password_hash",
			"users.phone_hash",
			"users.phone_tail",
			"users.state",
			"COUNT(devices.id) AS num_devices",
			"users.created_at",
			"users.updated_at",
		}).
		Joins("LEFT JOIN devices ON users.id = devices.user_id").
		Where("users.id = ?", userID).
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
