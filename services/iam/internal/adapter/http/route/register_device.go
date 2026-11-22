/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package route

import (
	"context"
	"errors"
	"net/http"

	"xchat/services/iam/internal/adapter/http/httpx"
	"xchat/services/iam/internal/adapter/http/rejection"
	"xchat/services/iam/internal/application/device/register"
	"xchat/services/iam/internal/domain/user"

	"xchat/pkg/log"
	slicepool "xchat/pkg/pool/slice"
	"xchat/pkg/reject"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// maxRegisterDeviceBodyBytes limits the request body. The fields of a valid
	// request, in base64, take less than 1 KiB.
	maxRegisterDeviceBodyBytes = 4 << 10

	// retryAfterBusySeconds is the value of the Retry-After header for a request
	// that the use case rejects as busy.
	retryAfterBusySeconds = "1"
)

// DeviceRegisterer is the use case that the handler of POST /api/device/register
// calls. *registerdevice.Usecase implements it.
type DeviceRegisterer interface {
	Register(
		ctx context.Context,
		userID user.ID,
		req *register.Request,
	) (*register.Response, error)
}

// RegisterDeviceHandler handles POST /api/device/register. It registers a
// device for the user in the token. The use case creates the user if it does
// not exist.
type RegisterDeviceHandler struct {
	usecase    DeviceRegisterer
	logger     *log.Logger
	rejections *reject.Stats[rejection.Reason]
}

// NewRegisterDeviceHandler makes a RegisterDeviceHandler. The caller must give all
// arguments.
func NewRegisterDeviceHandler(
	usecase DeviceRegisterer,
	logger *log.Logger,
	rejections *reject.Stats[rejection.Reason],
) *RegisterDeviceHandler {
	return &RegisterDeviceHandler{
		usecase:    usecase,
		logger:     logger,
		rejections: rejections,
	}
}

// Mount registers the route on group. group must run the authentication
// middleware, because the handler reads the claims that it attaches.
func (h *RegisterDeviceHandler) Mount(group *gin.RouterGroup) {
	group.POST("/device/register", h.handle)
}

func (h *RegisterDeviceHandler) handle(c *gin.Context) {
	userID, authenticated := requireAuthenticatedUser(c, h.rejections)
	if !authenticated {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRegisterDeviceBodyBytes)

	var req register.Request
	defer func() {
		slicepool.PutZeroed(req.Password)
		slicepool.PutZeroed(req.PhoneHash)
		slicepool.PutZeroed(req.DevicePeerPub)
		slicepool.PutZeroed(req.DeviceIdentityPub)
	}()

	if !decodeRequestBody(c, h.rejections, &req) {
		return
	}

	resp, err := h.usecase.Register(c.Request.Context(), user.ID(userID), &req)
	if err != nil {
		respondUsecaseFailure(
			c,
			h.logger,
			h.rejections,
			"failed to register device",
			userID,
			err,
			mapRegisterDeviceError(err),
		)
		return
	}

	err = httpx.RespondSuccess(c, http.StatusCreated, resp)
	switch {
	case err == nil:

	case httpx.IsClientGone(c, err):
		h.rejections.Record(rejection.ResponseWrite, err)
		// The device exists, but the client may not have the ID. The client
		// that tries again gets a conflict, without the ID. The log helps to
		// find the device afterward.
		h.logger.Warn(
			"client left before the response of register device",
			zap.Uint64("user_id", userID),
			zap.String("device_id", resp.DeviceID),
		)

	default:
		// The encoding failed, or write failed for a reason that is not the
		// client.
		h.logger.Error(
			"failed to respond to register device",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
	}
}

// mapRegisterDeviceError converts an error of the use case to a response.
//
// A wrong credential and an invalid field give the same response. A client
// cannot learn from the response whether the user exists.
func mapRegisterDeviceError(err error) usecaseFailure {
	switch {
	case errors.Is(err, register.ErrRequestInvalid):
		return usecaseFailure{
			status:      http.StatusBadRequest,
			code:        "invalid_request",
			description: register.ErrRequestInvalid.Error(),
			reason:      register.ErrRequestInvalid,
		}

	case errors.Is(err, register.ErrUserLocked):
		return usecaseFailure{
			status:      http.StatusForbidden,
			code:        "user_locked",
			description: register.ErrUserLocked.Error(),
		}

	case errors.Is(err, register.ErrUserReachMaxDevices):
		return usecaseFailure{
			status:      http.StatusForbidden,
			code:        "max_devices_reached",
			description: register.ErrUserReachMaxDevices.Error(),
		}

	case errors.Is(err, register.ErrUserExists):
		return usecaseFailure{
			status:      http.StatusConflict,
			code:        "user_exists",
			description: register.ErrUserExists.Error(),
		}

	case errors.Is(err, register.ErrDeviceExists):
		return usecaseFailure{
			status:      http.StatusConflict,
			code:        "device_exists",
			description: register.ErrDeviceExists.Error(),
		}

	case errors.Is(err, register.ErrBusy):
		return usecaseFailure{
			status:      http.StatusServiceUnavailable,
			code:        "busy",
			description: register.ErrBusy.Error(),
			retryAfter:  retryAfterBusySeconds,
		}

	case errors.Is(err, register.ErrInternal):
		return usecaseFailure{
			status:      http.StatusInternalServerError,
			code:        "internal_error",
			description: httpx.ErrInternal.Error(),
		}

	default:
		return usecaseFailure{
			status:      http.StatusInternalServerError,
			code:        "internal_error",
			description: httpx.ErrInternal.Error(),
			unexpected:  true,
		}
	}
}
