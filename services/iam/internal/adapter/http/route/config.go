/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package route

import (
	"time"

	"xchat/pkg/config"
)

const (
	defaultRegisterDeviceTimeout    = 3 * time.Second
	defaultIssueDeviceTicketTimeout = time.Second
)

// Config holds the settings of the API routes.
type Config struct {
	// RegisterDeviceTimeout is the maximum time that POST /api/device/register
	// gives to the use case. After this time, the client gets 503 and can try
	// again. The value must be less than the HTTP write timeout, so that the
	// server can write the error response. A negative value disables the
	// deadline.
	//
	// Default: 3s
	RegisterDeviceTimeout time.Duration

	// IssueDeviceTicketTimeout is the maximum time that POST /api/device/ticket
	// gives to the use case. After this time, the client gets 503 and can try
	// again. The value must be less than the HTTP write timeout, so that the
	// server can write the error response. A negative value disables the
	// deadline.
	//
	// Default: 1s
	IssueDeviceTicketTimeout time.Duration
}

// LoadConfig reads the settings of the API routes and applies the defaults. A
// nil loader is an empty source.
func LoadConfig(loader config.Loader) Config {
	var c Config
	if loader != nil {
		c.RegisterDeviceTimeout = loader.GetDuration("HTTP_REGISTER_DEVICE_TIMEOUT")
		c.IssueDeviceTicketTimeout = loader.GetDuration("HTTP_ISSUE_DEVICE_TICKET_TIMEOUT")
	}
	c.applyDefault()
	return c
}

// applyDefault puts the default value in each timeout that is zero. A negative
// timeout stays negative, because it disables that deadline.
func (c *Config) applyDefault() {
	if c.RegisterDeviceTimeout == 0 {
		c.RegisterDeviceTimeout = defaultRegisterDeviceTimeout
	}
	if c.IssueDeviceTicketTimeout == 0 {
		c.IssueDeviceTicketTimeout = defaultIssueDeviceTicketTimeout
	}
}
