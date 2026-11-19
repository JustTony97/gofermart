package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JustTony97/gofermart/internal/repository"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestTxManager_BeginFunc(t *testing.T) {
	t.Run("commits transaction when fn succeeds", func(t *testing.T) {
		db, mock := setupMockDB(t)
		txMgr := repository.NewTxManager(db)

		mock.ExpectBegin()
		mock.ExpectCommit()

		called := false
		err := txMgr.BeginFunc(context.Background(), func(tx *sqlx.Tx) error {
			called = true
			require.NotNil(t, tx)
			return nil
		})

		require.NoError(t, err)
		require.True(t, called)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back transaction when fn returns error", func(t *testing.T) {
		db, mock := setupMockDB(t)
		txMgr := repository.NewTxManager(db)
		fnErr := errors.New("business error")

		mock.ExpectBegin()
		mock.ExpectRollback()

		err := txMgr.BeginFunc(context.Background(), func(tx *sqlx.Tx) error {
			return fnErr
		})

		require.ErrorIs(t, err, fnErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back transaction and re-panics when fn panics", func(t *testing.T) {
		db, mock := setupMockDB(t)
		txMgr := repository.NewTxManager(db)

		mock.ExpectBegin()
		mock.ExpectRollback()

		require.PanicsWithValue(t, "something went wrong", func() {
			_ = txMgr.BeginFunc(context.Background(), func(tx *sqlx.Tx) error {
				panic("something went wrong")
			})
		})

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when BeginTxx fails", func(t *testing.T) {
		db, mock := setupMockDB(t)
		txMgr := repository.NewTxManager(db)
		beginErr := errors.New("connection closed")

		mock.ExpectBegin().WillReturnError(beginErr)

		err := txMgr.BeginFunc(context.Background(), func(tx *sqlx.Tx) error {
			t.Fatal("fn should not be called")
			return nil
		})

		require.ErrorIs(t, err, beginErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when Commit fails", func(t *testing.T) {
		db, mock := setupMockDB(t)
		txMgr := repository.NewTxManager(db)
		commitErr := errors.New("commit failed")

		mock.ExpectBegin()
		mock.ExpectCommit().WillReturnError(commitErr)

		err := txMgr.BeginFunc(context.Background(), func(tx *sqlx.Tx) error {
			return nil
		})

		require.ErrorIs(t, err, commitErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
