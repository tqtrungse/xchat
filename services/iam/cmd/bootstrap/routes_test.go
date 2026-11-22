/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package bootstrap

import (
	"strings"
	"testing"
	"time"

	"xchat/pkg/config"

	"github.com/stretchr/testify/require"
)

// mapLoader is a config.Loader that reads strings and integers from a map. A
// call to another method panics, because the embedded interface is nil.
type mapLoader struct {
	config.Loader
	values map[string]string
}

func (l mapLoader) IsSet(key string) bool {
	_, ok := l.values[key]
	return ok
}

func (l mapLoader) GetString(key string) string { return l.values[key] }

func (l mapLoader) GetBool(string) bool { return false }

func (l mapLoader) GetInt(string) int { return 0 }

func (l mapLoader) GetDuration(string) time.Duration { return 0 }

func (l mapLoader) GetUint64(key string) uint64 {
	var value uint64
	for _, char := range l.values[key] {
		value = value*10 + uint64(char-'0')
	}
	return value
}

func (l mapLoader) GetUint8(key string) uint8 { return uint8(l.GetUint64(key)) }

// TestBuildRoutesFailsBeforeOpeningTheDatabase checks that buildRoutes reads
// and validates all config before it connects to the database. The test has no
// database, so a call that reached db.Open would fail with another error.
func TestBuildRoutesFailsBeforeOpeningTheDatabase(t *testing.T) {
	t.Parallel()

	database := map[string]string{
		"SQL_DATABASE_USER":            "iam",
		"SQL_DATABASE_NAME":            "iam",
		"SQL_DATABASE_PRIMARY_ADDRESS": "127.0.0.1:1",
	}

	machine := map[string]string{
		"SERVICE_ID":  "2",
		"INSTANCE_ID": "0",
	}

	// The private key is a Base64 Ed25519 seed of 32 zero bytes. It is a test
	// value only.
	ticketSigning := map[string]string{
		"TICKET_SIGNING_KEY_ID":      "1",
		"TICKET_SIGNING_PRIVATE_KEY": strings.Repeat("A", 43) + "=",
	}

	merge := func(parts ...map[string]string) map[string]string {
		merged := map[string]string{}
		for _, part := range parts {
			for key, value := range part {
				merged[key] = value
			}
		}
		return merged
	}

	for _, test := range []struct {
		name    string
		values  map[string]string
		wantErr string
	}{
		{
			name:    "missing database config",
			values:  map[string]string{"SERVICE_ID": "2", "INSTANCE_ID": "0"},
			wantErr: "SQL_DATABASE_PRIMARY_ADDRESS is required",
		},
		{
			name:    "missing service ID",
			values:  merge(database, map[string]string{"INSTANCE_ID": "0"}),
			wantErr: "SERVICE_ID is required",
		},
		{
			name:    "service ID out of range",
			values:  merge(database, map[string]string{"SERVICE_ID": "256", "INSTANCE_ID": "0"}),
			wantErr: "service ID is out of range",
		},
		{
			name:    "instance ID out of range",
			values:  merge(database, map[string]string{"SERVICE_ID": "2", "INSTANCE_ID": "256"}),
			wantErr: "instance ID is out of range",
		},
		{
			name:    "missing ticket signing key ID",
			values:  merge(database, machine),
			wantErr: "invalid TICKET_SIGNING_KEY_ID",
		},
		{
			name: "invalid ticket signing private key",
			values: merge(database, machine, map[string]string{
				"TICKET_SIGNING_KEY_ID":      "1",
				"TICKET_SIGNING_PRIVATE_KEY": "not-base64",
			}),
			wantErr: "decode ticket signing seed",
		},
		{
			name:    "missing hub descriptor file",
			values:  merge(database, machine, ticketSigning),
			wantErr: "HUB_NOISE_DESCRIPTOR_FILE is required",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			routes, closeFn, err := buildRoutes(t.Context(), mapLoader{values: test.values}, nil, nil)

			require.ErrorContains(t, err, test.wantErr)
			require.Nil(t, routes)
			require.Nil(t, closeFn, "no database is open, so there is nothing to close")
		})
	}
}
