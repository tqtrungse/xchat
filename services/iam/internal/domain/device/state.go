/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package device

import shareddevice "xchat/services/internal/shared/device"

// State is the state of a device. It is an alias of the shared type, so that
// this service imports one device package.
type State = shareddevice.State

const (
	StateActive = shareddevice.StateActive
	StateLocked = shareddevice.StateLocked
)
