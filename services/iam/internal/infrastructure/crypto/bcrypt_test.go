/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package crypto

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"xchat/services/iam/internal/application/device/register"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestBcryptHash(t *testing.T) {
	t.Parallel()

	const password = "correct horse battery staple"

	hasher := NewBcrypt(1)
	hash, err := hasher.Hash(context.Background(), []byte(password))
	require.NoError(t, err)
	require.Len(t, hash, 60)
	require.True(t, strings.HasPrefix(string(hash), "$2a$12$"))

	cost, err := bcrypt.Cost(hash)
	require.NoError(t, err)
	require.Equal(t, bcryptCost, cost)

	otherHash, err := hasher.Hash(context.Background(), []byte(password))
	require.NoError(t, err)
	require.NotEqual(t, hash, otherHash, "a new salt must change the hash")
}

func TestBcryptHashPasswordLength(t *testing.T) {
	t.Parallel()

	hasher := NewBcrypt(1)

	hash, err := hasher.Hash(context.Background(), []byte(strings.Repeat("a", bcryptMaxPasswordLength)))
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	hash, err = hasher.Hash(context.Background(), []byte(strings.Repeat("a", bcryptMaxPasswordLength+1)))
	require.ErrorIs(t, err, register.ErrPasswordTooLong)
	require.Nil(t, hash)
}

// TestBcryptLongPasswordDoesNotWaitForSlot checks that Hash and Verify reject a
// long password before they wait for a slot. A request that the hasher rejects
// must not use the capacity of the hasher.
func TestBcryptLongPasswordDoesNotWaitForSlot(t *testing.T) {
	t.Parallel()

	password := []byte(strings.Repeat("a", bcryptMaxPasswordLength+1))

	hasher := NewBcrypt(1)
	require.NoError(t, hasher.acquire(context.Background()))
	defer hasher.release()

	// The only slot is in use. A check after acquire gives ErrPasswordHasherBusy.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	hash, err := hasher.Hash(ctx, password)
	require.ErrorIs(t, err, register.ErrPasswordTooLong)
	require.NotErrorIs(t, err, register.ErrPasswordHasherBusy)
	require.Nil(t, hash)

	err = hasher.Verify(ctx, password, []byte("hash"))
	require.ErrorIs(t, err, register.ErrPasswordTooLong)
	require.NotErrorIs(t, err, register.ErrPasswordHasherBusy)
}

func TestBcryptVerify(t *testing.T) {
	t.Parallel()

	const password = "correct horse battery staple"

	hasher := NewBcrypt(1)
	hash, err := hasher.Hash(context.Background(), []byte(password))
	require.NoError(t, err)

	require.NoError(t, hasher.Verify(context.Background(), []byte(password), hash))
	require.ErrorIs(t, hasher.Verify(context.Background(), []byte("wrong password"), hash), register.ErrPasswordMismatch)
	require.ErrorIs(t, hasher.Verify(context.Background(), nil, hash), register.ErrPasswordMismatch)
}

func TestBcryptVerifyRejectsLongPassword(t *testing.T) {
	t.Parallel()

	password := []byte(strings.Repeat("a", bcryptMaxPasswordLength))

	hasher := NewBcrypt(1)
	hash, err := hasher.Hash(context.Background(), password)
	require.NoError(t, err)

	require.NoError(t, hasher.Verify(context.Background(), password, hash))
	require.ErrorIs(t, hasher.Verify(context.Background(), append(password, 'x'), hash), register.ErrPasswordTooLong)
}

func TestBcryptVerifyUsesHashCost(t *testing.T) {
	t.Parallel()

	password := []byte("password")
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.MinCost)
	require.NoError(t, err)

	hasher := NewBcrypt(1)
	require.NoError(t, hasher.Verify(context.Background(), password, hash))
	require.ErrorIs(t, hasher.Verify(context.Background(), []byte("wrong"), hash), register.ErrPasswordMismatch)
}

func TestBcryptVerifyInvalidHash(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		hash string
	}{
		{
			name: "empty",
			hash: "",
		},
		{
			name: "too short",
			hash: "$2a$12$abc",
		},
		{
			name: "wrong prefix",
			hash: "$argon2id$v=19$m=65536,t=3,p=1$c2FsdA$aGFzaA",
		},
		{
			name: "invalid cost",
			hash: "$2a$99$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012345",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			hasher := NewBcrypt(1)
			err := hasher.Verify(context.Background(), []byte("password"), []byte(test.hash))
			require.ErrorIs(t, err, ErrBcryptInvalidHash)
		})
	}
}

