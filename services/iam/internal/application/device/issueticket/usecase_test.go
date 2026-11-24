/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

// The test is in an external package. The mocks import issue_ticket, so an
// internal test package forms an import cycle.
package issueticket_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"os"
	"testing"
	"time"

	"xchat/services/iam/internal/application/device/issueticket"
	issueticketmocks "xchat/services/iam/internal/application/device/issueticket/mocks"
	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/domain/user"
	"xchat/services/internal/shared/crypto"
	"xchat/services/internal/shared/hub"
	"xchat/services/internal/shared/ticket"

	"xchat/pkg/database/sql"
	sqlmocks "xchat/pkg/database/sql/mocks"
	"xchat/pkg/log"
	"xchat/pkg/log/logtest"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testUserID   = user.ID(42)
	testDeviceID = device.ID(21)
)

func validHub() hub.Descriptor {
	return hub.Descriptor{
		Address:   "hub.example:7443",
		Audience:  "xchat-hub",
		KeyID:     7,
		PublicKey: bytes.Repeat([]byte{9}, crypto.X25519PublicKeySize),
		Protocol:  "tcp",
		Version:   hub.WireVersion,
	}
}

func newIdentityPub(t *testing.T) []byte {
	t.Helper()

	identityPub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	return identityPub
}

type fixture struct {
	issuer       *issueticket.Usecase
	db           *sqlmocks.MockDB
	deviceReader *issueticketmocks.MockDeviceReader
	ticketSigner *issueticketmocks.MockTicketSigner
	logger       *log.Logger
	logPath      string
}

func newFixture(t *testing.T, hubDescriptor hub.Descriptor) *fixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	logger, closeFn, logPath := logtest.New(t, log.Config{})
	t.Cleanup(func() { _ = closeFn() })

	f := &fixture{
		db:           sqlmocks.NewMockDB(ctrl),
		deviceReader: issueticketmocks.NewMockDeviceReader(ctrl),
		ticketSigner: issueticketmocks.NewMockTicketSigner(ctrl),
		logger:       logger,
		logPath:      logPath,
	}
	f.issuer = issueticket.New(f.db, logger, f.deviceReader, f.ticketSigner, hubDescriptor)
	return f
}

// logs gives the entries that the logger wrote.
func (f *fixture) logs(t *testing.T) string {
	t.Helper()

	require.NoError(t, f.logger.Sync())
	raw, err := os.ReadFile(f.logPath)
	require.NoError(t, err)
	return string(raw)
}

// expectRead makes WithDB run the callback at once, and makes GetMeta return
// deviceMeta and readErr for testDeviceID.
func (f *fixture) expectRead(deviceMeta *issueticket.DeviceMeta, readErr error) {
	f.db.EXPECT().
		WithDB(gomock.Any(), sql.TxSourcePrimary, gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ sql.TxSource, fn func(context.Context) error) error {
			return fn(ctx)
		})
	f.deviceReader.EXPECT().
		GetMeta(gomock.Any(), testDeviceID).
		Return(deviceMeta, readErr)
}

func TestIssueRejectsInvalidDeviceID(t *testing.T) {
	for _, tc := range []struct {
		name     string
		deviceID string
	}{
		{
			name:     "empty",
			deviceID: "",
		},
		{
			name:     "not a number",
			deviceID: "abc",
		},
		{
			name:     "zero",
			deviceID: "0",
		},
		{
			name:     "negative",
			deviceID: "-1",
		},
		{
			name:     "overflow",
			deviceID: "18446744073709551616",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The fixture sets no expectation. Thus a read of the database fails
			// the test.
			f := newFixture(t, validHub())

			resp, err := f.issuer.IssueTicket(context.Background(), testUserID, issueticket.Request{DeviceID: tc.deviceID})
			require.ErrorIs(t, err, issueticket.ErrRequestInvalid)
			require.Nil(t, resp)
		})
	}
}

func TestIssueRejectsDevice(t *testing.T) {
	identityPub := newIdentityPub(t)

	for _, tc := range []struct {
		name       string
		deviceMeta *issueticket.DeviceMeta
		readErr    error
		wantErr    error
	}{
		{
			name:    "device not found",
			wantErr: issueticket.ErrDeviceNotFound,
		},
		{
			name: "device of another user",
			deviceMeta: &issueticket.DeviceMeta{
				UserID:      testUserID + 1,
				UserState:   user.StateActive,
				State:       device.StateActive,
				IdentityPub: identityPub,
			},
			wantErr: issueticket.ErrDeviceNotFound,
		},
		{
			name: "locked device",
			deviceMeta: &issueticket.DeviceMeta{
				UserID:      testUserID,
				UserState:   user.StateActive,
				State:       device.StateLocked,
				IdentityPub: identityPub,
			},
			wantErr: issueticket.ErrDeviceInactive,
		},
		{
			name: "locked user",
			deviceMeta: &issueticket.DeviceMeta{
				UserID:      testUserID,
				UserState:   user.StateLocked,
				State:       device.StateActive,
				IdentityPub: identityPub,
			},
			wantErr: issueticket.ErrDeviceInactive,
		},
		{
			name:    "database error",
			readErr: errors.New("connection refused"),
			wantErr: issueticket.ErrInternal,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The fixture sets no expectation on the signer. Thus a ticket for a
			// rejected device fails the test.
			f := newFixture(t, validHub())
			f.expectRead(tc.deviceMeta, tc.readErr)

			resp, err := f.issuer.IssueTicket(context.Background(), testUserID, issueticket.Request{DeviceID: "21"})
			require.ErrorIs(t, err, tc.wantErr)
			require.Nil(t, resp)
		})
	}
}

