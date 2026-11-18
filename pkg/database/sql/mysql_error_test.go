/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package sql

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestIsLockConflict(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "sentinel",
			err:  ErrLockConflict,
			want: true,
		},
		{
			name: "wrapped sentinel",
			err:  fmt.Errorf("transaction: %w", ErrLockConflict),
			want: true,
		},
		{
			name: "raw driver error is not translated",
			err:  &gomysql.MySQLError{Number: 1205},
			want: false,
		},
		{
			name: "duplicate key",
			err:  &DuplicateKeyError{Key: "PRIMARY"},
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("boom"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.want, IsLockConflict(test.err))
		})
	}
}

func TestDuplicateKey(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		err     error
		wantKey string
		wantOK  bool
	}{
		{
			name:    "duplicate key error",
			err:     &DuplicateKeyError{Key: "PRIMARY"},
			wantKey: "PRIMARY",
			wantOK:  true,
		},
		{
			name:    "wrapped duplicate key error",
			err:     fmt.Errorf("insert: %w", &DuplicateKeyError{Key: "unique_index_users_on_username"}),
			wantKey: "unique_index_users_on_username",
			wantOK:  true,
		},
		{
			name:   "raw driver error is not translated",
			err:    &gomysql.MySQLError{Number: 1062, Message: "Duplicate entry '1' for key 'users.PRIMARY'"},
			wantOK: false,
		},
		{
			name:   "gorm error",
			err:    gorm.ErrDuplicatedKey,
			wantOK: false,
		},
		{
			name:   "lock conflict",
			err:    ErrLockConflict,
			wantOK: false,
		},
		{
			name:   "plain error",
			err:    errors.New("boom"),
			wantOK: false,
		},
		{
			name:   "nil",
			err:    nil,
			wantOK: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			key, ok := DuplicateKey(test.err)
			require.Equal(t, test.wantOK, ok)
			require.Equal(t, test.wantKey, key)
		})
	}
}

func TestDuplicateKeyErrorMessage(t *testing.T) {
	t.Parallel()

	cause := errors.New("driver")
	err := &DuplicateKeyError{Key: "PRIMARY", Err: cause}
	require.ErrorIs(t, err, cause)
	require.Contains(t, err.Error(), "PRIMARY")
	require.Contains(t, err.Error(), "driver")

	require.Equal(t, `duplicate entry for key "PRIMARY"`, (&DuplicateKeyError{Key: "PRIMARY"}).Error())
}

func TestTranslateMySQLError(t *testing.T) {
	t.Parallel()

	lockWait := &gomysql.MySQLError{Number: 1205, Message: "Lock wait timeout exceeded"}
	deadlock := &gomysql.MySQLError{Number: 1213, Message: "Deadlock found"}
	duplicate := func(message string) *gomysql.MySQLError {
		return &gomysql.MySQLError{Number: 1062, Message: message}
	}

	t.Run("lock conflict", func(t *testing.T) {
		t.Parallel()

		for name, cause := range map[string]*gomysql.MySQLError{
			"lock wait timeout": lockWait,
			"deadlock":          deadlock,
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				got := translateMySQLError(fmt.Errorf("query: %w", cause))
				require.True(t, IsLockConflict(got))
				require.ErrorIs(t, got, cause, "the error of the driver must stay in the chain")

				_, isDuplicate := DuplicateKey(got)
				require.False(t, isDuplicate)
			})
		}
	})

	t.Run("duplicate key", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name    string
			cause   *gomysql.MySQLError
			wantKey string
			wantOK  bool
		}{
			{
				name:    "primary key",
				cause:   duplicate("Duplicate entry '1' for key 'users.PRIMARY'"),
				wantKey: "PRIMARY",
				wantOK:  true,
			},
			{
				name:    "unique index",
				cause:   duplicate("Duplicate entry 'alice' for key 'users.unique_index_users_on_username'"),
				wantKey: "unique_index_users_on_username",
				wantOK:  true,
			},
			{
				name:    "server before 8.0.19 writes only the key",
				cause:   duplicate("Duplicate entry '1' for key 'PRIMARY'"),
				wantKey: "PRIMARY",
				wantOK:  true,
			},
			{
				name:    "value contains the marker",
				cause:   duplicate("Duplicate entry 'x for key 'fake' for key 'users.PRIMARY'"),
				wantKey: "PRIMARY",
				wantOK:  true,
			},
			{
				name:   "message without the key",
				cause:  duplicate("duplicate entry"),
				wantOK: false,
			},
			{
				name:   "message without the closing quote",
				cause:  duplicate("Duplicate entry '1' for key 'users.PRIMARY"),
				wantOK: false,
			},
			{
				name:   "empty key",
				cause:  duplicate("Duplicate entry '1' for key ''"),
				wantOK: false,
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				got := translateMySQLError(fmt.Errorf("insert: %w", test.cause))
				key, ok := DuplicateKey(got)
				require.Equal(t, test.wantOK, ok)
				require.Equal(t, test.wantKey, key)
				require.False(t, IsLockConflict(got))
				require.ErrorIs(t, got, test.cause, "the error of the driver must stay in the chain")
			})
		}
	})

	t.Run("unchanged", func(t *testing.T) {
		t.Parallel()

		for name, err := range map[string]error{
			"other driver error": &gomysql.MySQLError{Number: 1040},
			"foreign key":        &gomysql.MySQLError{Number: 1452},
			"gorm error":         gorm.ErrRecordNotFound,
			"plain error":        errors.New("boom"),
			"nil":                nil,
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				require.Equal(t, err, translateMySQLError(err))
			})
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		t.Parallel()

		for name, err := range map[string]error{
			"lock conflict": translateMySQLError(lockWait),
			"duplicate":     translateMySQLError(duplicate("Duplicate entry '1' for key 'users.PRIMARY'")),
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				require.Same(t, err, translateMySQLError(err))
			})
		}
	})
}

// TestMySQL_TranslatesDriverErrors checks the error translation of the DB that
// Open makes. The DB must give translated errors to the caller.
func TestMySQL_TranslatesDriverErrors(t *testing.T) {
	t.Parallel()

	const statement = "INSERT INTO t VALUES (1)"

	for _, test := range []struct {
		name  string
		cause error
		check func(t *testing.T, err error)
	}{
		{
			name:  "duplicate key",
			cause: &gomysql.MySQLError{Number: 1062, Message: "Duplicate entry '1' for key 't.PRIMARY'"},
			check: func(t *testing.T, err error) {
				key, ok := DuplicateKey(err)
				require.True(t, ok)
				require.Equal(t, "PRIMARY", key)
			},
		},
		{
			name:  "deadlock",
			cause: &gomysql.MySQLError{Number: 1213, Message: "Deadlock found"},
			check: func(t *testing.T, err error) {
				require.True(t, IsLockConflict(err))
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			f := newMySQLRoutingFixture(t, false)
			f.primaryMock.
				ExpectExec("^" + regexp.QuoteMeta(statement) + "$").
				WillReturnError(test.cause)

			err := f.db.WithDB(t.Context(), TxSourcePrimary, func(ctx context.Context) error {
				handle, extractErr := ExtractDB(ctx)
				require.NoError(t, extractErr)
				return handle.(*gorm.DB).WithContext(ctx).Exec(statement).Error
			})
			require.Error(t, err)
			require.ErrorIs(t, err, test.cause, "the error of the driver must stay in the chain")
			test.check(t, err)
		})
	}
}

// The dialector must implement gorm.ErrorTranslator. GORM calls Translate only
// when the dialector has this interface.
var _ gorm.ErrorTranslator = (*mysqlDialector)(nil)
