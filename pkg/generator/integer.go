/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package generator

import "github.com/sony/sonyflake/v2"

type integer struct {
	sf *sonyflake.Sonyflake
}

func NewInt(id uint64) (Integer, error) {
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
