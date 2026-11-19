/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package route

import (
	"testing"
	"time"

	"xchat/internal/iam/core/rejection"

	"xchat/pkg/log"
	"xchat/pkg/log/logtest"
	"xchat/pkg/reject"

	"github.com/stretchr/testify/require"
)

func newTestDeps(t *testing.T) (*log.Logger, *reject.Stats[rejection.Reason]) {
	t.Helper()

	logger, rejections, _ := newTestDepsWithLog(t)
	return logger, rejections
}

// newTestDepsWithLog also gives the path of the file that receives the log.
func newTestDepsWithLog(t *testing.T) (*log.Logger, *reject.Stats[rejection.Reason], string) {
	t.Helper()

	logger, closeLogger, path := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })

	return logger, reject.New[rejection.Reason](logger, time.Hour, rejection.Names()), path
}
