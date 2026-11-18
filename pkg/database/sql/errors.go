/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package sql

import (
	"errors"
	"fmt"
)

var (
	ErrDBNotOpen       = errors.New("db not open")
	ErrDBClosed        = errors.New("db closed")
	ErrExtractDB       = errors.New("failed to extract DB")
	ErrInvalidTxSource = errors.New("invalid transaction source")

	// ErrLockConflict means that the server could not get a lock in time (lock
	// wait timeout) or chose this transaction as the victim of a deadlock. The
	// failure shows contention, not an outage. The caller can try again.
	ErrLockConflict = errors.New("lock conflict")
)

// DuplicateKeyError means that a write broke a unique constraint, such as the
// primary key or a unique index.
type DuplicateKeyError struct {
	// Key is the name of the broken constraint, without the name of the table.
	// For example: "PRIMARY" or "unique_index_users_on_username".
	Key string

	// Err is the error of the driver. It can be nil.
	Err error
}

func (e *DuplicateKeyError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("duplicate entry for key %q", e.Key)
	}
	return fmt.Sprintf("duplicate entry for key %q: %v", e.Key, e.Err)
}

func (e *DuplicateKeyError) Unwrap() error {
	return e.Err
}

// IsLockConflict reports whether err is, or wraps, ErrLockConflict.
//
// The DB that NewMySql returns translates the errors of the driver. Therefore,
// a repository gets this result without code that knows the driver.
// IsLockConflict returns false for a raw driver error that no layer translated.
func IsLockConflict(err error) bool {
	return errors.Is(err, ErrLockConflict)
}

// DuplicateKey gives the name of the constraint that a DuplicateKeyError
// reports. ok is false if err is not a DuplicateKeyError and does not wrap one.
//
// The DB that NewMySql returns translates the errors of the driver. DuplicateKey
// returns false for a raw driver error that no layer translated.
func DuplicateKey(err error) (key string, ok bool) {
	duplicate, ok := errors.AsType[*DuplicateKeyError](err)
	if !ok {
		return "", false
	}
	return duplicate.Key, true
}
