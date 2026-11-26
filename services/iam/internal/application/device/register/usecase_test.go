/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package register

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	registermocks "xchat/services/iam/internal/application/device/register/mocks"
	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/domain/user"
	"xchat/services/internal/shared/ticket"

	"xchat/pkg/database/sql"
	sqlmocks "xchat/pkg/database/sql/mocks"
	generatormocks "xchat/pkg/generator/mocks"
	"xchat/pkg/log"
	"xchat/pkg/log/logtest"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDeviceRegisterValidate(t *testing.T) {
	t.Parallel()

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	peerErr := errors.New("invalid peer public key")

	for _, test := range []struct {
		name       string
		userID     user.ID
		prepare    func(req *Request)
		deriverErr error
		wantErr    error
		// wantCause is the error that wantErr wraps. Optional.
		wantCause error
	}{
		{
			name:   "valid",
			userID: 1,
		},
		{
			name:    "empty user ID",
			userID:  0,
			wantErr: ErrRequestInvalid,
		},
		{
			name:   "empty username",
			userID: 1,
			prepare: func(req *Request) {
				req.Username = ""
			},
			wantErr: ErrRequestInvalid,
		},
		{
			name:   "empty password",
			userID: 1,
			prepare: func(req *Request) {
				req.Password = nil
			},
			wantErr: ErrRequestInvalid,
		},
		{
			name:   "short password",
			userID: 1,
			prepare: func(req *Request) {
				req.Password = bytes.Repeat([]byte{'p'}, sha256.Size-1)
			},
			wantErr: ErrRequestInvalid,
		},
		{
			name:   "long password",
			userID: 1,
			prepare: func(req *Request) {
				req.Password = bytes.Repeat([]byte{'p'}, sha256.Size+1)
			},
			wantErr: ErrRequestInvalid,
		},
		{
			name:   "long phone hash",
			userID: 1,
			prepare: func(req *Request) {
				req.PhoneHash = bytes.Repeat([]byte{'h'}, user.MaxPhoneHashLength+1)
			},
			wantErr: ErrRequestInvalid,
		},
		{
			name:   "zero phone tail",
			userID: 1,
			prepare: func(req *Request) {
				req.PhoneTail = 0
			},
			wantErr: ErrRequestInvalid,
		},
		{
			name:   "long device name",
			userID: 1,
			prepare: func(req *Request) {
				req.DeviceName = strings.Repeat("d", device.MaxNameLength+1)
			},
			wantErr: ErrRequestInvalid,
		},
		{
			name:   "invalid identity public key",
			userID: 1,
			prepare: func(req *Request) {
				req.DeviceIdentityPub = nil
			},
			wantErr:   ErrRequestInvalid,
			wantCause: ticket.ErrIdentityPubInvalid,
		},
		{
			name:       "invalid peer public key",
			userID:     1,
			deriverErr: peerErr,
			wantErr:    ErrRequestInvalid,
			wantCause:  peerErr,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			keyDeriver := registermocks.NewMockKeyDeriver(gomock.NewController(t))
			keyDeriver.EXPECT().
				ValidatePeerPub(gomock.Any()).
				Return(test.deriverErr).
				AnyTimes()

			req := validRegisterRequest(publicKey)
			if test.prepare != nil {
				test.prepare(req)
			}

			register := Usecase{keyDeriver: keyDeriver}
			validationErr := register.validateInput(test.userID, req)
			if test.wantErr == nil {
				require.NoError(t, validationErr)
				return
			}
			require.ErrorIs(t, validationErr, test.wantErr)
			if test.wantCause != nil {
				require.ErrorIs(t, validationErr, test.wantCause)
			}
		})
	}
}

// TestDeviceRegisterRegisterRejectsInvalidKey checks that Register gives
// ErrRequestInvalid for a key that is not valid. The user reader has no
// expectation, so a read of the user fails the test.
func TestDeviceRegisterRegisterRejectsInvalidKey(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	f := newRegisterFixture(t, nil, nil)
	req := validRegisterRequest(publicKey)
	req.DeviceIdentityPub = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	resp, err := f.register.Register(context.Background(), 1, req)
	require.ErrorIs(t, err, ErrRequestInvalid)
	require.ErrorIs(t, err, ticket.ErrIdentityPubInvalid)
	require.Nil(t, resp)
}

