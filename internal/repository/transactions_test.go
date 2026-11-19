package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/repository"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestTransactionRepository_WithinTxMethods(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	orderID := int64(12345678903)

	t.Run("GetUserBalanceForUpdateWithinTx returns balance or ErrUserNotFound", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewTransactionRepository(db)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT balance FROM users WHERE id = \$1 FOR UPDATE`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(500.75))
		mock.ExpectCommit()

		tx, err := db.Beginx()
		require.NoError(t, err)

		bal, err := repo.GetUserBalanceForUpdateWithinTx(context.Background(), tx, userID)
		require.NoError(t, err)
		require.Equal(t, 500.75, bal)
		require.NoError(t, tx.Commit())

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT balance FROM users WHERE id = \$1 FOR UPDATE`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		tx2, err := db.Beginx()
		require.NoError(t, err)

		_, err = repo.GetUserBalanceForUpdateWithinTx(context.Background(), tx2, userID)
		require.ErrorIs(t, err, models.ErrUserNotFound)
		_ = tx2.Rollback()

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("UpdateUserBalanceWithinTx and RecordTransactionWithinTx execute queries", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewTransactionRepository(db)

		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE users SET balance = balance - \$1 WHERE id = \$2`).
			WithArgs(150.5, userID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO transactions \(user_id, order_id, amount, type\) VALUES \(\$1, \$2, \$3, \$4\)`).
			WithArgs(userID, orderID, 150.5, "WITHDRAWAL").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		tx, err := db.Beginx()
		require.NoError(t, err)

		require.NoError(t, repo.UpdateUserBalanceWithinTx(context.Background(), tx, userID, 150.5))
		require.NoError(t, repo.RecordTransactionWithinTx(context.Background(), tx, orderID, userID, 150.5))
		require.NoError(t, tx.Commit())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTransactionRepository_GetUserBalance(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())

	t.Run("returns current and withdrawn balance", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewTransactionRepository(db)

		rows := sqlmock.NewRows([]string{"current", "withdrawn"}).AddRow(350.0, 150.5)
		mock.ExpectQuery(`SELECT u\.balance as current, COALESCE\(SUM\(t\.amount\), 0\.00\) as withdrawn FROM users u`).
			WithArgs(userID).
			WillReturnRows(rows)

		got, err := repo.GetUserBalance(context.Background(), userID)
		require.NoError(t, err)
		require.Equal(t, 350.0, got.Current)
		require.Equal(t, 150.5, got.Withdrawn)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrUserNotFound on sql.ErrNoRows", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewTransactionRepository(db)

		mock.ExpectQuery(`SELECT u\.balance as current`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		_, err := repo.GetUserBalance(context.Background(), userID)
		require.ErrorIs(t, err, models.ErrUserNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns wrapped error on db failure", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewTransactionRepository(db)

		mock.ExpectQuery(`SELECT u\.balance as current`).
			WithArgs(userID).
			WillReturnError(errors.New("db error"))

		_, err := repo.GetUserBalance(context.Background(), userID)
		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTransactionRepository_GetWithdrawals(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC().Truncate(time.Second)

	t.Run("returns list of withdrawals", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewTransactionRepository(db)

		rows := sqlmock.NewRows([]string{"order", "sum", "processed_at"}).
			AddRow(int64(12345678903), 150.5, now)

		mock.ExpectQuery(`SELECT order_id as order, amount as sum, processed_at FROM transactions WHERE user_id = \$1 AND type = 'WITHDRAWAL' ORDER BY processed_at DESC`).
			WithArgs(userID).
			WillReturnRows(rows)

		got, err := repo.GetWithdrawals(context.Background(), userID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, int64(12345678903), got[0].Order)
		require.Equal(t, 150.5, got[0].Sum)
		require.Equal(t, now, got[0].ProcessedAt)
		require.NoError(t, mock.ExpectationsWereMet())

	})

	t.Run("returns error on db failure", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewTransactionRepository(db)

		mock.ExpectQuery(`SELECT order_id as order`).
			WithArgs(userID).
			WillReturnError(errors.New("select error"))

		got, err := repo.GetWithdrawals(context.Background(), userID)
		require.Error(t, err)
		require.Nil(t, got)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
