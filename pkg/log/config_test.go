/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package log

import (
	"testing"
	"time"

	"xchat/pkg/config"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestConfigLoadDefaults(t *testing.T) {
	defaultConfig := Config{}
	defaultConfig.applyDefault()

	for _, tc := range []struct {
		name   string
		loader config.Loader
	}{
		{
			name:   "nil loader",
			loader: nil,
		},
		{
			name:   "unset keys",
			loader: viper.New(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := LoadConfig(tc.loader)
			require.Equal(t, defaultConfig, config)
		})
	}
}

func TestConfigLoadPreservesExplicitValues(t *testing.T) {
	l := viper.New()
	l.Set("LOGGER_QUEUE_SIZE", 32)
	l.Set("LOGGER_FLUSH_INTERVAL", "1s")
	l.Set("LOGGER_DISABLE_SAMPLING", true)

	config := LoadConfig(l)
	require.Equal(t, 32, config.QueueSize)
	require.Equal(t, time.Second, config.FlushInterval)
	require.True(t, config.DisableSampling)
	require.Equal(t, defaultBufferSize, config.BufferSize)
}
