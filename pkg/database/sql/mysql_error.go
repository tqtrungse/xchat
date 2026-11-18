/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package sql

import (
	"errors"
	"fmt"
	"strings"

	gomysql "github.com/go-sql-driver/mysql"
)

// translateMySQLError converts an error of the MySQL driver to an error of this
// package that does not depend on the driver:
//
//   - 1205 (lock wait timeout) and 1213 (deadlock) give ErrLockConflict.
//   - 1062 (duplicate entry) gives a *DuplicateKeyError.
//
// The result keeps the error of the driver in its chain. Therefore, errors.As
// and IsMySQLInfraError still find the driver error. An error outside the list
// above returns unchanged. A translated error also returns unchanged.
func translateMySQLError(err error) error {
	if err == nil ||
		errors.Is(err, ErrLockConflict) ||
		errors.As(err, new(*DuplicateKeyError)) {
		return err
	}

	myErr, ok := errors.AsType[*gomysql.MySQLError](err)
	if !ok {
		return err
	}

	switch myErr.Number {
	case mysqlErrLockWaitTimeout,
		mysqlErrDeadlock:
		return fmt.Errorf("%w: %w", ErrLockConflict, err)

	case mysqlErrDuplicateEntry:
		var key string
		key, ok = parseDuplicateKey(myErr.Message)
		if !ok {
			return err
		}
		return &DuplicateKeyError{
			Key: key,
			Err: err,
		}

	default:
		return err
	}
}

// parseDuplicateKey reads the name of the constraint from the message of a
// duplicate entry error, for example "Duplicate entry '1' for key
// 'users.PRIMARY'". MySQL 8.0.19 and later write the key as "<table>.<key>". An
// earlier server writes only the key. The result is the key, in both formats.
// ok is false if the message does not contain the key.
func parseDuplicateKey(message string) (key string, ok bool) {
	const marker = " for key '"

	// The duplicate value can contain the marker. Therefore, the parser reads
	// from the end.
	index := strings.LastIndex(message, marker)
	if index < 0 {
		return "", false
	}

	key, _, ok = strings.Cut(message[index+len(marker):], "'")
	if !ok || key == "" {
		return "", false
	}

	if dot := strings.LastIndexByte(key, '.'); dot >= 0 {
		key = key[dot+1:]
	}
	return key, key != ""
}
