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

	sharedpassword "xchat/internal/shared/password"

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
// NewBcrypt uses max(1, runtime.GOMAXPROCS(0)/2).
func NewBcrypt(limit int) Bcrypt {
	if limit <= 0 {
		limit = max(1, runtime.GOMAXPROCS(0)/2)
	}
	return Bcrypt{
		slots: make(chan struct{}, limit),
	}
}

// Hash hashes password with bcrypt and gives the hash as a 60-byte string.
// The string has the form: $2a$<cost>$<salt and hash>. Hash generates a new
// random salt for each call. It returns ErrBcryptPasswordTooLong if password
// is longer than 72 bytes. It returns sharedpassword.ErrBusy if ctx ends before a slot
// is free.
func (b *Bcrypt) Hash(ctx context.Context, password []byte) ([]byte, error) {
	if len(password) > bcryptMaxPasswordLength {
		return nil, ErrBcryptPasswordTooLong
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
// and the salt from hash. It returns ErrBcryptPasswordTooLong if password is
// longer than 72 bytes. A longer password must not pass, because bcrypt
// ignores the extra bytes. It returns sharedpassword.ErrBusy if ctx ends before a slot
// is free. It returns sharedpassword.ErrMismatch if the password does not match. It
// returns ErrBcryptInvalidHash if hash is not valid.
func (b *Bcrypt) Verify(ctx context.Context, password []byte, hash []byte) error {
	if len(password) > bcryptMaxPasswordLength {
		return ErrBcryptPasswordTooLong
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
		return sharedpassword.ErrMismatch
	}

	return fmt.Errorf("%w: %v", ErrBcryptInvalidHash, err)
}

// acquire waits for a free slot. It returns sharedpassword.ErrBusy, which wraps the
// error of ctx, if ctx ends first. A Bcrypt that NewBcrypt did not create has
// no limit, and acquire returns at once.
func (b *Bcrypt) acquire(ctx context.Context) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("%w: %w", sharedpassword.ErrBusy, err)
	}
	if b.slots == nil {
		return nil
	}

	select {
	case b.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", sharedpassword.ErrBusy, ctx.Err())
	}
}

// release frees the slot that acquire took.
func (b *Bcrypt) release() {
	if b.slots == nil {
		return
	}
	<-b.slots
}
