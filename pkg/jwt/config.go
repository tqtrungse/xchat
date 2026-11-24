/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package jwt

type AuthConfig struct {
	Issuer   string
	Audience string
	JWKSURL  string
}
