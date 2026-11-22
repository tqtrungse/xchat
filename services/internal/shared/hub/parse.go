/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package hub

import (
	"errors"
	"fmt"

	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
)

const MaxDescriptorBytes = 4096

// unknownFieldReason is the Reason of the jlexer.LexerError that the
// generated decoder adds for an unknown field. The generator flag
// -disallow_unknown_fields enables this check. See the json task of the
// shared Taskfile.
const unknownFieldReason = "unknown field"

// Parse decodes and validates the public provisioning format. It rejects a
// field that Descriptor does not have, for example a private key. If the data
// has the same field two times, the last value replaces the first value.
//
// The error does not contain a name or a value of the data, because a wrong
// file can contain private data.
func Parse(data []byte) (*Descriptor, error) {
	if len(data) > MaxDescriptorBytes {
		return nil, fmt.Errorf("hub descriptor exceeds %d bytes", MaxDescriptorBytes)
	}

	var descriptor Descriptor
	err := easyjson.Unmarshal(data, &descriptor)
	if err != nil {
		return nil, sanitizeDecodeError(err)
	}

	err = descriptor.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate hub descriptor: %w", err)
	}
	return &descriptor, nil
}

// sanitizeDecodeError keeps only the class of the error and the byte offset.
// jlexer.LexerError.Data contains a part of the input. Reason can contain a
// value of the input too, for example the text of a number that overflows.
// Thus neither field goes into the error.
func sanitizeDecodeError(err error) error {
	lexerErr, ok := errors.AsType[*jlexer.LexerError](err)
	if !ok {
		return errors.New("invalid hub descriptor JSON")
	}
	if lexerErr.Reason == unknownFieldReason {
		return ErrUnknownField
	}
	return fmt.Errorf("invalid hub descriptor JSON at byte %d", lexerErr.Offset)
}
