/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package config

import "time"

type Loader interface {
	// IsSet distinguishes an absent key from an explicitly configured zero value.
	IsSet(key string) bool

	GetString(key string) string

	GetInt(key string) int
	GetUint8(key string) uint8
	GetUint16(key string) uint16
	GetUint32(key string) uint32
	GetUint64(key string) uint64

	GetFloat64(key string) float64

	GetBool(key string) bool

	GetTime(key string) time.Time
	GetDuration(key string) time.Duration
}
