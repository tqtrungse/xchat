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
	"xchat/internal/shared/device"

	"xchat/pkg/database/sql"
	"xchat/pkg/database/sql/mocks"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/mock/gomock"
)

func Test_DeviceWriter_Create(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	type fields struct {
		ctx     context.Context
		sqlMock sqlmock.Sqlmock
	}

	type testcase struct {
		prepare func(f *fields)
		args    device.Info
		wantErr bool
	}

	identityPub := make([]byte, 32)
	_, _ = rand.Read(identityPub)

	peerPub := make([]byte, 32)
	_, _ = rand.Read(peerPub)

	insertQuery := "INSERT INTO `devices` (`id`,`user_id`,`name`,`state`,`peer_pub`,`identity_pub`) VALUES (?,?,?,?,?,?)"
	query := fmt.Sprintf("^%s$", regexp.QuoteMeta(insertQuery))

	testCases := map[string]testcase{
		"Success": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectExec(query).
					WithArgs(
						device.ID(1),
						user.ID(11),
						"iPhoneX-IOS10",
						device.StateActive,
						peerPub,
						identityPub,
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			args: device.Info{
				ID:          device.ID(1),
				UserID:      user.ID(11),
				Name:        "iPhoneX-IOS10",
				State:       device.StateActive,
				PeerPub:     peerPub,
				IdentityPub: identityPub,
			},
			wantErr: false,
		},
		"Error": {
			prepare: func(f *fields) {
				f.sqlMock.
					ExpectExec(query).
					WithArgs(
						device.ID(1),
						user.ID(11),
						"iPhoneX-IOS10",
						device.StateActive,
						peerPub,
						identityPub,
					).
					WillReturnError(errors.New("expected error"))
			},
			args: device.Info{
				ID:          device.ID(1),
				UserID:      user.ID(11),
				Name:        "iPhoneX-IOS10",
				State:       device.StateActive,
				PeerPub:     peerPub,
				IdentityPub: identityPub,
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
				writer = NewDeviceWriter()
			)

			err = db.Transaction(f.ctx, sql.TxSourcePrimary, func(dbCtx context.Context) error {
				err = writer.Create(dbCtx, tc.args)
				return err
			})
			if (err != nil) != tc.wantErr {
				t.Errorf("[datastore] deviceWrtier.Create error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
