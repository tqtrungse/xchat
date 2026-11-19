/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package bootstrap

import (
	"errors"
	"fmt"

	"xchat/pkg/config"
	"xchat/pkg/generator"
	"xchat/pkg/k8s"
)

const (
	serviceIDKey  = "SERVICE_ID"
	instanceIDKey = "INSTANCE_ID"
)

// machineID makes the machine ID that the ID generator uses.
//
// The ID of the instance comes from INSTANCE_ID, for a deployment on Docker or
// on a VM. If the config does not set INSTANCE_ID, it comes from the ordinal of
// the StatefulSet pod. Two instances of a service must not have the same ID.
func machineID(loader config.Loader) (uint64, error) {
	return resolveMachineID(loader, k8s.MachineIDFromHostname)
}

// resolveMachineID is machineID with the source for Kubernetes as an argument.
func resolveMachineID(
	loader config.Loader,
	fromHostname func(serviceID uint64) (uint64, error),
) (uint64, error) {
	if !loader.IsSet(serviceIDKey) {
		return 0, fmt.Errorf("%s is required", serviceIDKey)
	}
	serviceID := loader.GetUint64(serviceIDKey)

	if loader.IsSet(instanceIDKey) {
		id, err := generator.MachineID(serviceID, loader.GetUint64(instanceIDKey))
		if err != nil {
			return 0, fmt.Errorf("%s: %w", instanceIDKey, err)
		}
		return id, nil
	}

	id, err := fromHostname(serviceID)
	if err != nil {
		if errors.Is(err, k8s.ErrNotInCluster) {
			return 0, fmt.Errorf("%s is required outside Kubernetes", instanceIDKey)
		}
		return 0, fmt.Errorf("%s is not set and the host name gives no instance ID: %w", instanceIDKey, err)
	}
	return id, nil
}