// TestDeviceRegisterRegisterExistingUserCredentialError checks that Register
// maps each error of the credential check of an existing user one time. The
// transaction does not run, so the use case writes no device.
func TestDeviceRegisterRegisterExistingUserCredentialError(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	req := validRegisterRequest(publicKey)
	storedUser := &user.Info{
		ID:        1,
		Username:  req.Username,
		PhoneHash: req.PhoneHash,
		PhoneTail: req.PhoneTail,
		State:     user.StateActive,
	}
	otherUsername := *storedUser
	otherUsername.Username = "other"

	for _, test := range []struct {
		name       string
		storedUser *user.Info
		verifyErr  error
		wantErr    error
	}{
		{
			name:       "password mismatch",
			storedUser: storedUser,
			verifyErr:  ErrPasswordMismatch,
			wantErr:    ErrRequestInvalid,
		},
		{
			name:       "password hasher busy",
			storedUser: storedUser,
			verifyErr:  fmt.Errorf("%w: %w", ErrPasswordHasherBusy, context.DeadlineExceeded),
			wantErr:    ErrBusy,
		},
		{
			name:       "username mismatch",
			storedUser: &otherUsername,
			wantErr:    ErrRequestInvalid,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRegisterFixture(t, nil, nil)
			f.expectRead(false, test.storedUser, nil)
			f.passwordHasher.EXPECT().
				Verify(gomock.Any(), req.Password, test.storedUser.PasswordHash).
				Return(test.verifyErr)

			resp, err := f.register.Register(context.Background(), 1, req)
			require.ErrorIs(t, err, test.wantErr)
			require.NotErrorIs(t, err, ErrInternal)
			require.Nil(t, resp)
			require.Empty(t, f.devices)
		})
	}
}

func validRegisterRequest(publicKey ed25519.PublicKey) *Request {
	return &Request{
		Username:          "username",
		Password:          bytes.Repeat([]byte{'p'}, sha256.Size),
		PhoneHash:         bytes.Repeat([]byte{'h'}, user.MaxPhoneHashLength),
		PhoneTail:         123,
		DeviceName:        "device",
		DeviceIdentityPub: publicKey,
		DevicePeerPub:     bytes.Repeat([]byte{1}, 32),
	}
}

// The tests below use a logger that writes to a file. Therefore, they must not
// call t.Parallel.

func TestDeviceRegisterValidateUserPassword(t *testing.T) {
	logger, closeFn, _ := logtest.New(t, log.Config{})
	t.Cleanup(func() { _ = closeFn() })

	storedUser := user.Info{
		Username:  "username",
		PhoneHash: bytes.Repeat([]byte{'h'}, user.MaxPhoneHashLength),
		PhoneTail: 123,
	}
	cause := errors.New("bcrypt: hash is corrupted")

	for _, test := range []struct {
		name      string
		verifyErr error
		wantErr   error
	}{
		{
			name: "match",
		},
		{
			name:      "mismatch",
			verifyErr: ErrPasswordMismatch,
			wantErr:   ErrRequestInvalid,
		},
		{
			name:      "busy",
			verifyErr: fmt.Errorf("%w: %w", ErrPasswordHasherBusy, context.DeadlineExceeded),
			wantErr:   ErrBusy,
		},
		{
			// A password that is too long is an error of the client, not of
			// the server.
			name:      "too long",
			verifyErr: ErrPasswordTooLong,
			wantErr:   ErrRequestInvalid,
		},
		{
			name:      "other error",
			verifyErr: cause,
			wantErr:   ErrInternal,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			passwordHasher := registermocks.NewMockPasswordHasher(gomock.NewController(t))
			passwordHasher.EXPECT().
				Verify(gomock.Any(), gomock.Any(), storedUser.PasswordHash).
				Return(test.verifyErr)

			register := Usecase{
				logger:         logger,
				passwordHasher: passwordHasher,
			}
			req := validRegisterRequest(nil)
			req.PhoneHash = storedUser.PhoneHash

			err := register.verifyCredentials(context.Background(), &storedUser, req)
			if test.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, test.wantErr)
			require.NotErrorIs(t, err, cause, "the cause must not reach the client")
			if !errors.Is(test.wantErr, ErrBusy) {
				require.NotErrorIs(t, err, ErrBusy)
			}
		})
	}
}

