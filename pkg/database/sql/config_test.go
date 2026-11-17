/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package sql

import (
	"testing"
	"time"

	"xchat/pkg/config"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func requiredLoader() *viper.Viper {
	l := viper.New()
	l.Set("SQL_DATABASE_PRIMARY_ADDRESS", "primary:3306")
	l.Set("SQL_DATABASE_USER", "user")
	l.Set("SQL_DATABASE_NAME", "db")
	return l
}

func TestConfigLoadDefaults(t *testing.T) {
	want := Config{
		User:          "user",
		DBName:        "db",
		PrimaryAddr:   "primary:3306",
		DialTimeout:   3 * time.Second,
		ReadTimeout:   10 * time.Second,
		WriteTimeout:  5 * time.Second,
		SlowThreshold: 200 * time.Millisecond,
	}

	t.Run("unset keys", func(t *testing.T) {
		dbConfig, err := LoadConfig(requiredLoader())
		require.NoError(t, err)
		require.Equal(t, want, dbConfig)
	})
}

func TestConfigLoadRequiredFields(t *testing.T) {
	for _, key := range []string{
		"SQL_DATABASE_PRIMARY_ADDRESS",
		"SQL_DATABASE_USER",
		"SQL_DATABASE_NAME",
	} {
		t.Run(key, func(t *testing.T) {
			l := requiredLoader()
			l.Set(key, " ")
			_, err := LoadConfig(l)
			require.EqualError(t, err, key+" is required")
		})
	}

	t.Run("nil loader", func(t *testing.T) {
		_, err := LoadConfig(nil)
		require.EqualError(t, err, "SQL_DATABASE_PRIMARY_ADDRESS is required")
	})
}

func TestPoolConfigLoadDefaults(t *testing.T) {
	for _, loader := range []config.Loader{nil, viper.New()} {
		poolConfig := loadPoolConfig(loader)
		require.Equal(t, PoolConfig{}, poolConfig)
	}
}

func TestConfigLoadPreservesExplicitValues(t *testing.T) {
	l := requiredLoader()
	l.Set("SQL_DATABASE_MAX_OPEN_CONNS", 8)
	l.Set("SQL_DATABASE_MAX_IDLE_CONNS", 0)

	dbConfig, err := LoadConfig(l)
	require.NoError(t, err)
	require.Equal(t, 8, dbConfig.Pool.MaxOpenConns)
	require.Zero(t, dbConfig.Pool.MaxIdleConns)
}

func TestConfigLoadTimeoutOverrides(t *testing.T) {
	defaults := Config{
		DialTimeout:   3 * time.Second,
		ReadTimeout:   10 * time.Second,
		WriteTimeout:  5 * time.Second,
		SlowThreshold: 200 * time.Millisecond,
	}
	same := func(d time.Duration) func(time.Duration) time.Duration {
		return func(time.Duration) time.Duration { return d }
	}

	for _, tc := range []struct {
		name  string
		value any
		want  func(def time.Duration) time.Duration
	}{
		{
			name:  "duration",
			value: 2 * time.Second,
			want:  same(2 * time.Second),
		},
		{
			name:  "duration_string",
			value: "750ms",
			want:  same(750 * time.Millisecond),
		},
		{
			name:  "zero",
			value: 0,
			want:  func(def time.Duration) time.Duration { return def },
		},
		{
			name:  "zero_string",
			value: "0s",
			want:  func(def time.Duration) time.Duration { return def },
		},
		{
			name:  "negative_string",
			value: "-1s",
			want:  same(-time.Second),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loader := requiredLoader()
			loader.Set("SQL_DATABASE_DIAL_TIMEOUT", tc.value)
			loader.Set("SQL_DATABASE_READ_TIMEOUT", tc.value)
			loader.Set("SQL_DATABASE_WRITE_TIMEOUT", tc.value)
			loader.Set("SQL_DATABASE_SLOW_THRESHOLD", tc.value)

			dbConfig, err := LoadConfig(loader)
			require.NoError(t, err)
			require.Equal(t, tc.want(defaults.DialTimeout), dbConfig.DialTimeout)
			require.Equal(t, tc.want(defaults.ReadTimeout), dbConfig.ReadTimeout)
			require.Equal(t, tc.want(defaults.WriteTimeout), dbConfig.WriteTimeout)
			require.Equal(t, tc.want(defaults.SlowThreshold), dbConfig.SlowThreshold)
		})
	}
}

func TestConfigLoadPartialTimeoutOverride(t *testing.T) {
	loader := requiredLoader()
	loader.Set("SQL_DATABASE_READ_TIMEOUT", "30s")

	dbConfig, err := LoadConfig(loader)
	require.NoError(t, err)
	require.Equal(t, 3*time.Second, dbConfig.DialTimeout)
	require.Equal(t, 30*time.Second, dbConfig.ReadTimeout)
	require.Equal(t, 5*time.Second, dbConfig.WriteTimeout)
}

func TestConfigLoadTimeoutsFromEnvironment(t *testing.T) {
	t.Setenv("SQL_DATABASE_DIAL_TIMEOUT", "0")
	t.Setenv("SQL_DATABASE_READ_TIMEOUT", "20s")
	t.Setenv("SQL_DATABASE_WRITE_TIMEOUT", "-1s")
	loader := requiredLoader()
	loader.AutomaticEnv()

	dbConfig, err := LoadConfig(loader)
	require.NoError(t, err)
	require.Equal(t, 3*time.Second, dbConfig.DialTimeout)
	require.Equal(t, 20*time.Second, dbConfig.ReadTimeout)
	require.Equal(t, -time.Second, dbConfig.WriteTimeout)
}

func TestConfigApplyDefaultKeepsNegativeLimits(t *testing.T) {
	dbConfig := Config{
		ReadTimeout:   -1,
		SlowThreshold: -1,
	}
	dbConfig.applyDefault()
	require.Equal(t, 3*time.Second, dbConfig.DialTimeout)
	require.EqualValues(t, -1, dbConfig.ReadTimeout)
	require.Equal(t, 5*time.Second, dbConfig.WriteTimeout)
	require.EqualValues(t, -1, dbConfig.SlowThreshold)
}
