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

	numReasons
)

var names = [numReasons]string{
	AuthHeader: "iam_auth_header",
	AuthToken:  "iam_auth_token",
}

// Names returns a copy of the stable labels indexed by Reason.
func Names() []string {
	return names[:]
}