func TestDeviceRegisterRegisterPasswordHashError(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	req := validRegisterRequest(publicKey)
	cause := errors.New("bcrypt: internal failure")

	for _, test := range []struct {
		name    string
		hashErr error
		wantErr error
	}{
		{
			name:    "busy",
			hashErr: ErrPasswordHasherBusy,
			wantErr: ErrBusy,
		},
		{
			name:    "too long",
			hashErr: ErrPasswordTooLong,
			wantErr: ErrRequestInvalid,
		},
		{
			name:    "other error",
			hashErr: cause,
			wantErr: ErrInternal,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRegisterFixture(t, nil, nil)
			f.expectRead(false, nil, nil)
			f.passwordHasher.EXPECT().
				Hash(gomock.Any(), req.Password).
				Return(nil, test.hashErr)

			resp, regErr := f.register.Register(context.Background(), 1, req)
			require.ErrorIs(t, regErr, test.wantErr)
			require.NotErrorIs(t, regErr, cause, "the cause must not reach the client")
			require.Nil(t, resp)
			require.Empty(t, f.users, "no user must be written after a hash error")
			require.Empty(t, f.devices)
		})
	}
}

func TestDeviceRegisterCreateUser(t *testing.T) {
	f := newRegisterFixture(t, nil, nil)
	req := validRegisterRequest(nil)

	require.NoError(t, f.register.createUser(context.Background(), 7, []byte("hash"), req))
	require.Len(t, f.users, 1)
	require.Equal(t, user.ID(7), f.users[0].ID)
	require.Equal(t, []byte("hash"), f.users[0].PasswordHash)
}

type registerFixture struct {
	register       *Usecase
	userReader     *registermocks.MockUserReader
	userWriter     *registermocks.MockUserWriter
	deviceWriter   *registermocks.MockDeviceWriter
	passwordHasher *registermocks.MockPasswordHasher
	keyDeriver     *registermocks.MockKeyDeriver
	intGenerator   *generatormocks.MockInteger

	users   []user.Info
	devices []device.Info

	// commitErr is the error that the transaction mock returns after the
	// callback succeeds. It simulates an error at commit.
	commitErr error
	logger    *log.Logger
	logPath   string
}

// newRegisterFixture makes a Usecase with mocks. The transaction and
// WithDB mocks run the callback immediately. The fixture records each user and
// device that the use case writes.
func newRegisterFixture(t *testing.T, userWriterErr, deviceWriterErr error) *registerFixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	logger, closeFn, logPath := logtest.New(t, log.Config{})
	t.Cleanup(func() { _ = closeFn() })

	f := &registerFixture{
		userReader:     registermocks.NewMockUserReader(ctrl),
		userWriter:     registermocks.NewMockUserWriter(ctrl),
		deviceWriter:   registermocks.NewMockDeviceWriter(ctrl),
		passwordHasher: registermocks.NewMockPasswordHasher(ctrl),
		keyDeriver:     registermocks.NewMockKeyDeriver(ctrl),
		intGenerator:   generatormocks.NewMockInteger(ctrl),
		logger:         logger,
		logPath:        logPath,
	}

	db := sqlmocks.NewMockDB(ctrl)
	db.EXPECT().
		WithDB(gomock.Any(), sql.TxSourcePrimary, gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ sql.TxSource, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()
	db.EXPECT().
		Transaction(gomock.Any(), sql.TxSourcePrimary, gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ sql.TxSource, fn func(context.Context) error) error {
			err := fn(ctx)
			if err != nil {
				return err
			}
			return f.commitErr
		}).
		AnyTimes()

	f.keyDeriver.EXPECT().
		ValidatePeerPub(gomock.Any()).
		Return(nil).
		AnyTimes()
	f.intGenerator.EXPECT().
		Next().
		Return(uint64(42), nil).
		AnyTimes()
	f.userWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, info user.Info) error {
			f.users = append(f.users, info)
			return userWriterErr
		}).
		AnyTimes()
	f.deviceWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, info device.Info) error {
			f.devices = append(f.devices, info)
			return deviceWriterErr
		}).
		AnyTimes()

	f.register = New(
		db,
		logger,
		f.intGenerator,
		f.keyDeriver,
		f.passwordHasher,
		f.userReader,
		f.userWriter,
		f.deviceWriter,
	)
	return f
}

