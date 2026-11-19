/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package generator

import (
	"errors"
	"fmt"
)

// The machine ID has 16 bits. This is the default size that sonyflake gives to
// the machine ID. The high 8 bits hold the ID of the service, and the low 8
// bits hold the ID of the instance of that service.
const (
	serviceIDBits  = 8
	instanceIDBits = 8

	// MaxServiceID is the largest service ID. There are 256 services.
	MaxServiceID = 1<<serviceIDBits - 1

	// MaxInstanceID is the largest instance ID. A service has 256 instances.
	MaxInstanceID = 1<<instanceIDBits - 1

	// maxMachineID is the largest value that fits in 16 bits.
	maxMachineID = 1<<(serviceIDBits+instanceIDBits) - 1
)

var (
	ErrServiceIDOutOfRange  = errors.New("service ID is out of range")
	ErrInstanceIDOutOfRange = errors.New("instance ID is out of range")
	ErrMachineIDOutOfRange  = errors.New("machine ID is out of range")
)

// MachineID joins the ID of a service and the ID of one of its instances into
// a machine ID: serviceID<<8 | instanceID. Two running instances must not have
// the same machine ID, because they could then generate the same ID.
//
// MachineID returns ErrServiceIDOutOfRange if serviceID is more than
// MaxServiceID, and ErrInstanceIDOutOfRange if instanceID is more than
// MaxInstanceID.
func MachineID(serviceID, instanceID uint64) (uint64, error) {
	if serviceID > MaxServiceID {
		return 0, fmt.Errorf("%w: %d, the maximum is %d", ErrServiceIDOutOfRange, serviceID, MaxServiceID)
	}
	if instanceID > MaxInstanceID {
		return 0, fmt.Errorf("%w: %d, the maximum is %d", ErrInstanceIDOutOfRange, instanceID, MaxInstanceID)
	}
	return serviceID<<instanceIDBits | instanceID, nil
}
