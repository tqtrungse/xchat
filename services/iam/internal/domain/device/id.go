/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package device

import shareddevice "xchat/services/internal/shared/device"

// ID is the ID of a device. It is an alias of the shared type, so that the
// shared packages and this service use one type for the ID of a device.
type ID = shareddevice.ID