func TestNewBcryptDefaultLimit(t *testing.T) {
	t.Parallel()

	for _, limit := range []int{0, -1} {
		hasher := NewBcrypt(limit)
		require.Equal(t, max(1, runtime.GOMAXPROCS(0)*3/4), cap(hasher.slots))
	}

	hasher := NewBcrypt(3)
	require.Equal(t, 3, cap(hasher.slots))
}

func TestBcryptBusyWhenNoSlotIsFree(t *testing.T) {
	t.Parallel()

	hasher := NewBcrypt(1)
	require.NoError(t, hasher.acquire(context.Background()))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	hash, err := hasher.Hash(ctx, []byte("password"))
	require.ErrorIs(t, err, register.ErrPasswordHasherBusy)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, hash)

	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err = hasher.Verify(ctx, []byte("password"), []byte("hash"))
	require.ErrorIs(t, err, register.ErrPasswordHasherBusy)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	hasher.release()

	hash, err = hasher.Hash(context.Background(), []byte("password"))
	require.NoError(t, err)
	require.NotEmpty(t, hash)
}

func TestBcryptBusyWhenContextIsDone(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	hasher := NewBcrypt(1)
	hash, err := hasher.Hash(ctx, []byte("password"))
	require.ErrorIs(t, err, register.ErrPasswordHasherBusy)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, hash)

	require.Len(t, hasher.slots, 0, "a failed call must not keep a slot")
}

func TestBcryptReleasesSlotAfterError(t *testing.T) {
	t.Parallel()

	hasher := NewBcrypt(1)
	for range 3 {
		err := hasher.Verify(context.Background(), []byte("password"), []byte("invalid"))
		require.ErrorIs(t, err, ErrBcryptInvalidHash)
	}
	require.Len(t, hasher.slots, 0)
}

func TestBcryptLimitsConcurrentOperations(t *testing.T) {
	t.Parallel()

	const (
		limit      = 3
		goroutines = 50
	)

	var (
		hasher    = NewBcrypt(limit)
		running   atomic.Int32
		maxActive atomic.Int32
		waitGroup sync.WaitGroup
	)

	for range goroutines {
		waitGroup.Go(func() {
			err := hasher.acquire(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			defer hasher.release()

			active := running.Add(1)
			defer running.Add(-1)

			for {
				previous := maxActive.Load()
				if active <= previous || maxActive.CompareAndSwap(previous, active) {
					break
				}
			}
			time.Sleep(time.Millisecond)
		})
	}
	waitGroup.Wait()

	require.LessOrEqual(t, int(maxActive.Load()), limit)
	require.Len(t, hasher.slots, 0)
}

func TestBcryptZeroValueHasNoLimit(t *testing.T) {
	t.Parallel()

	var hasher Bcrypt
	hash, err := hasher.Hash(context.Background(), []byte("password"))
	require.NoError(t, err)
	require.NoError(t, hasher.Verify(context.Background(), []byte("password"), hash))
}

func BenchmarkBcryptHash(b *testing.B) {
	benchmarkBcrypt(b, func(hasher *Bcrypt, password []byte, _ []byte) error {
		_, err := hasher.Hash(context.Background(), password)
		return err
	})
}

func BenchmarkBcryptVerify(b *testing.B) {
	benchmarkBcrypt(b, func(hasher *Bcrypt, password []byte, hash []byte) error {
		return hasher.Verify(context.Background(), password, hash)
	})
}

// benchmarkBcrypt runs operation for several limits and for several numbers of
// goroutines for each CPU core. The cases show that a limit above the number
// of cores does not increase throughput.
func benchmarkBcrypt(b *testing.B, operation func(hasher *Bcrypt, password []byte, hash []byte) error) {
	password := []byte("correct horse battery staple")
	hash, err := bcrypt.GenerateFromPassword(password, bcryptCost)
	require.NoError(b, err)

	cores := runtime.GOMAXPROCS(0)
	for _, limitCase := range []struct {
		name  string
		limit int
	}{
		{
			name:  "limit=1",
			limit: 1,
		},
		{
			name:  "limit=cores",
			limit: cores,
		},
		{
			name:  "limit=4xcores",
			limit: 4 * cores,
		},
	} {
		for _, parallelism := range []int{1, 8} {
			name := fmt.Sprintf("%s/goroutines=%dxcores", limitCase.name, parallelism)
			b.Run(name, func(b *testing.B) {
				hasher := NewBcrypt(limitCase.limit)

				b.SetParallelism(parallelism)
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						if err := operation(&hasher, password, hash); err != nil {
							b.Error(err)
							return
						}
					}
				})
			})
		}
	}
}
