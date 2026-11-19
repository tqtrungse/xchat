/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package device

import "errors"

// ErrAlreadyExists means that a device with the same unique value, such as the
// identity public key, exists.
var ErrAlreadyExists = errors.New("device already exists")
