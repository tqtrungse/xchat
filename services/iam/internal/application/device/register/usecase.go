/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package register

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"

	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/domain/user"
	"xchat/services/internal/shared/ticket"

	"xchat/pkg/bytesconv"
	"xchat/pkg/database/sql"
	"xchat/pkg/generator"
	"xchat/pkg/log"

	"go.uber.org/zap"
)

const maxInputPasswordLength = 32

// Usecase registers a device for a user. It creates the user if the user
// does not exist. For an existing user, it checks the credentials, the state,
// and the number of devices before it adds the device.
type Usecase struct {
	db             sql.DB
	logger         *log.Logger
	intGenerator   generator.Integer
	keyDeriver     KeyDeriver
	passwordHasher PasswordHasher
	userReader     UserReader
	userWriter     UserWriter
	deviceWriter   DeviceWriter
}

// New creates a Usecase. All arguments are required.
func New(
	db sql.DB,
	log *log.Logger,
	intGenerator generator.Integer,
	keyDeriver KeyDeriver,
	passwordHasher PasswordHasher,
	userReader UserReader,
	userWriter UserWriter,
	deviceWriter DeviceWriter,
) *Usecase {
	return &Usecase{
		db:             db,
		logger:         log,
		intGenerator:   intGenerator,
		keyDeriver:     keyDeriver,
		passwordHasher: passwordHasher,
		userReader:     userReader,
		userWriter:     userWriter,
		deviceWriter:   deviceWriter,
	}
}

// Register registers a device for the user with ID userID. It reads the user
// before the transaction. Then it runs the writes and the locking check in one
// transaction.
//
// Register returns ErrRequestInvalid if the input or the credentials are not
// valid. It returns ErrUserLocked, ErrUserReachMaxDevices, ErrUserExists, or
// ErrDeviceExists if the data does not permit the request. It returns ErrBusy
// if a new try can succeed. It returns ErrInternal for any other failure.
func (u *Usecase) Register(
	ctx context.Context,
	userID user.ID,
	req *Request,
) (*Response, error) {
	err := u.validateInput(userID, req)
	if err != nil {
		return nil, err
	}

	id, err := u.intGenerator.Next()
	if err != nil || id == 0 {
		u.logger.Error("failed to generate ID", zap.Error(err))
		return nil, ErrInternal
	}

	var (
		storedUser   *user.Info
		passwordHash []byte
	)

	err = u.db.WithDB(ctx, sql.TxSourcePrimary, func(dbCtx context.Context) error {
		storedUser, err = u.userReader.Get(dbCtx, userID, false)
		return err
	})
	if err != nil {
		return nil, u.mapReadError(ctx, err)
	}

	if storedUser == nil {
		passwordHash, err = u.passwordHasher.Hash(ctx, req.Password)
	} else {
		err = u.verifyCredentials(ctx, storedUser, req)
	}
	if err != nil {
		return nil, u.mapPasswordError(err)
	}

	// The callback returns only errors of this use case. Transaction can also
	// return an error of the database, for example if it cannot begin or commit
	// the transaction. mappedErr keeps the error of the callback, so that the
	// two kinds stay separate.
	var mappedErr error
	err = u.db.Transaction(ctx, sql.TxSourcePrimary, func(dbCtx context.Context) error {
		if storedUser == nil {
			mappedErr = u.createUser(dbCtx, userID, passwordHash, req)
		} else {
			mappedErr = u.lockAndCheck(dbCtx, userID, storedUser)
		}
		if mappedErr != nil {
			return mappedErr
		}
		mappedErr = u.createDevice(dbCtx, userID, device.ID(id), req)
		return mappedErr
	})
	if mappedErr != nil {
		return nil, mappedErr
	}
	if err != nil {
		return nil, u.mapTransactionError(ctx, err)
	}
	return &Response{
		DeviceID: device.ID(id).String(),
	}, nil
}

// mapTransactionError converts an error of the database transaction to an
// error of this use case. A lock conflict, for example a deadlock at commit, is
// temporary. The client can try again. Any other error is a server error.
// mapTransactionError logs it and hides the cause from the client.
func (u *Usecase) mapTransactionError(ctx context.Context, err error) error {
	if sql.IsLockConflict(err) {
		return ErrBusy
	}
	u.logError(ctx, "failed to run the register transaction", err)
	return ErrInternal
}

func (u *Usecase) createUser(
	dbCtx context.Context,
	userID user.ID,
	passwordHash []byte,
	req *Request,
) error {
	err := u.userWriter.Create(dbCtx, user.Info{
		ID:           userID,
		Username:     req.Username,
		PasswordHash: passwordHash,
		PhoneHash:    req.PhoneHash,
		PhoneTail:    req.PhoneTail,
		State:        user.StateActive,
	})
	if err == nil {
		return nil
	}
	if sql.IsLockConflict(err) {
		return ErrBusy
	}
	// Another request created the user with the same ID after the first
	// read. A new try finds the user and can succeed.
	if errors.Is(err, user.ErrAlreadyExists) {
		return ErrBusy
	}
	// Another user has the same username or phone hash. A new try cannot
	// succeed.
	if errors.Is(err, user.ErrUniqueConflict) {
		return ErrUserExists
	}
	u.logError(dbCtx, "failed to register user", err)
	return ErrInternal
}