func (f *registerFixture) expectRead(locking bool, info *user.Info, err error) {
	f.userReader.EXPECT().
		Get(gomock.Any(), gomock.Any(), locking).
		Return(info, err)
}

func TestDeviceRegisterRegisterLocksExistingUser(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	req := validRegisterRequest(publicKey)
	activeUser := &user.Info{
		ID:        1,
		Username:  req.Username,
		PhoneHash: req.PhoneHash,
		PhoneTail: req.PhoneTail,
		State:     user.StateActive,
	}

	for _, test := range []struct {
		name           string
		locked         *user.Info
		wantErr        error
		wantDeviceSize int
	}{
		{
			name:           "user is under the limit",
			locked:         activeUser,
			wantDeviceSize: 1,
		},
		{
			// The first read does not see the device that a parallel request
			// added. The locking read sees it.
			name: "locking read reaches the device limit",
			locked: &user.Info{
				ID:         1,
				Username:   req.Username,
				PhoneHash:  req.PhoneHash,
				PhoneTail:  req.PhoneTail,
				State:      user.StateActive,
				NumDevices: 5,
			},
			wantErr: ErrUserReachMaxDevices,
		},
		{
			name: "locking read finds a locked user",
			locked: &user.Info{
				ID:        1,
				Username:  req.Username,
				PhoneHash: req.PhoneHash,
				PhoneTail: req.PhoneTail,
				State:     user.StateLocked,
			},
			wantErr: ErrUserLocked,
		},
		{
			name:    "user disappears before the locking read",
			locked:  nil,
			wantErr: ErrBusy,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRegisterFixture(t, nil, nil)
			f.expectRead(false, activeUser, nil)
			f.expectRead(true, test.locked, nil)
			f.passwordHasher.EXPECT().
				Verify(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil)

			resp, regErr := f.register.Register(context.Background(), 1, req)
			require.Len(t, f.devices, test.wantDeviceSize)
			if test.wantErr != nil {
				require.ErrorIs(t, regErr, test.wantErr)
				require.Nil(t, resp)
				return
			}
			require.NoError(t, regErr)
			require.Equal(t, device.ID(42).String(), resp.DeviceID)
		})
	}
}

func TestDeviceRegisterRegisterNewUser(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	req := validRegisterRequest(publicKey)
	lockConflict := fmt.Errorf("%w: mysql error 1205", sql.ErrLockConflict)

	for _, test := range []struct {
		name          string
		userWriterErr error
		wantErr       error
	}{
		{
			name: "creates user without a locking read",
		},
		{
			name:          "parallel request created the user first",
			userWriterErr: fmt.Errorf("%w: duplicate entry", user.ErrAlreadyExists),
			wantErr:       ErrBusy,
		},
		{
			name:          "user insert has a lock conflict",
			userWriterErr: lockConflict,
			wantErr:       ErrBusy,
		},
		{
			name:          "another user has the same username or phone",
			userWriterErr: fmt.Errorf("%w: duplicate entry", user.ErrUniqueConflict),
			wantErr:       ErrUserExists,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRegisterFixture(t, test.userWriterErr, nil)
			f.expectRead(false, nil, nil)
			f.passwordHasher.EXPECT().
				Hash(gomock.Any(), req.Password).
				Return([]byte("hash"), nil)

			resp, regErr := f.register.Register(context.Background(), 1, req)
			if test.wantErr != nil {
				require.ErrorIs(t, regErr, test.wantErr)
				require.Nil(t, resp)
				require.Empty(t, f.devices, "no device must be written after a user conflict")
				return
			}
			require.NoError(t, regErr)
			require.Len(t, f.users, 1)
			require.Len(t, f.devices, 1)
			require.Equal(t, device.ID(42).String(), resp.DeviceID)
		})
	}
}

