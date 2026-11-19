/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package k8s

import (
	"testing"

	"xchat/pkg/generator"

	"github.com/stretchr/testify/require"
)

func TestMachineIDFromName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		host    string
		service uint64
		want    uint64
		wantErr string
	}{
		{name: "first pod", host: "iam-0", service: 2, want: 0x0200},
		{name: "third pod", host: "iam-2", service: 2, want: 0x0202},
		{name: "last ordinal", host: "iam-255", service: 2, want: 0x02FF},
		{name: "name with dashes", host: "xchat-iam-12", service: 2, want: 0x020C},
		{name: "ordinal too large", host: "iam-256", service: 2, wantErr: "instance ID is out of range"},
		{name: "service too large", host: "iam-1", service: 256, wantErr: "service ID is out of range"},
		{name: "no dash", host: "iam", service: 2, wantErr: "no ordinal"},
		{name: "empty suffix", host: "iam-", service: 2, wantErr: "invalid ordinal"},
		{name: "deployment pod", host: "iam-7d9f8b6c5-x2k4q", service: 2, wantErr: "invalid ordinal"},
		{name: "plus sign", host: "iam-+1", service: 2, wantErr: "invalid ordinal"},
		{name: "padded ordinal", host: "iam-01", service: 2, wantErr: "invalid ordinal"},
		{name: "number overflows", host: "iam-99999999999999999999", service: 2, wantErr: "invalid ordinal"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := machineIDFromName(test.host, test.service)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestMachineIDFromNameKeepsTheCause(t *testing.T) {
	t.Parallel()

	_, err := machineIDFromName("iam-300", 2)
	require.ErrorIs(t, err, generator.ErrInstanceIDOutOfRange)
}

func TestMachineIDFromHostnameOutsideCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	_, err := MachineIDFromHostname(2)
	require.ErrorIs(t, err, ErrNotInCluster)
}