func (u *Usecase) createDevice(
	dbCtx context.Context,
	userID user.ID,
	deviceID device.ID,
	req *Request,
) error {
	err := u.deviceWriter.Create(dbCtx, device.Info{
		ID:          deviceID,
		UserID:      userID,
		Name:        req.DeviceName,
		State:       device.StateActive,
		IdentityPub: req.DeviceIdentityPub,
		PeerPub:     req.DevicePeerPub,
	})
	if err == nil {
		return nil
	}
	if sql.IsLockConflict(err) {
		return ErrBusy
	}
	// A device with the same unique value, such as the identity public
	// key, exists. A new try cannot succeed.
	if errors.Is(err, device.ErrAlreadyExists) {
		return ErrDeviceExists
	}
	u.logError(dbCtx, "failed to register device", err)
	return ErrInternal
}

func (u *Usecase) verifyCredentials(
	ctx context.Context,
	storedUser *user.Info,
	req *Request,
) error {
	// The password check always runs, even if the username or the phone is
	// wrong. Therefore, the response time does not show the cause.
	verifyErr := u.passwordHasher.Verify(ctx, req.Password, storedUser.PasswordHash)

	identityOK := subtle.ConstantTimeCompare(bytesconv.StringToBytes(req.Username), bytesconv.StringToBytes(storedUser.Username)) == 1 &&
		subtle.ConstantTimeCompare(req.PhoneHash, storedUser.PhoneHash) == 1 &&
		req.PhoneTail == storedUser.PhoneTail

	if verifyErr != nil {
		return u.mapPasswordError(verifyErr)
	}
	if !identityOK {
		return ErrRequestInvalid
	}
	return nil
}

func (u *Usecase) lockAndCheck(
	dbCtx context.Context,
	userID user.ID,
	verified *user.Info,
) error {
	locked, err := u.userReader.Get(dbCtx, userID, true)
	if err != nil {
		return u.mapReadError(dbCtx, err)
	}
	if locked == nil {
		return ErrBusy
	}

	// The credentials changed after the verification. Therefore, the result of
	// the verification is not valid.
	if !bytes.Equal(locked.PasswordHash, verified.PasswordHash) ||
		locked.Username != verified.Username ||
		!bytes.Equal(locked.PhoneHash, verified.PhoneHash) ||
		locked.PhoneTail != verified.PhoneTail {
		return ErrBusy
	}
	if locked.State == user.StateLocked {
		return ErrUserLocked
	}
	if locked.ReachMaxDevices() {
		return ErrUserReachMaxDevices
	}
	return nil
}

// mapReadError converts an error of the user reader to an error of this use
// case. A lock conflict is temporary. The client can try again. Any other error
// is a server error. mapReadError logs it and hides the cause from the client.
func (u *Usecase) mapReadError(ctx context.Context, err error) error {
	if sql.IsLockConflict(err) {
		return ErrBusy
	}
	u.logError(ctx, "failed to read user", err)
	return ErrInternal
}

// logError logs err at the error level. If the client canceled the request,
// logError logs err at the debug level, because the failure is then not a
// server error. A deadline of the server is a server error, so it stays at the
// error level.
func (u *Usecase) logError(ctx context.Context, message string, err error) {
	if errors.Is(ctx.Err(), context.Canceled) {
		u.logger.Debug(message, zap.Error(err))
		return
	}
	u.logger.Error(message, zap.Error(err))
}

// mapPasswordError converts an error of the password hasher to an error of
// this use case. A wrong password is a client error. A busy hasher is a
// temporary error. The client can try again. Any other error is a server
// error. mapPasswordError logs it and hides the cause from the client.
func (u *Usecase) mapPasswordError(err error) error {
	switch {
	case errors.Is(err, ErrPasswordMismatch):
		return ErrRequestInvalid
	case errors.Is(err, ErrPasswordHasherBusy):
		return ErrBusy
	default:
		u.logger.Error("failed to hash or verify password", zap.Error(err))
		return ErrInternal
	}
}

func (u *Usecase) validateInput(userID user.ID, req *Request) error {
	if req == nil {
		return ErrRequestInvalid
	}
	if userID == 0 ||
		!isValidLength(req.Username, user.MaxUsernameLength) ||
		!isValidLength(req.Password, maxInputPasswordLength) ||
		!isValidLength(req.PhoneHash, user.MaxPhoneHashLength) ||
		!isValidLength(req.DeviceName, device.MaxNameLength) ||
		req.PhoneTail == 0 {
		return ErrRequestInvalid
	}

	err := ticket.ValidateIdentity(req.DeviceIdentityPub)
	if err != nil {
		return err
	}

	return u.keyDeriver.ValidatePeerPub(req.DevicePeerPub)
}

// isValidLength reports whether value has a length in [1, limit].
func isValidLength[T ~string | ~[]byte](value T, limit int) bool {
	length := len(value)
	return length > 0 && length <= limit
}
