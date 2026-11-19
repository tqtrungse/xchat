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
	"strings"
	"testing"

	"xchat/services/iam/internal/domain/user"

	"xchat/pkg/database/sql"
	"xchat/pkg/database/sql/mocks"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"
)

func Test_UserReader_Get(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	type fields struct {
		ctx     context.Context
		sqlMock sqlmock.Sqlmock
	}

	type Args struct {
		userID  user.ID
		locking bool
	}

	type testcase struct {
		prepare  func(f *fields)
		args     Args
		expected *user.Info
		wantErr  bool
	}

	passwordHash := make([]byte, 255)
	_, _ = rand.Read(passwordHash)

	phoneHash := make([]byte, 16)
	_, _ = rand.Read(phoneHash)

	selectQuery := strings.Join([]string{
		"SELECT users.id,users.external_id,users.username,users.password_hash,users.phone_hash,users.phone_tail,users.state,COUNT(devices.id) AS num_devices,users.created_at,users.updated_at",
		"FROM `users`",
		"LEFT JOIN devices ON users.id = devices.user_id",
		"WHERE users.id = ?",
		"GROUP BY `users`.`id`",
		"LIMIT ?",
	}, " ")
	columns := []string{
		"id",
		"external_id",
		"username",
		"password_hash",
		"phone_hash",
		"phone_tail",
		"state",
		"num_devices",
	}
	query := fmt.Sprintf("^%s$", regexp.QuoteMeta(selectQuery))

	testCases := map[string]testcase{
		"Success (found + without locking)": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectQuery(query).
					WithArgs(
						user.ID(11),
						1,
					).
					WillReturnRows(
						sqlmock.NewRows(columns).AddRow(
							user.ID(11),
							"sms|123456789",
							"username",
							passwordHash,
							phoneHash,
							uint16(32),
							user.StateActive,
							uint8(3),
						),
					)
			},
			args: Args{
				userID:  user.ID(11),
				locking: false,
			},
			expected: &user.Info{
				ID:           user.ID(11),
				ExternalID:   "sms|123456789",
				Username:     "username",
				PasswordHash: passwordHash,
				PhoneHash:    phoneHash,
				PhoneTail:    32,
				State:        user.StateActive,
				NumDevices:   3,
			},
			wantErr: false,
		},
		"Success (found + with locking)": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(
						strings.Join([]string{
							"SELECT users.id,users.external_id,users.username,users.password_hash,users.phone_hash,users.phone_tail,users.state,COUNT(devices.id) AS num_devices,users.created_at,users.updated_at",
							"FROM `users`",
							"LEFT JOIN devices ON users.id = devices.user_id",
							"WHERE users.id = ?",
							"GROUP BY `users`.`id`",
							"LIMIT ? FOR UPDATE",
						}, " "),
					))).
					WithArgs(
						user.ID(11),
						1,
					).
					WillReturnRows(
						sqlmock.NewRows(columns).AddRow(
							user.ID(11),
							"sms|123456789",
							"username",
							passwordHash,
							phoneHash,
							uint16(32),
							user.StateActive,
							uint8(3),
						),
					)
			},
			args: Args{
				userID:  user.ID(11),
				locking: true,
			},
			expected: &user.Info{
				ID:           user.ID(11),
				ExternalID:   "sms|123456789",
				Username:     "username",
				PasswordHash: passwordHash,
				PhoneHash:    phoneHash,
				PhoneTail:    32,
				State:        user.StateActive,
				NumDevices:   3,
			},
			wantErr: false,
		},
		"Success (not found)": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectQuery(query).
					WithArgs(
						user.ID(11),
						1,
					).
					WillReturnRows(sqlmock.NewRows(columns))
			},
			args: Args{
				userID:  user.ID(11),
				locking: false,
			},
			expected: nil,
			wantErr:  false,
		},
		"Error": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectQuery(query).
					WithArgs(
						user.ID(11),
						1,
					).
					WillReturnError(errors.New("expected error"))
			},
			args: Args{
				userID:  user.ID(11),
				locking: false,
			},
			expected: nil,
			wantErr:  true,
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

			doneSqlMock := mocks.ExpectSqlMock(
				t,
				f.sqlMock,
				func() {
					tc.prepare(f)
				},
			)
			defer doneSqlMock()

			var (
				err    error
				ret    *user.Info
				reader = NewUserReader()
			)

			err = db.WithDB(f.ctx, sql.TxSourceReplicas, func(dbCtx context.Context) error {
				ret, err = reader.Get(dbCtx, tc.args.userID, tc.args.locking)
				return err
			})
			if (err != nil) != tc.wantErr {
				t.Errorf("[datastore] userReader.Read error = %v, wantErr %v", err, tc.wantErr)
			}

			diff := cmp.Diff(ret, tc.expected)
			if diff != "" {
				t.Errorf("[datastore] userReader.Read value is mismatch (-actual +expected):n%s", diff)
			}
		})
	}
}
