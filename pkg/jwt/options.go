/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package jwt

import "time"

const (
	// jwksRequestTimeout limits each request to the JWKS endpoint. It also
	// limits the startup time when the endpoint does not answer.
	jwksRequestTimeout = 10 * time.Second

	// jwksRetryMin and jwksRetryMax are the base and the cap of the
	// decorrelated jitter backoff. The backoff occurs between two fetches
	// while the JWKS has no key.
	jwksRetryMin = 1 * time.Second
	jwksRetryMax = 30 * time.Second
)

// options holds the settings that a test can change.
type options struct {
	requestTimeout time.Duration
	retryMin       time.Duration
	retryMax       time.Duration
}

var defaultOptions = options{
	requestTimeout: jwksRequestTimeout,
	retryMin:       jwksRetryMin,
	retryMax:       jwksRetryMax,
}
