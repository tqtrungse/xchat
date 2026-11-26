/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package crypto

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"xchat/services/iam/internal/application/device/register"

	"golang.org/x/crypto/bcrypt"
)

const (
	// bcryptCost is the work factor of bcrypt. Each increment doubles the time.
	bcryptCost = 12

	// bcryptMaxPasswordLength is the maximum password length in bytes. bcrypt
	// ignores the bytes after this limit.
	bcryptMaxPasswordLength = 72
)

// Bcrypt hashes and verifies passwords with bcrypt. A bcrypt operation uses
// one CPU core for a long time. Bcrypt limits the number of operations that
// run at the same time. A copy of Bcrypt shares the limit with the original.
type Bcrypt struct {
	slots chan struct{}
}

// NewBcrypt creates a Bcrypt hasher. The argument limit is the maximum number
// of bcrypt operations that run at the same time. If limit is zero or less,
// NewBcrypt uses max(1, runtime.GOMAXPROCS(0)*3/4). The rest of the cores
// stay free for the other work of the process.
func NewBcrypt(limit int) Bcrypt {
	if limit <= 0 {
		limit = max(1, runtime.GOMAXPROCS(0)*3/4)
	}
	return Bcrypt{
		slots: make(chan struct{}, limit),
	}
}

// Hash hashes password with bcrypt and gives the hash as a 60-byte string.
// The string has the form: $2a$<cost>$<salt and hash>. Hash generates a new
// random salt for each call. It returns register.ErrPasswordTooLong if password
// is longer than 72 bytes. It returns register.ErrPasswordHasherBusy if ctx
// ends before a slot is free.
func (b *Bcrypt) Hash(ctx context.Context, password []byte) ([]byte, error) {
	if len(password) > bcryptMaxPasswordLength {
		return nil, register.ErrPasswordTooLong
	}

	err := b.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer b.release()

	hash, err := bcrypt.GenerateFromPassword(password, bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("bcrypt hash: %w", err)
	}
	return hash, nil
}

// Verify checks password against the stored bcrypt hash. Verify reads the cost
// and the salt from hash. It returns register.ErrPasswordTooLong if password is
// longer than 72 bytes. A longer password must not pass, because bcrypt
// ignores the extra bytes. It returns register.ErrPasswordHasherBusy if ctx
// ends before a slot is free. It returns register.ErrPasswordMismatch if the
// password does not match. It returns ErrBcryptInvalidHash if hash is not
// valid.
func (b *Bcrypt) Verify(ctx context.Context, password []byte, hash []byte) error {
	if len(password) > bcryptMaxPasswordLength {
		return register.ErrPasswordTooLong
	}

	err := b.acquire(ctx)
	if err != nil {
		return err
	}
	defer b.release()

	err = bcrypt.CompareHashAndPassword(hash, password)
	if err == nil {
		return nil
	}

	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return register.ErrPasswordMismatch
	}

	return fmt.Errorf("%w: %v", ErrBcryptInvalidHash, err)
}

// acquire waits for a free slot. It returns register.ErrPasswordHasherBusy,
// which wraps the error of ctx, if ctx ends first. A Bcrypt that NewBcrypt did
// not create has no limit, and acquire returns at once.
func (b *Bcrypt) acquire(ctx context.Context) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("%w: %w", register.ErrPasswordHasherBusy, err)
	}
	if b.slots == nil {
		return nil
	}

	select {
	case b.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", register.ErrPasswordHasherBusy, ctx.Err())
	}
}

// release frees the slot that acquire took.
func (b *Bcrypt) release() {
	if b.slots == nil {
		return
	}
	<-b.slots
}
