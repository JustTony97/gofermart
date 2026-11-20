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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func setupMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()
	rawDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	sqlxDB := sqlx.NewDb(rawDB, "sqlmock")
	t.Cleanup(func() {
		_ = sqlxDB.Close()
	})

	return sqlxDB, mock
}

func TestOrderRepository_GetOrders(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC().Truncate(time.Second)
	points := 250.5

	t.Run("returns list of orders for user", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		rows := sqlmock.NewRows([]string{"id", "status", "reward_points", "created_at"}).
			AddRow(int64(12345678903), string(models.StatusProcessed), points, now)

		mock.ExpectQuery(`SELECT id, status, reward_points, created_at FROM orders WHERE user_id = \$1 ORDER BY created_at DESC`).
			WithArgs(userID).
			WillReturnRows(rows)

		got, err := repo.GetOrders(context.Background(), userID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, int64(12345678903), got[0].ID)
		require.Equal(t, string(models.StatusProcessed), got[0].Status)
		require.Equal(t, &points, got[0].Accrual)
		require.NoError(t, mock.ExpectationsWereMet())

	})

	t.Run("returns wrapped error on db failure", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectQuery(`SELECT id, status, reward_points, created_at FROM orders`).
			WithArgs(userID).
			WillReturnError(errors.New("db failure"))

		got, err := repo.GetOrders(context.Background(), userID)

		require.Error(t, err)
		require.Nil(t, got)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestOrderRepository_CreateOrder(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	otherUserID := uuid.Must(uuid.NewV4())
	orderID := int64(12345678903)

	t.Run("successfully inserts new order", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectExec(`INSERT INTO orders \(id, user_id, status\) VALUES \(\$1, \$2, \$3\)`).
			WithArgs(orderID, userID, models.StatusNew).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.CreateOrder(context.Background(), userID, orderID)

		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrOrderAlreadyExists when duplicate order belongs to same user", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		pgUniqueErr := &pgconn.PgError{Code: "23505"}
		mock.ExpectExec(`INSERT INTO orders`).
			WithArgs(orderID, userID, models.StatusNew).
			WillReturnError(pgUniqueErr)

		mock.ExpectQuery(`SELECT user_id FROM orders WHERE id = \$1`).
			WithArgs(orderID).
			WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(userID))

		err := repo.CreateOrder(context.Background(), userID, orderID)

		require.ErrorIs(t, err, models.ErrOrderAlreadyExists)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrOrderAlreadyExistsByAnotherUser when duplicate order belongs to another user", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		pgUniqueErr := &pgconn.PgError{Code: "23505"}
		mock.ExpectExec(`INSERT INTO orders`).
			WithArgs(orderID, userID, models.StatusNew).
			WillReturnError(pgUniqueErr)

		mock.ExpectQuery(`SELECT user_id FROM orders WHERE id = \$1`).
			WithArgs(orderID).
			WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(otherUserID))

		err := repo.CreateOrder(context.Background(), userID, orderID)

		require.ErrorIs(t, err, models.ErrOrderAlreadyExistsByAnotherUser)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns select error if lookup after 23505 fails", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		pgUniqueErr := &pgconn.PgError{Code: "23505"}
		selectErr := errors.New("select failed")

		mock.ExpectExec(`INSERT INTO orders`).
			WithArgs(orderID, userID, models.StatusNew).
			WillReturnError(pgUniqueErr)

		mock.ExpectQuery(`SELECT user_id FROM orders WHERE id = \$1`).
			WithArgs(orderID).
			WillReturnError(selectErr)

		err := repo.CreateOrder(context.Background(), userID, orderID)

		require.ErrorIs(t, err, selectErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns generic db error on insert failure", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		insertErr := errors.New("connection lost")
		mock.ExpectExec(`INSERT INTO orders`).
			WithArgs(orderID, userID, models.StatusNew).
			WillReturnError(insertErr)

		err := repo.CreateOrder(context.Background(), userID, orderID)

		require.ErrorIs(t, err, insertErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
func TestOrderRepository_GetOrderByIDForUpdate(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	orderID := int64(12345678903)

	t.Run("returns order on success", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, status, user_id FROM orders WHERE id = \$1 FOR UPDATE`).
			WithArgs(orderID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "status", "user_id"}).
				AddRow(orderID, string(models.StatusNew), userID))
		mock.ExpectCommit()

		tx, err := db.Beginx()
		require.NoError(t, err)

		got, err := repo.GetOrderByIDForUpdate(context.Background(), tx, orderID)
		require.NoError(t, err)
		require.Equal(t, orderID, got.ID)
		require.Equal(t, userID, got.UserID)
		require.NoError(t, tx.Commit())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrOrderNotFound on sql.ErrNoRows", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, status, user_id FROM orders WHERE id = \$1 FOR UPDATE`).
			WithArgs(orderID).
			WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		tx, err := db.Beginx()
		require.NoError(t, err)

		_, err = repo.GetOrderByIDForUpdate(context.Background(), tx, orderID)
		require.ErrorIs(t, err, models.ErrOrderNotFound)
		_ = tx.Rollback()
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns generic error on db failure", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)
		dbErr := errors.New("lock timeout")

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, status, user_id FROM orders WHERE id = \$1 FOR UPDATE`).
			WithArgs(orderID).
			WillReturnError(dbErr)
		mock.ExpectRollback()

		tx, err := db.Beginx()
		require.NoError(t, err)

		_, err = repo.GetOrderByIDForUpdate(context.Background(), tx, orderID)
		require.ErrorIs(t, err, dbErr)
		_ = tx.Rollback()
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestOrderRepository_GetOrderByStatusForUpdate(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	orderID := int64(12345678903)

	t.Run("returns orderID and userID on success", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, user_id FROM orders WHERE status = \$1 ORDER BY created_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED`).
			WithArgs(models.StatusNew).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(orderID, userID))
		mock.ExpectCommit()

		tx, err := db.Beginx()
		require.NoError(t, err)

		gotID, gotUser, err := repo.GetOrderByStatusForUpdate(context.Background(), tx, models.StatusNew)
		require.NoError(t, err)
		require.Equal(t, orderID, gotID)
		require.Equal(t, userID, gotUser)
		require.NoError(t, tx.Commit())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrOrderNotFound on sql.ErrNoRows", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, user_id FROM orders WHERE status = \$1`).
			WithArgs(models.StatusNew).
			WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		tx, err := db.Beginx()
		require.NoError(t, err)

		_, _, err = repo.GetOrderByStatusForUpdate(context.Background(), tx, models.StatusNew)
		require.ErrorIs(t, err, models.ErrOrderNotFound)
		_ = tx.Rollback()
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns generic error on db failure", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)
		dbErr := errors.New("query failed")

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, user_id FROM orders WHERE status = \$1`).
			WithArgs(models.StatusNew).
			WillReturnError(dbErr)
		mock.ExpectRollback()

		tx, err := db.Beginx()
		require.NoError(t, err)

		_, _, err = repo.GetOrderByStatusForUpdate(context.Background(), tx, models.StatusNew)
		require.ErrorIs(t, err, dbErr)
		_ = tx.Rollback()
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestOrderRepository_Updates(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	orderID := int64(12345678903)

	t.Run("UpdateStatusWithinTx updates status", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE orders SET status = \$1, modified_at = NOW\(\) WHERE id = \$2`).
			WithArgs(models.StatusProcessing, orderID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		tx, err := db.Beginx()
		require.NoError(t, err)

		require.NoError(t, repo.UpdateStatusWithinTx(context.Background(), tx, orderID, models.StatusProcessing))
		require.NoError(t, tx.Commit())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("UpdateOrderAccrualWithinTx updates status and reward points", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE orders SET status = \$1, reward_points = \$2, modified_at = NOW\(\) WHERE id = \$3`).
			WithArgs(models.StatusProcessed, 300.0, orderID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		tx, err := db.Beginx()
		require.NoError(t, err)

		require.NoError(t, repo.UpdateOrderAccrualWithinTx(context.Background(), tx, orderID, models.StatusProcessed, 300.0))
		require.NoError(t, tx.Commit())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("UpdateStatus updates status without tx", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectExec(`UPDATE orders SET status = \$1, modified_at = NOW\(\) WHERE id = \$2`).
			WithArgs(models.StatusInvalid, orderID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, repo.UpdateStatus(context.Background(), orderID, models.StatusInvalid))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("UpdateUserBalanceWithinTx updates balance or returns ErrUserNotFound", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewOrderRepository(db)

		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE users SET balance = balance \+ \$1 WHERE id = \$2`).
			WithArgs(150.0, userID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE users SET balance = balance \+ \$1 WHERE id = \$2`).
			WithArgs(150.0, userID).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		tx, err := db.Beginx()
		require.NoError(t, err)

		require.NoError(t, repo.UpdateUserBalanceWithinTx(context.Background(), tx, userID, 150.0))
		require.ErrorIs(t, repo.UpdateUserBalanceWithinTx(context.Background(), tx, userID, 150.0), models.ErrUserNotFound)
		require.NoError(t, tx.Commit())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
