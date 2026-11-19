/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package user

type State uint8

const (
	StateActive = State(1)
	StateLocked = State(2)
)
