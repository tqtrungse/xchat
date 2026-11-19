/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"flag"
	"io"
	"testing"

	"xchat/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		wantEnv config.Env
	}{
		{name: "default", args: nil, wantEnv: config.EnvProd},
		{name: "prod", args: []string{"-env=prod"}, wantEnv: config.EnvProd},
		{name: "local", args: []string{"-env", "local"}, wantEnv: config.EnvLocal},
		// parseArgs does not check the name. config.Load rejects an unknown
		// name with the list of valid names.
		{name: "unknown", args: []string{"-env=staging"}, wantEnv: "staging"},
	} {
		t.Run(test.name, func(t *testing.T) {
			env, err := parseArgs(test.args, io.Discard)
			require.NoError(t, err)
			require.Equal(t, test.wantEnv, env)
		})
	}
}

func TestParseArgsErrors(t *testing.T) {
	_, err := parseArgs([]string{"-unknown"}, io.Discard)
	require.Error(t, err)

	_, err = parseArgs([]string{"local"}, io.Discard)
	require.ErrorContains(t, err, `unexpected argument "local"`)

	_, err = parseArgs([]string{"-h"}, io.Discard)
	require.ErrorIs(t, err, flag.ErrHelp)
}
