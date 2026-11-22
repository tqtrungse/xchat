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

// Env names the environment of a service. It selects the environment file
// that Load reads. A command can convert a flag value to Env, and Load rejects
// an unknown name.
type Env string

const (
	// EnvProd reads .prod.env. See Load for the folders that Load searches.
	EnvProd Env = "prod"

	// EnvLocal reads .local.env. See Load for the folders that Load searches.
	EnvLocal Env = "local"
)

// Load reads the config file of the service and the environment.
//
// Load searches the environment file in two folders. The first folder is the
// working directory. The second folder is the folder of the executable. Load
// uses the first file that it finds. serviceName appears in the errors only.
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
	candidates := []string{envFile}
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
		"%s config not found; provide %s in the working directory or next to the executable",
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
		return "", fmt.Errorf("unknown config environment %q; use %q or %q", string(e), EnvProd, EnvLocal)
	}
}
