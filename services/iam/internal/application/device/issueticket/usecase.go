/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package issueticket

import (
	"context"
	"crypto/rand"
	"errors"
	"strconv"
	"time"

	"xchat/services/iam/internal/domain/device"
	"xchat/services/iam/internal/domain/user"
	"xchat/services/internal/shared/hub"
	"xchat/services/internal/shared/ticket"

	"xchat/pkg/database/sql"
	"xchat/pkg/log"

	"go.uber.org/zap"
)

// Usecase issues a short-lived ticket. A device sends the ticket to the
// hub to connect.
type Usecase struct {
	db           sql.DB
	logger       *log.Logger
	deviceReader DeviceReader
	ticketSigner TicketSigner
	// hub.PublicKey is shared with each Response. Nothing changes it after New.
	hub hub.Descriptor
}

// New creates a Usecase. All arguments are required. The caller must
// validate hubDescriptor before it calls New. The bootstrap does this when it
// loads the config.
//
// New does not copy hubDescriptor.PublicKey. The caller must not change these
// bytes after New, because each Response shares them.
func New(
	db sql.DB,
	logger *log.Logger,
	deviceReader DeviceReader,
	ticketSigner TicketSigner,
	hubDescriptor hub.Descriptor,
) *Usecase {
	return &Usecase{
		db:           db,
		logger:       logger,
		deviceReader: deviceReader,
		ticketSigner: ticketSigner,
		hub:          hubDescriptor,
	}
}

// IssueTicket issues a ticket for the device in req. The device must belong to the
// user with ID userID, and the device must be active.
//
// Issue returns ErrRequestInvalid if the device ID is not valid. It returns
// ErrDeviceNotFound if the device does not exist, is deleted, or belongs to
// another user. It returns ErrDeviceInactive if the device is not active. It
// returns ErrBusy if the deadline of ctx expires before the read of the device
// ends. It returns ErrInternal for any other failure.
func (u *Usecase) IssueTicket(
	ctx context.Context,
	userID user.ID,
	req Request,
) (*Response, error) {
	id, err := strconv.ParseUint(req.DeviceID, 10, 64)
	if err != nil || id == 0 {
		return nil, ErrRequestInvalid
	}

	var deviceMeta *DeviceMeta
	err = u.db.WithDB(ctx, sql.TxSourcePrimary, func(dbCtx context.Context) error {
		deviceMeta, err = u.deviceReader.GetMeta(dbCtx, device.ID(id))
		return err
	})
	if err != nil {
		// The server is busy, and the client can try again. The access log
		// records each response status, so this case is not logged.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ErrBusy
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			u.logger.Debug("failed to read device", zap.Error(err), zap.Uint64("id", id))
		} else {
			u.logger.Error("failed to read device", zap.Error(err), zap.Uint64("id", id))
		}
		return nil, ErrInternal
	}
	if deviceMeta == nil || deviceMeta.UserID != userID {
		return nil, ErrDeviceNotFound
	}
	// The state of the user is not checked here. A lock of a user must also
	// lock each device of the user. Then a locked user cannot get a ticket.
	if deviceMeta.State != device.StateActive {
		return nil, ErrDeviceInactive
	}

	now := time.Now()
	info := &ticket.Info{
		DeviceID:       device.ID(id),
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(ticket.MaxTTL).Unix(),
		HubAudience:    u.hub.Audience,
		HubKeyID:       u.hub.KeyID,
		HubWireVersion: u.hub.Version,
	}

	// rand.Read does not return an error since Go 1.24.
	// It stops the program if the system source fails.
	_, _ = rand.Read(info.TicketID[:])

	// The register use case validates the identity public key before it stores
	// the device. The bootstrap validates the hub public key when it loads the
	// config.
	copy(info.IdentityPub[:], deviceMeta.IdentityPub)
	copy(info.HubPublicKey[:], u.hub.PublicKey)

	wire, err := u.ticketSigner.Sign(info)
	if err != nil {
		u.logger.Error("failed to sign ticket", zap.Error(err), zap.Uint64("id", id))
		return nil, ErrInternal
	}

	// Hub.PublicKey is not copied. The response shares it with u.hub. See
	// Response.Hub.
	return &Response{
		Ticket: wire,
		Hub:    u.hub,
	}, nil
}
