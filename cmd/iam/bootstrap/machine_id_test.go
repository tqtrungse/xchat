/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package bootstrap

import (
	"errors"
	"testing"

	"xchat/pkg/config"
	"xchat/pkg/generator"
	"xchat/pkg/k8s"

	"github.com/stretchr/testify/require"
)

// stubLoader is a config.Loader with a few integer keys. A call to any other
// method panics, because the embedded interface is nil.
type stubLoader struct {
	config.Loader
	values map[string]uint64
}

func (l stubLoader) IsSet(key string) bool {
	_, ok := l.values[key]
	return ok
}

func (l stubLoader) GetUint64(key string) uint64 { return l.values[key] }

func TestResolveMachineID(t *testing.T) {
	t.Parallel()

	hostnameOrdinal := func(serviceID uint64) (uint64, error) {
		return serviceID<<8 | 7, nil
	}
	hostnameMustNotRun := func(uint64) (uint64, error) {
		t.Error("the host name must not be read when INSTANCE_ID is set")
		return 0, nil
	}
	outsideCluster := func(uint64) (uint64, error) { return 0, k8s.ErrNotInCluster }
	badHostname := func(uint64) (uint64, error) { return 0, errors.New("invalid ordinal") }

	for _, test := range []struct {
		name         string
		values       map[string]uint64
		fromHostname func(uint64) (uint64, error)
		want         uint64
		wantErr      string
	}{
		{
			name:         "instance ID from the config",
			values:       map[string]uint64{"SERVICE_ID": 2, "INSTANCE_ID": 5},
			fromHostname: hostnameMustNotRun,
			want:         2<<8 | 5,
		},
		{
			name:         "instance ID zero in the config is a value",
			values:       map[string]uint64{"SERVICE_ID": 2, "INSTANCE_ID": 0},
			fromHostname: hostnameMustNotRun,
			want:         2 << 8,
		},
		{
			name:         "instance ID from the host name",
			values:       map[string]uint64{"SERVICE_ID": 2},
			fromHostname: hostnameOrdinal,
			want:         2<<8 | 7,
		},
		{
			name:         "service ID is required",
			values:       map[string]uint64{"INSTANCE_ID": 1},
			fromHostname: hostnameMustNotRun,
			wantErr:      "SERVICE_ID is required",
		},
		{
			name:         "service ID is out of range",
			values:       map[string]uint64{"SERVICE_ID": 256, "INSTANCE_ID": 1},
			fromHostname: hostnameMustNotRun,
			wantErr:      "service ID is out of range",
		},
		{
			name:         "instance ID is out of range",
			values:       map[string]uint64{"SERVICE_ID": 2, "INSTANCE_ID": 256},
			fromHostname: hostnameMustNotRun,
			wantErr:      "INSTANCE_ID: instance ID is out of range",
		},
		{
			name:         "no instance ID outside Kubernetes",
			values:       map[string]uint64{"SERVICE_ID": 2},
			fromHostname: outsideCluster,
			wantErr:      "INSTANCE_ID is required outside Kubernetes",
		},
		{
			name:         "host name without an ordinal",
			values:       map[string]uint64{"SERVICE_ID": 2},
			fromHostname: badHostname,
			wantErr:      "INSTANCE_ID is not set and the host name gives no instance ID",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveMachineID(stubLoader{values: test.values}, test.fromHostname)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestResolveMachineIDErrorsKeepTheCause(t *testing.T) {
	t.Parallel()

	_, err := resolveMachineID(
		stubLoader{values: map[string]uint64{"SERVICE_ID": 2, "INSTANCE_ID": 300}},
		nil,
	)
	require.ErrorIs(t, err, generator.ErrInstanceIDOutOfRange)
}
