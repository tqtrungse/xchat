/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"xchat/services/iam/cmd/iam/bootstrap"

	"xchat/pkg/config"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// parseArgs reads the command-line arguments. The -env flag selects the
// config file: "prod" reads cmd/iam/.prod.env, and "local" reads
// cmd/iam/.local.env. The default is "prod", so that a deployment needs no
// flag.
func parseArgs(args []string, output io.Writer) (config.Env, error) {
	flags := flag.NewFlagSet("iam", flag.ContinueOnError)
	flags.SetOutput(output)
	env := flags.String("env", string(config.EnvProd), `config environment: "prod" or "local"`)

	err := flags.Parse(args)
	if err != nil {
		return "", err
	}
	if flags.NArg() > 0 {
		return "", fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return config.Env(*env), nil
}

// run starts the IAM service and returns the exit code of the process.
//
// The first SIGINT or SIGTERM starts the graceful shutdown. A second signal
// aborts the graceful shutdown: the server closes each connection at once,
// and Run still flushes the logger. After the second signal, SIGINT and
// SIGTERM have their default action again. A third signal then stops the
// process immediately.
func run(args []string) int {
	env, err := parseArgs(args, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "IAM:", err)
		return 2
	}

	// The buffer keeps a second signal that arrives before the goroutine reads
	// the first signal.
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	abort, abortShutdown := context.WithCancel(context.Background())
	defer abortShutdown()

	go func() {
		select {
		case <-signals:
		case <-ctx.Done():
			return
		}
		stop()

		select {
		case <-signals:
		case <-abort.Done():
			return
		}
		signal.Stop(signals)
		abortShutdown()
	}()

	err = bootstrap.Run(ctx, abort, env)
	if abort.Err() != nil {
		_, _ = fmt.Fprintln(os.Stderr, "IAM stopped before the graceful shutdown finished:", err)
		return 1
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintln(os.Stderr, "IAM stopped:", err)
		return 1
	}
	return 0
}
