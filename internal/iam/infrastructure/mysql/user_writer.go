/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"
	"fmt"
	"strings"

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

func (uw *UserWriter) Create(dbCtx context.Context, info user.Info) error {
	val, err := sql.ExtractDB(dbCtx)
	if err != nil {
		return err
	}

	userModel := &model.User{
		ID:           info.ID,
		ExternalID:   info.ExternalID,
		Username:     info.Username,
		PasswordHash: info.PasswordHash,
		PhoneHash:    info.PhoneHash,
		PhoneTail:    info.PhoneTail,
		State:        info.State,
	}
	err = val.(*gorm.DB).
		WithContext(dbCtx).
		Create(userModel).
		Error
	if err == nil {
		return nil
	}

	key, duplicate := sql.DuplicateKey(err)
	if !duplicate {
		return err
	}

	switch {
	case key == "PRIMARY":
		return fmt.Errorf("%w: %w", user.ErrAlreadyExists, err)
	case strings.HasPrefix(key, "unique_index_users_on_"):
		return fmt.Errorf("%w: %w", user.ErrUniqueConflict, err)
	default:
		return err
	}
}
