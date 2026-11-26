/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package rejection

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNames(t *testing.T) {
	t.Parallel()

	labels := Names()
	require.Len(t, labels, int(numReasons))
	for idx, label := range labels {
		require.NotEmpty(t, label, "reason %d has no label", idx)
	}
	require.Equal(t, "iam_auth_header", labels[AuthHeader])
	require.Equal(t, "iam_response_write", labels[ResponseWrite])
}
