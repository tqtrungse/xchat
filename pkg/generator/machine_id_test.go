/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package generator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMachineID(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		service  uint64
		instance uint64
		want     uint64
		wantErr  error
	}{
		{name: "both zero", want: 0},
		{name: "service in the high bits", service: 2, instance: 0, want: 0x0200},
		{name: "instance in the low bits", service: 0, instance: 5, want: 0x0005},
		{name: "both set", service: 2, instance: 5, want: 0x0205},
		{name: "maximum", service: MaxServiceID, instance: MaxInstanceID, want: 0xFFFF},
		{name: "service too large", service: MaxServiceID + 1, wantErr: ErrServiceIDOutOfRange},
		{name: "instance too large", instance: MaxInstanceID + 1, wantErr: ErrInstanceIDOutOfRange},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := MachineID(test.service, test.instance)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestMachineIDFitsTheDefaultSonyflakeBits(t *testing.T) {
	t.Parallel()

	got, err := MachineID(MaxServiceID, MaxInstanceID)
	require.NoError(t, err)
	require.Equal(t, uint64(maxMachineID), got)

	generator, err := NewInt(got)
	require.NoError(t, err)
	id, err := generator.Next()
	require.NoError(t, err)
	require.NotZero(t, id)
}

func TestNewIntRejectsAMachineIDThatDoesNotFit(t *testing.T) {
	t.Parallel()

	_, err := NewInt(maxMachineID + 1)
	require.ErrorIs(t, err, ErrMachineIDOutOfRange)
}

func TestNewIntGivesUniqueIDs(t *testing.T) {
	t.Parallel()

	generator, err := NewInt(0x0205)
	require.NoError(t, err)

	seen := make(map[uint64]struct{}, 1000)
	for range 1000 {
		id, err := generator.Next()
		require.NoError(t, err)
		_, duplicate := seen[id]
		require.False(t, duplicate)
		seen[id] = struct{}{}
	}
}
