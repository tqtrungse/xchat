/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	iamhttp "xchat/internal/iam/adapter/http"
	"xchat/internal/iam/core/rejection"

	"xchat/pkg/config"
	"xchat/pkg/log"
	"xchat/pkg/reject"

	"go.uber.org/zap"
)

const shutdownTimeout = 30 * time.Second

// Run constructs IAM's dependencies, starts its HTTPS API, and shuts it down
// gracefully when ctx is canceled. The graceful shutdown waits no more than
// shutdownTimeout for active requests. A canceled abort ends that wait early:
// the server closes each connection at once. Run flushes the logger after a
// normal shutdown and after an aborted shutdown. env selects the config file
// that Run reads.
func Run(ctx, abort context.Context, env config.Env) (retErr error) {
	loader, err := config.Load("iam", env)
	if err != nil {
		return fmt.Errorf("failed to load IAM config: %w", err)
	}

	logger, closeLogger, err := log.NewLogger(log.LoadConfig(loader))
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	defer func() {
		closeErr := closeLogger()
		if closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("failed to close logger: %w", closeErr))
		}
	}()

	httpConfig, err := iamhttp.LoadConfig(loader)
	if err != nil {
		return fmt.Errorf("failed to load HTTP config: %w", err)
	}

	rejections := reject.New[rejection.Reason](
		logger,
		5*time.Second,
		rejection.Names(),
	)

	server, err := iamhttp.NewServer(
		ctx,
		httpConfig,
		logger,
		rejections,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize IAM HTTP server: %w", err)
	}

	serverResult := make(chan error, 1)
	go func() { serverResult <- server.Run() }()

	logger.Info("starting IAM HTTP server", zap.String("address", httpConfig.Addr))
	syncLogger(logger)

	select {
	case err = <-serverResult:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("failed to run IAM HTTP server: %w", err)

	case <-ctx.Done():
		logger.Info("shutting down IAM HTTP server")
		syncLogger(logger)

		// ctx is already canceled here. The shutdown context uses abort.
		// Therefore, only a second signal or the timeout ends the wait.
		timeoutCtx, cancel := context.WithTimeout(abort, shutdownTimeout)
		defer cancel()

		shutdownErr := server.Shutdown(timeoutCtx)
		if shutdownErr != nil && abort.Err() != nil {
			logger.Warn("graceful shutdown aborted, closed each connection")
		}
		serverErr := <-serverResult

		if errors.Is(serverErr, http.ErrServerClosed) {
			serverErr = nil
		} else if serverErr != nil {
			serverErr = fmt.Errorf("failed to run IAM HTTP server: %w", serverErr)
		}

		return errors.Join(shutdownErr, serverErr)
	}
}

// syncLogger writes each queued log entry to the sink. A failed sync writes
// the error to stderr, not to the logger. The sink is stuck or broken in this
// case, so the sink would also lose an entry about the failure. The logger
// writes its own diagnostics to stderr too.
func syncLogger(logger *log.Logger) {
	err := logger.Sync()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to sync logger:", err)
	}
}
