/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package registerdevice

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"

	"xchat/api/iam/v1/json"
	"xchat/internal/iam/domain/user"
	"xchat/internal/shared/device"
	"xchat/internal/shared/password"
	"xchat/internal/shared/ticket"

	"xchat/pkg/database/sql"
	"xchat/pkg/generator"
	"xchat/pkg/log"
	"xchat/pkg/utils/bs"

	"go.uber.org/zap"
)

const maxInputPasswordLength = 32

// DeviceRegister registers a device for a user. It creates the user if the user
// does not exist. For an existing user, it checks the credentials, the state,
// and the number of devices before it adds the device.
type DeviceRegister struct {
	db             sql.DB
	logger         *log.Logger
	intGenerator   generator.Integer
	keyDeriver     KeyDeriver
	passwordHasher PasswordHasher
	userReader     UserReader
	userWriter     UserWriter
	deviceWriter   DeviceWriter
}

// New creates a DeviceRegister. All arguments are required.
func New(
	db sql.DB,
	log *log.Logger,
	intGenerator generator.Integer,
	keyDeriver KeyDeriver,
	passwordHasher PasswordHasher,
	userReader UserReader,
	userWriter UserWriter,
	deviceWriter DeviceWriter,
) DeviceRegister {
	return DeviceRegister{
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
func (p *DeviceRegister) Register(
	ctx context.Context,
	userID user.ID,
	req json.RegisterDeviceReq,
) (*json.RegisterDeviceResp, error) {
	err := p.validateInput(userID, &req)
	if err != nil {
		return nil, err
	}

	id, err := p.intGenerator.Next()
	if err != nil || id == 0 {
		p.logger.Error("failed to generate ID", zap.Error(err))
		return nil, ErrInternal
	}

	var (
		storedUser   *user.Info
		passwordHash []byte
	)

	err = p.db.WithDB(ctx, sql.TxSourcePrimary, func(dbCtx context.Context) error {
		storedUser, err = p.userReader.Get(dbCtx, userID, false)
		return err
	})
	if err != nil {
		return nil, p.mapReadError(err)
	}

	if storedUser == nil {
		passwordHash, err = p.passwordHasher.Hash(ctx, req.Password)
	} else {
		err = p.verifyCredentials(ctx, storedUser, &req)
	}
	if err != nil {
		return nil, p.mapPasswordError(err)
	}

	err = p.db.Transaction(ctx, sql.TxSourcePrimary, func(dbCtx context.Context) error {
		if storedUser == nil {
			err = p.createUser(dbCtx, userID, passwordHash, &req)
		} else {
			err = p.lockAndCheck(dbCtx, userID, storedUser)
		}
		if err != nil {
			return err
		}
		return p.createDevice(dbCtx, userID, device.ID(id), &req)
	})
	if err != nil {
		return nil, err
	}
	return &json.RegisterDeviceResp{
		DeviceID: device.ID(id).String(),
	}, err
}

func (p *DeviceRegister) createUser(
	dbCtx context.Context,
	userID user.ID,
	passwordHash []byte,
	req *json.RegisterDeviceReq,
) error {
	err := p.userWriter.Create(dbCtx, user.Info{
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
	p.logger.Error("failed to register user", zap.Error(err))
	return ErrInternal
}

func (p *DeviceRegister) createDevice(
	dbCtx context.Context,
	userID user.ID,
	deviceID device.ID,
	req *json.RegisterDeviceReq,
) error {
	err := p.deviceWriter.Create(dbCtx, device.Info{
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
	p.logger.Error("failed to register device", zap.Error(err))
	return ErrInternal
}

func (p *DeviceRegister) verifyCredentials(
	ctx context.Context,
	storedUser *user.Info,
	req *json.RegisterDeviceReq,
) error {
	// The password check always runs, even if the username or the phone is
	// wrong. Therefore, the response time does not show the cause.
	verifyErr := p.passwordHasher.Verify(ctx, req.Password, storedUser.PasswordHash)

	identityOK := subtle.ConstantTimeCompare(bs.StringToBytes(req.Username), bs.StringToBytes(storedUser.Username)) == 1 &&
		subtle.ConstantTimeCompare(req.PhoneHash, storedUser.PhoneHash) == 1 &&
		req.PhoneTail == storedUser.PhoneTail

	if verifyErr != nil {
		return p.mapPasswordError(verifyErr)
	}
	if !identityOK {
		return ErrRequestInvalid
	}
	return nil
}

func (p *DeviceRegister) lockAndCheck(
	dbCtx context.Context,
	userID user.ID,
	verified *user.Info,
) error {
	locked, err := p.userReader.Get(dbCtx, userID, true)
	if err != nil {
		return p.mapReadError(err)
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
func (p *DeviceRegister) mapReadError(err error) error {
	if sql.IsLockConflict(err) {
		return ErrBusy
	}
	p.logger.Error("failed to read user", zap.Error(err))
	return ErrInternal
}

// mapPasswordError converts an error of the password hasher to an error of
// this use case. A wrong password is a client error. A busy hasher is a
// temporary error. The client can try again. Any other error is a server
// error. mapPasswordError logs it and hides the cause from the client.
func (p *DeviceRegister) mapPasswordError(err error) error {
	switch {
	case errors.Is(err, password.ErrMismatch):
		return ErrRequestInvalid
	case errors.Is(err, password.ErrBusy):
		return ErrBusy
	default:
		p.logger.Error("failed to hash or verify password", zap.Error(err))
		return ErrInternal
	}
}

func (p *DeviceRegister) validateInput(userID user.ID, req *json.RegisterDeviceReq) error {
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

	return p.keyDeriver.ValidatePeerPub(req.DevicePeerPub)
}

// isValidLength reports whether value has a length in [1, limit].
func isValidLength[T ~string | ~[]byte](value T, limit int) bool {
	length := len(value)
	return length > 0 && length <= limit
}
