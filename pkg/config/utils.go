/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Env selects the environment file that Load reads.
type Env uint8

const (
	// EnvProd reads cmd/SERVICE/.prod.env.
	EnvProd Env = iota

	// EnvLocal reads cmd/SERVICE/.local.env.
	EnvLocal
)

// Load reads the config file of the service and the environment.
func Load(serviceName string, env Env) (Loader, error) {
	path, err := findPath(serviceName, env)
	if err != nil {
		return nil, err
	}
	return defaultLoader(path)
}

func SplitList(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func defaultLoader(path string) (Loader, error) {
	v := viper.New()
	v.AutomaticEnv()
	if len(path) == 0 {
		return v, nil
	}

	// SetConfigFile reads the exact file at path and gets the format from its extension.
	// A search by base name selects the first match in the extension list of viper.
	// Thus app.json replaces app.yaml when the two files are in the same folder.
	v.SetConfigFile(path)
	// Read in config file
	err := v.ReadInConfig()
	if err != nil {
		return nil, err
	}

	// Watch config dynamically
	v.WatchConfig()
	return v, nil
}

func findPath(serviceName string, env Env) (string, error) {
	envFile, err := env.fileName()
	if err != nil {
		return "", err
	}

	serviceName = strings.ToLower(serviceName)
	candidates := []string{
		filepath.Join("cmd", serviceName, envFile),
		envFile,
	}
	if executable, exeErr := os.Executable(); exeErr == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), envFile))
	}

	var absPath string
	for _, candidate := range candidates {
		absPath, err = filepath.Abs(candidate)
		if err != nil {
			return "", err
		}
		if _, err = os.Stat(absPath); err == nil {
			return absPath, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect %s config %q: %w", serviceName, absPath, err)
		}
	}
	return "", fmt.Errorf(
		"%s config not found; provide cmd/%s/%s",
		serviceName,
		serviceName,
		envFile,
	)
}

// fileName gives the environment file name.
func (e Env) fileName() (string, error) {
	switch e {
	case EnvProd:
		return ".prod.env", nil

	case EnvLocal:
		return ".local.env", nil

	default:
		return "", fmt.Errorf("unknown config environment %d", e)
	}
}
