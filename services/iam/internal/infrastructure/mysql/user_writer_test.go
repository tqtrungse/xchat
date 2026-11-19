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

	"xchat/services/iam/internal/domain/user"

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
		prepare   func(f *fields)
		args      user.Info
		wantErr   bool
		wantErrIs error
	}

	passwordHash := make([]byte, 255)
	_, _ = rand.Read(passwordHash)

	phoneHash := make([]byte, 16)
	_, _ = rand.Read(phoneHash)

	insertQuery := "INSERT INTO `users` (`id`,`external_id`,`username`,`password_hash`,`phone_hash`,`phone_tail`,`state`) VALUES (?,?,?,?,?,?,?)"
	query := fmt.Sprintf("^%s$", regexp.QuoteMeta(insertQuery))

	testCases := map[string]testcase{
		"Success": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectExec(query).
					WithArgs(
						user.ID(11),
						"sms|123456789",
						"username",
						passwordHash,
						phoneHash,
						uint16(123),
						user.StateActive,
					).
					WillReturnResult(sqlmock.NewResult(123, 1))
			},
			args: user.Info{
				ID:           user.ID(11),
				Username:     "username",
				ExternalID:   "sms|123456789",
				PasswordHash: passwordHash,
				PhoneHash:    phoneHash,
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
						user.ID(11),
						"sms|123456789",
						"username",
						passwordHash,
						phoneHash,
						uint16(32),
						user.StateActive,
					).
					WillReturnError(errors.New("expected error"))
			},
			args: user.Info{
				ID:           user.ID(11),
				ExternalID:   "sms|123456789",
				Username:     "username",
				PasswordHash: passwordHash,
				PhoneHash:    phoneHash,
				PhoneTail:    32,
				State:        user.StateActive,
			},
			wantErr: true,
		},
		"Duplicate entry": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectExec(query).
					WithArgs(
						user.ID(11),
						"sms|123456789",
						"username",
						passwordHash,
						phoneHash,
						uint16(32),
						user.StateActive,
					).
					WillReturnError(&sql.DuplicateKeyError{Key: "PRIMARY"})
			},
			args: user.Info{
				ID:           user.ID(11),
				ExternalID:   "sms|123456789",
				Username:     "username",
				PasswordHash: passwordHash,
				PhoneHash:    phoneHash,
				PhoneTail:    32,
				State:        user.StateActive,
			},
			wantErr:   true,
			wantErrIs: user.ErrAlreadyExists,
		},
		"Duplicate username": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectExec(query).
					WithArgs(
						user.ID(11),
						"sms|123456789",
						"username",
						passwordHash,
						phoneHash,
						uint16(32),
						user.StateActive,
					).
					WillReturnError(&sql.DuplicateKeyError{Key: "unique_index_users_on_username"})
			},
			args: user.Info{
				ID:           user.ID(11),
				ExternalID:   "sms|123456789",
				Username:     "username",
				PasswordHash: passwordHash,
				PhoneHash:    phoneHash,
				PhoneTail:    32,
				State:        user.StateActive,
			},
			wantErr:   true,
			wantErrIs: user.ErrUniqueConflict,
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
			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Errorf("[datastore] userWrtier.Create error = %v, want error %v", err, tc.wantErrIs)
			}
		})
	}
}
