/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package rejection

type Reason uint8

// Append only; labels are used by metrics, dashboards, and alerts.
const (
	AuthHeader Reason = iota
	AuthToken
	RequestBody
	ResponseWrite

	numReasons
)

var names = [numReasons]string{
	AuthHeader:    "iam_auth_header",
	AuthToken:     "iam_auth_token",
	RequestBody:   "iam_request_body",
	ResponseWrite: "iam_response_write",
}

// Names returns the stable labels indexed by Reason. The slice shares its array
// with the package, and reject.New keeps it. The caller must not change the
// slice.
func Names() []string {
	return names[:]
}
