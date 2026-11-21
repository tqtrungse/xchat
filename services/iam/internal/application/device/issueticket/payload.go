/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package issueticket

import "xchat/services/internal/shared/hub"

type Request struct {
	DeviceID string `json:"device_id"`
}

type Response struct {
	// The handler must call `slicepool.PutZeroed` after writing the response.
	Ticket []byte `json:"ticket"`
	// Hub.PublicKey shares its bytes with the use case and with each other
	// response. Do not change these bytes. To change them, change a copy that
	// bytes.Clone makes.
	Hub hub.Descriptor `json:"hub"`
}
