/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package route

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                         string
		values                       map[string]string
		nilLoader                    bool
		wantRegisterDeviceTimeout    time.Duration
		wantIssueDeviceTicketTimeout time.Duration
	}{
		{
			name:                         "nil loader gives the defaults",
			nilLoader:                    true,
			wantRegisterDeviceTimeout:    3 * time.Second,
			wantIssueDeviceTicketTimeout: time.Second,
		},
		{
			name:                         "empty loader gives the defaults",
			values:                       map[string]string{},
			wantRegisterDeviceTimeout:    3 * time.Second,
			wantIssueDeviceTicketTimeout: time.Second,
		},
		{
			name: "loader values replace the defaults",
			values: map[string]string{
				"HTTP_REGISTER_DEVICE_TIMEOUT":     "5s",
				"HTTP_ISSUE_DEVICE_TICKET_TIMEOUT": "500ms",
			},
			wantRegisterDeviceTimeout:    5 * time.Second,
			wantIssueDeviceTicketTimeout: 500 * time.Millisecond,
		},
		{
			name: "zero gives the default",
			values: map[string]string{
				"HTTP_REGISTER_DEVICE_TIMEOUT":     "0s",
				"HTTP_ISSUE_DEVICE_TICKET_TIMEOUT": "0s",
			},
			wantRegisterDeviceTimeout:    3 * time.Second,
			wantIssueDeviceTicketTimeout: time.Second,
		},
		{
			name: "negative value stays negative",
			values: map[string]string{
				"HTTP_REGISTER_DEVICE_TIMEOUT":     "-1s",
				"HTTP_ISSUE_DEVICE_TICKET_TIMEOUT": "-1s",
			},
			wantRegisterDeviceTimeout:    -time.Second,
			wantIssueDeviceTicketTimeout: -time.Second,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var got Config
			if test.nilLoader {
				got = LoadConfig(nil)
			} else {
				loader := viper.New()
				for key, value := range test.values {
					loader.Set(key, value)
				}
				got = LoadConfig(loader)
			}

			require.Equal(t, test.wantRegisterDeviceTimeout, got.RegisterDeviceTimeout)
			require.Equal(t, test.wantIssueDeviceTicketTimeout, got.IssueDeviceTicketTimeout)
		})
	}
}
