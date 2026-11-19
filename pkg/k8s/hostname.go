/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package k8s

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"xchat/pkg/generator"
)

// ErrNotInCluster means that the process does not run in a Kubernetes pod.
var ErrNotInCluster = errors.New("not running in a Kubernetes pod")

// MachineIDFromHostname makes the machine ID of this process from the host name
// of a StatefulSet pod. The pod name is "<name>-<ordinal>", for example
// "iam-2". The ordinal is the instance ID. It is unique in the StatefulSet,
// and it stays the same when Kubernetes starts the pod again.
//
// A pod of a Deployment has a random suffix, and not an ordinal. The function
// returns an error for it. It also returns ErrNotInCluster outside Kubernetes:
// the host name of a VM, such as "app-3", would otherwise give a wrong ID
// without an error. Kubernetes sets KUBERNETES_SERVICE_HOST in each pod.
//
// For the layout of the machine ID, see generator.MachineID.
func MachineIDFromHostname(serviceID uint64) (uint64, error) {
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return 0, ErrNotInCluster
	}

	hostname, err := os.Hostname()
	if err != nil {
		return 0, fmt.Errorf("read hostname: %w", err)
	}
	return machineIDFromName(hostname, serviceID)
}

// machineIDFromName makes the machine ID from the pod name.
func machineIDFromName(name string, serviceID uint64) (uint64, error) {
	index := strings.LastIndexByte(name, '-')
	if index < 0 {
		return 0, fmt.Errorf("no ordinal in hostname %q", name)
	}

	suffix := name[index+1:]
	ordinal, err := strconv.ParseUint(suffix, 10, 64)
	// StatefulSet does not pad the ordinal. A suffix such as "01" is not an
	// ordinal, and it would give the same ID as "1".
	if err != nil || strconv.FormatUint(ordinal, 10) != suffix {
		return 0, fmt.Errorf("invalid ordinal in hostname %q", name)
	}

	id, err := generator.MachineID(serviceID, ordinal)
	if err != nil {
		return 0, fmt.Errorf("hostname %q: %w", name, err)
	}
	return id, nil
}
