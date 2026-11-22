/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package bootstrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"xchat/services/internal/shared/hub"

	"xchat/pkg/config"
)

// errDuplicateKey does not contain the key, because a wrong file can contain
// private data.
var errDuplicateKey = errors.New("hub descriptor has a duplicate key")

// loadHubEndpoint reads public data only. It intentionally rejects a private
// keyring or extra private_key field. buildRoutes gives the descriptor to the
// issue ticket use case.
func loadHubEndpoint(loader config.Loader) (*hub.Descriptor, error) {
	if loader == nil {
		return nil, fmt.Errorf("config loader is nil")
	}

	path := strings.TrimSpace(loader.GetString("HUB_NOISE_DESCRIPTOR_FILE"))
	if len(path) == 0 {
		return nil, fmt.Errorf("HUB_NOISE_DESCRIPTOR_FILE is required")
	}

	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open hub descriptor: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat hub descriptor: %w", err)
	}

	if !info.Mode().IsRegular() {
		return nil, errors.New("hub descriptor must be a regular file")
	}

	data, err := io.ReadAll(io.LimitReader(f, hub.MaxDescriptorBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read hub descriptor: %w", err)
	}

	// hub.Parse checks the size, the syntax, the fields, and the values first.
	// Thus, rejectDuplicateKeys reads only valid JSON, and each error of
	// hub.Parse keeps its message.
	descriptor, err := hub.Parse(data)
	if err != nil {
		return nil, err
	}

	err = rejectDuplicateKeys(data)
	if err != nil {
		return nil, err
	}
	return descriptor, nil
}

// jsonFrame is an open object or an open array of the JSON data.
type jsonFrame struct {
	// keys holds the keys of an object. It is nil for an array.
	keys map[string]struct{}
	// expectKey is true if the next token of an object is a key.
	expectKey bool
}

// rejectDuplicateKeys returns errDuplicateKey if an object in data has the
// same key two times. hub.Parse keeps the last value of a duplicate key.
// Thus two different values in one file can go unnoticed.
//
// The check reads the JSON tokens. It does not read the lines, so it works for
// each layout of the JSON data. The decoder unescapes each key, so "key_id"
// and "key_id" are the same key.
func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var frames []*jsonFrame

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return errors.New("invalid hub descriptor JSON")
		}

		var top *jsonFrame
		if len(frames) > 0 {
			top = frames[len(frames)-1]
		}

		switch token {
		case json.Delim('{'):
			frames = append(frames, &jsonFrame{
				keys:      make(map[string]struct{}),
				expectKey: true,
			})
			continue
		case json.Delim('['):
			frames = append(frames, &jsonFrame{})
			continue
		case json.Delim('}'), json.Delim(']'):
			frames = frames[:len(frames)-1]
			// The closed object or array is a value of the parent object.
			if len(frames) > 0 && frames[len(frames)-1].keys != nil {
				frames[len(frames)-1].expectKey = true
			}
			continue
		}

		if top == nil || top.keys == nil {
			// A value at the top level or in an array.
			continue
		}
		if !top.expectKey {
			// A scalar value of an object. The next token is a key.
			top.expectKey = true
			continue
		}

		key, _ := token.(string)
		_, exists := top.keys[key]
		if exists {
			return errDuplicateKey
		}
		top.keys[key] = struct{}{}
		top.expectKey = false
	}
}
