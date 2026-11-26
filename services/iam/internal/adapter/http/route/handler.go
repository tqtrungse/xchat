/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package route

import (
	"context"
	"errors"
	"net/http"
	"time"

	"xchat/services/iam/internal/adapter/http/httpx"
	"xchat/services/iam/internal/adapter/http/rejection"

	"xchat/pkg/log"
	"xchat/pkg/reject"

	"github.com/gin-gonic/gin"
	"github.com/mailru/easyjson"
	"go.uber.org/zap"
)

// requireAuthenticatedUser gives the ID of the user in the claims that the
// authentication middleware attached to the request.
//
// If the claims are missing or the user ID is not valid, requireAuthenticatedUser
// records the rejection, writes the 401 response, and returns false. The caller
// must stop.
func requireAuthenticatedUser(
	c *gin.Context,
	rejections *reject.Stats[rejection.Reason],
) (uint64, string, bool) {
	userID, userExternalID, authenticated := httpx.AuthenticatedUserID(c)
	if !authenticated {
		rejections.Record(rejection.AuthToken, httpx.ErrAuthVerify)
		httpx.RespondInvalidToken(c)
		return 0, "", false
	}
	return userID, userExternalID, true
}

// withTimeout gives a copy of ctx with a deadline after timeout. If timeout is
// zero or negative, withTimeout gives ctx without a new deadline. The caller
// must call the cancel function after the use case returns.
//
// The request context ends only when the client leaves. The HTTP write timeout
// does not end it. Without a deadline, a use case can wait without limit for a
// free database connection or a free password hasher.
func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

// decodeRequestBody reads the JSON body of the request into out. The caller
// must limit the size of the body with http.MaxBytesReader before it calls
// decodeRequestBody.
//
// If the decoding fails, decodeRequestBody records the rejection, writes the
// error response, and returns false. The caller must stop. The caller must
// still clear the buffers of out, because the decoder can fill them before it
// fails.
func decodeRequestBody(
	c *gin.Context,
	rejections *reject.Stats[rejection.Reason],
	out easyjson.Unmarshaler,
) bool {
	err := easyjson.UnmarshalFromReader(c.Request.Body, out)
	if err == nil {
		return true
	}

	// Decoder errors can contain request values; record only the rejection category.
	if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
		rejections.Record(rejection.RequestBody, httpx.ErrBodyTooLarge)
		httpx.RespondError(c, http.StatusRequestEntityTooLarge, "request_too_large", httpx.ErrBodyTooLarge.Error())
		return false
	}
	rejections.Record(rejection.RequestBody, httpx.ErrBodyInvalid)
	httpx.RespondError(c, http.StatusBadRequest, "invalid_request_body", httpx.ErrBodyInvalid.Error())
	return false
}

// usecaseFailure is the HTTP form of an error of a use case. Each handler has a
// function that converts the errors of its use case to a usecaseFailure.
type usecaseFailure struct {
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

// respondUsecaseFailure writes the response for failure. err is the error that
// the use case returned. message is the log message for an unexpected error.
//
// The response never contains the text of err. The use case already logs the
// causes of its internal error, so only an error that the use case does not
// define is logged here.
func respondUsecaseFailure(
	c *gin.Context,
	logger *log.Logger,
	rejections *reject.Stats[rejection.Reason],
	message string,
	userID uint64,
	err error,
	failure usecaseFailure,
) {
	switch {
	case failure.reason != nil:
		rejections.Record(rejection.RequestBody, failure.reason)

	case failure.unexpected:
		logger.Error(
			message,
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
	}

	if failure.retryAfter != "" {
		c.Header("Retry-After", failure.retryAfter)
	}
	httpx.RespondError(c, failure.status, failure.code, failure.description)
}