func TestDeviceRegisterRegisterReadError(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	req := validRegisterRequest(publicKey)
	activeUser := &user.Info{
		ID:        1,
		Username:  req.Username,
		PhoneHash: req.PhoneHash,
		PhoneTail: req.PhoneTail,
		State:     user.StateActive,
	}
	cause := errors.New("driver: connection reset")
	lockConflict := fmt.Errorf("%w: mysql error 1205", sql.ErrLockConflict)

	for _, test := range []struct {
		name       string
		firstErr   error
		lockingErr error
		wantErr    error
	}{
		{
			name:     "first read has a lock conflict",
			firstErr: lockConflict,
			wantErr:  ErrBusy,
		},
		{
			name:     "first read fails",
			firstErr: cause,
			wantErr:  ErrInternal,
		},
		{
			name:       "locking read has a lock conflict",
			lockingErr: lockConflict,
			wantErr:    ErrBusy,
		},
		{
			name:       "locking read fails",
			lockingErr: cause,
			wantErr:    ErrInternal,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRegisterFixture(t, nil, nil)
			if test.firstErr != nil {
				f.expectRead(false, nil, test.firstErr)
			} else {
				f.expectRead(false, activeUser, nil)
				f.passwordHasher.EXPECT().
					Verify(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				f.expectRead(true, nil, test.lockingErr)
			}

			resp, regErr := f.register.Register(context.Background(), 1, req)
			require.ErrorIs(t, regErr, test.wantErr)
			require.NotErrorIs(t, regErr, cause, "the cause must not reach the client")
			require.Nil(t, resp)
			require.Empty(t, f.devices)
		})
	}
}

func TestDeviceRegisterRegisterDeviceError(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	req := validRegisterRequest(publicKey)
	activeUser := &user.Info{
		ID:        1,
		Username:  req.Username,
		PhoneHash: req.PhoneHash,
		PhoneTail: req.PhoneTail,
		State:     user.StateActive,
	}
	cause := errors.New("driver: connection reset")

	for _, test := range []struct {
		name      string
		deviceErr error
		wantErr   error
	}{
		{
			name:      "device is registered before",
			deviceErr: fmt.Errorf("%w: duplicate entry", device.ErrAlreadyExists),
			wantErr:   ErrDeviceExists,
		},
		{
			name:      "device insert has a lock conflict",
			deviceErr: fmt.Errorf("%w: mysql error 1205", sql.ErrLockConflict),
			wantErr:   ErrBusy,
		},
		{
			name:      "other error",
			deviceErr: cause,
			wantErr:   ErrInternal,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRegisterFixture(t, nil, test.deviceErr)
			f.expectRead(false, activeUser, nil)
			f.expectRead(true, activeUser, nil)
			f.passwordHasher.EXPECT().
				Verify(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil)

			resp, regErr := f.register.Register(context.Background(), 1, req)
			require.ErrorIs(t, regErr, test.wantErr)
			require.NotErrorIs(t, regErr, cause, "the cause must not reach the client")
			require.Nil(t, resp)
		})
	}
}

// TestDeviceRegisterMapReadErrorLogLevel checks the result and the level of
// the log entry for a read error. A request that the client canceled is not a
// server error. An expired deadline makes the server busy, and it is not
// logged.
func TestDeviceRegisterMapReadErrorLogLevel(t *testing.T) {
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
			wantErr:   ErrInternal,
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
			wantErr:   ErrInternal,
			wantError: false,
		},
		{
			name: "server deadline",
			makeCtx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			readErr:   context.DeadlineExceeded,
			wantErr:   ErrBusy,
			wantError: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger, closeFn, logPath := logtest.New(t, log.Config{})
			t.Cleanup(func() { _ = closeFn() })
			ctx, cancel := tc.makeCtx()
			defer cancel()

			usecase := &Usecase{logger: logger}
			err := usecase.mapReadError(ctx, tc.readErr)
			require.ErrorIs(t, err, tc.wantErr)

			require.NoError(t, logger.Sync())
			raw, err := os.ReadFile(logPath)
			require.NoError(t, err)
			if tc.wantError {
				require.Contains(t, string(raw), `"level":"error"`)
				require.Contains(t, string(raw), `"msg":"failed to read user"`)
			} else {
				require.NotContains(t, string(raw), `"level":"error"`)
			}
		})
	}
}