// TestIssueLogsReadError checks the result and the level of the log entry for
// a read error. A request that the client canceled is not a server error. An
// expired deadline makes the server busy, and it is not logged.
func TestIssueLogsReadError(t *testing.T) {
	for _, tc := range []struct {
		name      string
		makeCtx   func() (context.Context, context.CancelFunc)
		readErr   error
		wantErr   error
		wantError bool
	}{
		{
			name: "database error",
			makeCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			readErr:   errors.New("connection refused"),
			wantErr:   issueticket.ErrInternal,
			wantError: true,
		},
		{
			name: "client canceled",
			makeCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			readErr:   context.Canceled,
			wantErr:   issueticket.ErrInternal,
			wantError: false,
		},
		{
			name: "server deadline",
			makeCtx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			readErr:   context.DeadlineExceeded,
			wantErr:   issueticket.ErrBusy,
			wantError: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, validHub())
			f.expectRead(nil, tc.readErr)
			ctx, cancel := tc.makeCtx()
			defer cancel()

			resp, err := f.issuer.IssueTicket(ctx, testUserID, issueticket.Request{DeviceID: "21"})
			require.ErrorIs(t, err, tc.wantErr)
			require.Nil(t, resp)

			logs := f.logs(t)
			if tc.wantError {
				require.Contains(t, logs, `"level":"error"`)
				require.Contains(t, logs, `"msg":"failed to read device"`)
			} else {
				require.NotContains(t, logs, `"level":"error"`)
			}
		})
	}
}

func TestIssueHidesSignError(t *testing.T) {
	f := newFixture(t, validHub())
	f.expectRead(
		&issueticket.DeviceMeta{
			UserID:      testUserID,
			UserState:   user.StateActive,
			State:       device.StateActive,
			IdentityPub: newIdentityPub(t),
		},
		nil,
	)
	f.ticketSigner.EXPECT().
		Sign(gomock.Any()).
		Return(nil, ticket.ErrTicketInvalid)

	resp, err := f.issuer.IssueTicket(context.Background(), testUserID, issueticket.Request{DeviceID: "21"})
	require.ErrorIs(t, err, issueticket.ErrInternal)
	require.NotErrorIs(t, err, ticket.ErrTicketInvalid)
	require.Nil(t, resp)
}

func TestIssueSuccess(t *testing.T) {
	hubDescriptor := validHub()
	originalHubKey := bytes.Clone(hubDescriptor.PublicKey)
	identityPub := newIdentityPub(t)
	wire := []byte("signed ticket")

	f := newFixture(t, hubDescriptor)
	f.expectRead(
		&issueticket.DeviceMeta{
			UserID:      testUserID,
			UserState:   user.StateActive,
			State:       device.StateActive,
			IdentityPub: identityPub,
		},
		nil,
	)

	var signedInfo ticket.Info
	f.ticketSigner.EXPECT().
		Sign(gomock.Any()).
		DoAndReturn(func(info *ticket.Info) ([]byte, error) {
			signedInfo = *info
			return wire, nil
		})

	before := time.Now().Unix()
	resp, err := f.issuer.IssueTicket(context.Background(), testUserID, issueticket.Request{DeviceID: "21"})
	after := time.Now().Unix()
	require.NoError(t, err)
	require.NotNil(t, resp)

	// The claims that the signer receives.
	require.Equal(t, testDeviceID, signedInfo.DeviceID)
	require.NotEqual(t, [ticket.IDSize]byte{}, signedInfo.TicketID)
	require.GreaterOrEqual(t, signedInfo.IssuedAt, before)
	require.LessOrEqual(t, signedInfo.IssuedAt, after)
	require.Equal(t, int64(ticket.MaxTTL/time.Second), signedInfo.ExpiresAt-signedInfo.IssuedAt)
	require.Equal(t, identityPub, signedInfo.IdentityPub[:])
	require.Equal(t, originalHubKey, signedInfo.HubPublicKey[:])
	require.Equal(t, "xchat-hub", signedInfo.HubAudience)
	require.Equal(t, uint32(7), signedInfo.HubKeyID)
	require.Equal(t, hub.WireVersion, signedInfo.HubWireVersion)

	// The response.
	require.Equal(t, wire, resp.Ticket)
	wantHub := validHub()
	wantHub.PublicKey = originalHubKey
	require.Equal(t, wantHub, resp.Hub)

	// The response shares the key with the descriptor of New. The use case
	// does not copy it for each request. See Response.Hub.
	require.Same(t, &hubDescriptor.PublicKey[0], &resp.Hub.PublicKey[0])
}
