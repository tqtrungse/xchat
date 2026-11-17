/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package mysql

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"xchat/internal/iam/domain/user"

	"xchat/pkg/database/sql"
	"xchat/pkg/database/sql/mocks"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/mock/gomock"
)

func Test_UserWriter_Create(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	type fields struct {
		ctx     context.Context
		sqlMock sqlmock.Sqlmock
	}

	type testcase struct {
		prepare func(f *fields)
		args    user.Info
		wantErr bool
	}

	passwordHash := make([]byte, 255)
	_, _ = rand.Read(passwordHash)

	insertQuery := "INSERT INTO `users` (`id`,`username`,`password_hash`,`phone_hash`,`phone_tail`,`state`) VALUES (?,?,?,?,?,?)"
	query := fmt.Sprintf("^%s$", regexp.QuoteMeta(insertQuery))

	testCases := map[string]testcase{
		"Success": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectExec(query).
					WithArgs(
						user.ID("123"),
						"username",
						string(passwordHash),
						"xxxxxxx",
						uint16(123),
						user.StateActive,
					).
					WillReturnResult(sqlmock.NewResult(123, 1))
			},
			args: user.Info{
				ID:           user.ID("123"),
				Username:     "username",
				PasswordHash: string(passwordHash),
				PhoneHash:    "xxxxxxx",
				PhoneTail:    123,
				State:        user.StateActive,
			},
			wantErr: false,
		},
		"Error": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectExec(query).
					WithArgs(
						user.ID("123"),
						"username",
						string(passwordHash),
						"xxxxxxx",
						uint16(32),
						user.StateActive,
					).
					WillReturnError(errors.New("expected error"))
			},
			args: user.Info{
				ID:           user.ID("123"),
				Username:     "username",
				PasswordHash: string(passwordHash),
				PhoneHash:    "xxxxxxx",
				PhoneTail:    32,
				State:        user.StateActive,
			},
			wantErr: true,
		},
	}

	for description, tc := range testCases {
		t.Run(description, func(t *testing.T) {
			db, sqlMock := mocks.NewMysql(t)
			mocks.Setup(t, db)

			f := &fields{
				ctx:     t.Context(),
				sqlMock: sqlMock,
			}

			doneSqlMock := mocks.ExpectTransactionSqlMock(
				t,
				f.sqlMock,
				func() {
					tc.prepare(f)
				},
				tc.wantErr,
			)
			defer doneSqlMock()

			var (
				err    error
				writer = NewUserWriter()
			)

			err = db.Transaction(f.ctx, sql.TxSourcePrimary, func(dbCtx context.Context) error {
				err = writer.Create(dbCtx, tc.args)
				return err
			})
			if (err != nil) != tc.wantErr {
				t.Errorf("[datastore] userWrtier.Create error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
