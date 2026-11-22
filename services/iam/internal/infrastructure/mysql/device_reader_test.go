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

	"xchat/services/iam/internal/application/device/issueticket"
	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/domain/user"

	"xchat/pkg/database/sql"
	"xchat/pkg/database/sql/mocks"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
)

func Test_DeviceReader_GetMeta(t *testing.T) {
	t.Parallel()

	type fields struct {
		ctx     context.Context
		sqlMock sqlmock.Sqlmock
	}

	type testcase struct {
		prepare  func(f *fields)
		deviceID device.ID
		expected *issueticket.DeviceMeta
		wantErr  bool
	}

	identityPub := make([]byte, 32)
	_, _ = rand.Read(identityPub)

	// The deleted_at condition makes sure that a deleted device is not found.
	selectQuery := strings.Join([]string{
		"SELECT `user_id`,`state`,`identity_pub`",
		"FROM `devices`",
		"WHERE id = ? AND `devices`.`deleted_at` IS NULL",
		"LIMIT ?",
	}, " ")
	query := fmt.Sprintf("^%s$", regexp.QuoteMeta(selectQuery))
	columns := []string{
		"user_id",
		"state",
		"identity_pub",
	}

	testCases := map[string]testcase{
		"Success (found)": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectQuery(query).
					WithArgs(
						device.ID(21),
						1,
					).
					WillReturnRows(
						sqlmock.NewRows(columns).AddRow(
							uint64(123),
							device.StateLocked,
							identityPub,
						),
					)
			},
			deviceID: device.ID(21),
			expected: &issueticket.DeviceMeta{
				UserID:      user.ID(123),
				State:       device.StateLocked,
				IdentityPub: identityPub,
			},
			wantErr: false,
		},
		"Success (not found)": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectQuery(query).
					WithArgs(
						device.ID(21),
						1,
					).
					WillReturnRows(sqlmock.NewRows(columns))
			},
			deviceID: device.ID(21),
			expected: nil,
			wantErr:  false,
		},
		"Error": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectQuery(query).
					WithArgs(
						device.ID(21),
						1,
					).
					WillReturnError(errors.New("expected error"))
			},
			deviceID: device.ID(21),
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
				ret    *issueticket.DeviceMeta
				reader = NewDeviceReader()
			)

			err = db.WithDB(f.ctx, sql.TxSourceReplicas, func(dbCtx context.Context) error {
				ret, err = reader.GetMeta(dbCtx, tc.deviceID)
				return err
			})
			if (err != nil) != tc.wantErr {
				t.Errorf("[datastore] deviceReader.GetMeta error = %v, wantErr %v", err, tc.wantErr)
			}

			diff := cmp.Diff(ret, tc.expected)
			if diff != "" {
				t.Errorf("[datastore] deviceReader.GetMeta value is mismatch (-actual +expected):\n%s", diff)
			}
		})
	}
}
