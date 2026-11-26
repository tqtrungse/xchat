/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"

	"xchat/pkg/database/sql"

	"gorm.io/gorm"
)

// gormDB gives the GORM handle that Transaction or WithDB attached to dbCtx.
// The handle uses dbCtx, so the query stops when dbCtx ends. The MySQL DB of
// pkg/database/sql attaches only a *gorm.DB.
func gormDB(dbCtx context.Context) (*gorm.DB, error) {
	val, err := sql.ExtractDB(dbCtx)
	if err != nil {
		return nil, err
	}

	db, isGormDB := val.(*gorm.DB)
	if !isGormDB {
		return nil, sql.ErrExtractDB
	}
	return db.WithContext(dbCtx), nil
}