// TestDeviceRegisterRegisterTransactionError checks an error that the
// transaction returns after the callback succeeds, for example at commit. The
// cause must not reach the client.
func TestDeviceRegisterRegisterTransactionError(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	req := validRegisterRequest(publicKey)
	cause := errors.New("driver: bad connection at commit")

	for _, test := range []struct {
		name      string
		commitErr error
		wantErr   error
		wantLog   bool
	}{
		{
			name:      "commit has a lock conflict",
			commitErr: fmt.Errorf("%w: mysql error 1213", sql.ErrLockConflict),
			wantErr:   ErrBusy,
			wantLog:   false,
		},
		{
			name:      "commit fails",
			commitErr: cause,
			wantErr:   ErrInternal,
			wantLog:   true,
		},
		{
			name:      "database is closed",
			commitErr: sql.ErrDBClosed,
			wantErr:   ErrInternal,
			wantLog:   true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRegisterFixture(t, nil, nil)
			f.commitErr = test.commitErr
			f.expectRead(false, nil, nil)
			f.passwordHasher.EXPECT().
				Hash(gomock.Any(), req.Password).
				Return([]byte("hash"), nil)

			resp, regErr := f.register.Register(context.Background(), 1, req)
			require.ErrorIs(t, regErr, test.wantErr)
			require.NotErrorIs(t, regErr, test.commitErr, "the cause must not reach the client")
			require.Nil(t, resp)

			require.NoError(t, f.logger.Sync())
			raw, err := os.ReadFile(f.logPath)
			require.NoError(t, err)
			if test.wantLog {
				require.Contains(t, string(raw), `"level":"error"`)
				require.Contains(t, string(raw), `"msg":"failed to run the register transaction"`)
			} else {
				require.NotContains(t, string(raw), `"level":"error"`)
			}
		})
	}
}

// TestDeviceRegisterRegisterDeadline checks an error of the database after the
// deadline of the request expired. The server is busy, so the client gets
// ErrBusy and no error is logged. An error that the use case defines, such as a
// duplicate device, still has priority.
func TestDeviceRegisterRegisterDeadline(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	req := validRegisterRequest(publicKey)
	activeUser := &user.Info{
		ID:        1,
		Username:  req.Username,
		PhoneHash: req.PhoneHash,
		PhoneTail: req.PhoneTail,
		State:     user.StateActive,
	}
	driverErr := errors.New("driver: invalid connection")

	for _, test := range []struct {
		name            string
		existingUser    bool
		firstReadErr    error
		userWriterErr   error
		deviceWriterErr error
		commitErr       error
		wantErr         error
	}{
		{
			name:         "first read fails",
			firstReadErr: context.DeadlineExceeded,
			wantErr:      ErrBusy,
		},
		{
			name:          "user insert fails",
			userWriterErr: driverErr,
			wantErr:       ErrBusy,
		},
		{
			name:            "device insert fails",
			existingUser:    true,
			deviceWriterErr: driverErr,
			wantErr:         ErrBusy,
		},
		{
			name:      "commit fails",
			commitErr: context.DeadlineExceeded,
			wantErr:   ErrBusy,
		},
		{
			name:            "duplicate device has priority",
			existingUser:    true,
			deviceWriterErr: fmt.Errorf("%w: duplicate entry", device.ErrAlreadyExists),
			wantErr:         ErrDeviceExists,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRegisterFixture(t, test.userWriterErr, test.deviceWriterErr)
			f.commitErr = test.commitErr
			switch {
			case test.firstReadErr != nil:
				f.expectRead(false, nil, test.firstReadErr)
			case test.existingUser:
				f.expectRead(false, activeUser, nil)
				f.passwordHasher.EXPECT().
					Verify(gomock.Any(), req.Password, activeUser.PasswordHash).
					Return(nil)
				f.expectRead(true, activeUser, nil)
			default:
				f.expectRead(false, nil, nil)
				f.passwordHasher.EXPECT().
					Hash(gomock.Any(), req.Password).
					Return([]byte("hash"), nil)
			}

			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()

			resp, regErr := f.register.Register(ctx, 1, req)
			require.ErrorIs(t, regErr, test.wantErr)
			require.NotErrorIs(t, regErr, driverErr, "the cause must not reach the client")
			require.Nil(t, resp)

			require.NoError(t, f.logger.Sync())
			raw, err := os.ReadFile(f.logPath)
			require.NoError(t, err)
			require.NotContains(t, string(raw), `"level":"error"`)
		})
	}
}
