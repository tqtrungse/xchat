/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package route

import (
	"context"
	"errors"
	"net/http"

	"xchat/internal/iam/adapter/http/httpx"
	"xchat/internal/iam/application/registerdevice"
	"xchat/internal/iam/core/rejection"
	"xchat/internal/iam/domain/user"

	"xchat/pkg/log"
	slicepool "xchat/pkg/pool/slice"
	"xchat/pkg/reject"

	"github.com/gin-gonic/gin"
	"github.com/mailru/easyjson"
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
		req *registerdevice.Request,
	) (*registerdevice.Response, error)
}

// RegisterDeviceHandler handles POST /api/device/register. It registers a
// device for the user in the token. The use case creates the user if it does
// not exist.
type RegisterDeviceHandler struct {
	usecase    DeviceRegisterer
	logger     *log.Logger
	rejections *reject.Stats[rejection.Reason]
}

// NewRegisterDeviceHandler makes a RegisterDeviceHandler. It returns an error
// if an argument is nil.
func NewRegisterDeviceHandler(
	usecase DeviceRegisterer,
	logger *log.Logger,
	rejections *reject.Stats[rejection.Reason],
) (*RegisterDeviceHandler, error) {
	if usecase == nil {
		return nil, errUsecaseRequired
	}
	if logger == nil {
		return nil, errLoggerRequired
	}
	if rejections == nil {
		return nil, errRejectionsRequired
	}
	return &RegisterDeviceHandler{
		usecase:    usecase,
		logger:     logger,
		rejections: rejections,
	}, nil
}

// Mount registers the route on group. group must run the authentication
// middleware, because the handler reads the claims that it attaches.
func (h *RegisterDeviceHandler) Mount(group *gin.RouterGroup) {
	group.POST("/device/register", h.handle)
}

func (h *RegisterDeviceHandler) handle(c *gin.Context) {
	userID, ok := httpx.AuthenticatedUserID(c)
	if !ok {
		h.rejections.Record(rejection.AuthToken, httpx.ErrAuthVerify)
		httpx.RespondInvalidToken(c)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRegisterDeviceBodyBytes)

	var req registerdevice.Request
	defer func() {
		slicepool.PutZeroed(req.Password)
		slicepool.PutZeroed(req.PhoneHash)
		slicepool.PutZeroed(req.DevicePeerPub)
		slicepool.PutZeroed(req.DeviceIdentityPub)
	}()

	err := easyjson.UnmarshalFromReader(c.Request.Body, &req)
	if err != nil {
		// Decoder errors can contain request values; log only the rejection category.
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			h.rejections.Record(rejection.RequestBody, httpx.ErrBodyTooLarge)
			httpx.RespondError(c, http.StatusRequestEntityTooLarge, "request_too_large", httpx.ErrBodyTooLarge.Error())
			return
		}
		h.rejections.Record(rejection.RequestBody, httpx.ErrBodyInvalid)
		httpx.RespondError(c, http.StatusBadRequest, "invalid_request_body", httpx.ErrBodyInvalid.Error())
		return
	}

	resp, err := h.usecase.Register(c.Request.Context(), user.ID(userID), &req)
	if err != nil {
		h.respondUsecaseError(c, userID, err)
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

// respondUsecaseError writes the response for an error of the use case.
//
// The response never contains the text of err. The use case already logs the
// causes of ErrInternal, so only an error that the use case does not define is
// logged here.
func (h *RegisterDeviceHandler) respondUsecaseError(c *gin.Context, userID uint64, err error) {
	failure := mapRegisterDeviceError(err)

	switch {
	case failure.reason != nil:
		h.rejections.Record(rejection.RequestBody, failure.reason)

	case failure.unexpected:
		h.logger.Error(
			"failed to register device",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
	}

	if failure.retryAfter != "" {
		c.Header("Retry-After", failure.retryAfter)
	}
	httpx.RespondError(c, failure.status, failure.code, failure.description)
}

// registerDeviceFailure is the HTTP form of an error of the use case.
type registerDeviceFailure struct {
	status      int
	code        string
	description string

	// reason is not nil if the client sent a request that is not valid. The
	// handler records it in the rejection stats.
	reason error

	// retryAfter is the value of the Retry-After header. Optional.
	retryAfter string

	// unexpected is true if the use case does not define the error.
	unexpected bool
}

// mapRegisterDeviceError converts an error of the use case to a response.
//
// A wrong credential and an invalid field give the same response. A client
// cannot learn from the response whether the user exists.
func mapRegisterDeviceError(err error) registerDeviceFailure {
	switch {
	case errors.Is(err, registerdevice.ErrRequestInvalid):
		return registerDeviceFailure{
			status:      http.StatusBadRequest,
			code:        "invalid_request",
			description: registerdevice.ErrRequestInvalid.Error(),
			reason:      registerdevice.ErrRequestInvalid,
		}

	case errors.Is(err, registerdevice.ErrUserLocked):
		return registerDeviceFailure{
			status:      http.StatusForbidden,
			code:        "user_locked",
			description: registerdevice.ErrUserLocked.Error(),
		}

	case errors.Is(err, registerdevice.ErrUserReachMaxDevices):
		return registerDeviceFailure{
			status:      http.StatusForbidden,
			code:        "max_devices_reached",
			description: registerdevice.ErrUserReachMaxDevices.Error(),
		}

	case errors.Is(err, registerdevice.ErrUserExists):
		return registerDeviceFailure{
			status:      http.StatusConflict,
			code:        "user_exists",
			description: registerdevice.ErrUserExists.Error(),
		}

	case errors.Is(err, registerdevice.ErrDeviceExists):
		return registerDeviceFailure{
			status:      http.StatusConflict,
			code:        "device_exists",
			description: registerdevice.ErrDeviceExists.Error(),
		}

	case errors.Is(err, registerdevice.ErrBusy):
		return registerDeviceFailure{
			status:      http.StatusServiceUnavailable,
			code:        "busy",
			description: registerdevice.ErrBusy.Error(),
			retryAfter:  retryAfterBusySeconds,
		}

	case errors.Is(err, registerdevice.ErrInternal):
		return registerDeviceFailure{
			status:      http.StatusInternalServerError,
			code:        "internal_error",
			description: httpx.ErrInternal.Error(),
		}

	default:
		return registerDeviceFailure{
			status:      http.StatusInternalServerError,
			code:        "internal_error",
			description: httpx.ErrInternal.Error(),
			unexpected:  true,
		}
	}
}
