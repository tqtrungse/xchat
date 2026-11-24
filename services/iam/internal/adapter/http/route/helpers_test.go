/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package route

import (
	"testing"
	"time"

	"xchat/services/iam/internal/adapter/http/httpx"
	"xchat/services/iam/internal/adapter/http/rejection"

	"xchat/pkg/jwt"
	"xchat/pkg/log"
	"xchat/pkg/log/logtest"
	"xchat/pkg/reject"

	"github.com/stretchr/testify/require"
)

// testUsecaseTimeout is the deadline of the use case in the handler tests. It
// is long, so that a slow test machine does not reach it.
const testUsecaseTimeout = time.Minute

// newTestDepsWithLog also gives the path of the file that receives the log.
func newTestDepsWithLog(t *testing.T) (*log.Logger, *reject.Stats[rejection.Reason], string) {
	t.Helper()

	logger, closeLogger, path := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })

	return logger, reject.New[rejection.Reason](logger, time.Hour, rejection.Names()), path
}

// testToken gives a verified token with userID in the ext claim.
func testToken(userID string) *jwt.ExtClaims[httpx.Claims] {
	return &jwt.ExtClaims[httpx.Claims]{
		Extension: httpx.Claims{
			UserID: userID,
		},
	}
}
