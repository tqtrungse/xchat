/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package generator

import (
	"fmt"

	"github.com/sony/sonyflake/v2"
)

type integer struct {
	sf *sonyflake.Sonyflake
}

// NewInt creates a generator of unique IDs. The argument id is the machine ID
// of this process. Use MachineID to make it. NewInt returns
// ErrMachineIDOutOfRange if id does not fit in 16 bits.
func NewInt(id uint64) (Integer, error) {
	if id > maxMachineID {
		return nil, fmt.Errorf("%w: %d, the maximum is %d", ErrMachineIDOutOfRange, id, maxMachineID)
	}

	sf, err := sonyflake.New(
		sonyflake.Settings{
			MachineID: func() (int, error) { return int(id), nil },
		},
	)
	if err != nil {
		return nil, err
	}
	return &integer{sf: sf}, nil
}

func (i *integer) Next() (uint64, error) {
	id, err := i.sf.NextID()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}
