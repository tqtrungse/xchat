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
	"xchat/services/iam/internal/application/device/issueticket"
	"xchat/services/iam/internal/domain/user"

	"xchat/pkg/log"
	slicepool "xchat/pkg/pool/slice"
	"xchat/pkg/reject"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// maxIssueDeviceTicketBodyBytes limits the request body. A valid request has
// one device ID, so it takes less than 100 bytes.
const maxIssueDeviceTicketBodyBytes = 1 << 10

// DeviceTicketIssuer is the use case that the handler of POST /api/device/ticket
// calls. *issueticket.Usecase implements it.
type DeviceTicketIssuer interface {
	IssueTicket(
		ctx context.Context,
		userID user.ID,
		req issueticket.Request,
	) (*issueticket.Response, error)
}

// IssueDeviceTicketHandler handles POST /api/device/ticket. It issues a ticket
// for a device of the user in the token. The device sends the ticket to the hub
// to connect.
type IssueDeviceTicketHandler struct {
	usecase    DeviceTicketIssuer
	logger     *log.Logger
	rejections *reject.Stats[rejection.Reason]
}

// NewIssueDeviceTicketHandler makes a IssueDeviceTicketHandler. The caller must give all
// arguments.
func NewIssueDeviceTicketHandler(
	usecase DeviceTicketIssuer,
	logger *log.Logger,
	rejections *reject.Stats[rejection.Reason],
) *IssueDeviceTicketHandler {
	return &IssueDeviceTicketHandler{
		usecase:    usecase,
		logger:     logger,
		rejections: rejections,
	}
}

// Mount registers the route on group. group must run the authentication
// middleware, because the handler reads the claims that it attaches.
func (h *IssueDeviceTicketHandler) Mount(group *gin.RouterGroup) {
	group.POST("/device/ticket", h.handle)
}

func (h *IssueDeviceTicketHandler) handle(c *gin.Context) {
	userID, authenticated := requireAuthenticatedUser(c, h.rejections)
	if !authenticated {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxIssueDeviceTicketBodyBytes)

	var req issueticket.Request
	if !decodeRequestBody(c, h.rejections, &req) {
		return
	}

	resp, err := h.usecase.IssueTicket(c.Request.Context(), user.ID(userID), req)
	if err != nil {
		respondUsecaseFailure(
			c,
			h.logger,
			h.rejections,
			"failed to issue device ticket",
			userID,
			err,
			mapIssueDeviceTicketError(err),
		)
		return
	}
	if resp == nil {
		// The use case must return a response when it returns no error.
		h.logger.Error(
			"issue device ticket returned no response",
			zap.Uint64("user_id", userID),
		)
		httpx.RespondError(c, http.StatusInternalServerError, "internal_error", httpx.ErrInternal.Error())
		return
	}
	// The ticket buffer belongs to the handler. It holds a signed credential,
	// so the handler clears it and returns it to the pool on each path.
	defer slicepool.PutZeroed(resp.Ticket)

	err = httpx.RespondSuccess(c, http.StatusOK, resp)
	switch {
	case err == nil:

	case httpx.IsClientGone(c, err):
		h.rejections.Record(rejection.ResponseWrite, err)

	default:
		// The encoding failed, or write failed for a reason that is not the
		// client.
		h.logger.Error(
			"failed to respond to issue device ticket",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
	}
}

// mapIssueDeviceTicketError converts an error of the use case to a response.
//
// A device that does not exist and a device of another user give the same
// response. A client cannot learn from the response whether a device ID exists.
func mapIssueDeviceTicketError(err error) usecaseFailure {
	switch {
	case errors.Is(err, issueticket.ErrRequestInvalid):
		return usecaseFailure{
			status:      http.StatusBadRequest,
			code:        "invalid_request",
			description: issueticket.ErrRequestInvalid.Error(),
			reason:      issueticket.ErrRequestInvalid,
		}

	case errors.Is(err, issueticket.ErrDeviceNotFound):
		return usecaseFailure{
			status:      http.StatusNotFound,
			code:        "device_not_found",
			description: issueticket.ErrDeviceNotFound.Error(),
		}

	case errors.Is(err, issueticket.ErrDeviceInactive):
		return usecaseFailure{
			status:      http.StatusForbidden,
			code:        "device_inactive",
			description: issueticket.ErrDeviceInactive.Error(),
		}

	case errors.Is(err, issueticket.ErrInternal):
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
