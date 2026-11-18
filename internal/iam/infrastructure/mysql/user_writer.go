/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"

	"xchat/internal/iam/domain/user"
	"xchat/internal/iam/infrastructure/mysql/model"

	"xchat/pkg/database/sql"

	"gorm.io/gorm"
)

type UserWriter struct{}

// NewUserWriter creates the MySQL writer used to create/update/delete user.
func NewUserWriter() UserWriter {
	return UserWriter{}
}

func (uw *UserWriter) Create(dbCtx context.Context, user user.Info) error {
	val, err := sql.ExtractDB(dbCtx)
	if err != nil {
		return err
	}

	userModel := &model.User{
		ID:           user.ID,
		ExternalID:   user.ExternalID,
		Username:     user.Username,
		PasswordHash: user.PasswordHash,
		PhoneHash:    user.PhoneHash,
		PhoneTail:    user.PhoneTail,
		State:        user.State,
	}
	return val.(*gorm.DB).
		WithContext(dbCtx).
		Create(userModel).
		Error
}
