package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

type OrderRepository struct {
	db *sqlx.DB
}

func NewOrderRepository(db *sqlx.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) GetOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error) {
	query := `
		SELECT id, status, reward_points, created_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC;
	`
	var orders []models.Order
	err := r.db.SelectContext(ctx, &orders, query, userID)
	if err != nil {
		return nil, fmt.Errorf("select orders by user id: %w", err)
	}

	return orders, nil
}

func (r *OrderRepository) CreateOrder(ctx context.Context, userID uuid.UUID, orderID int64) error {
	query := `
		INSERT INTO orders (id, user_id, status) 
		VALUES ($1, $2, $3) 
	`
	_, err := r.db.ExecContext(ctx, query, orderID, userID, models.StatusNew)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return models.ErrOrderAlreadyExists
		}
		return err
	}
	return nil
}

func (r *OrderRepository) GetOrderByIDForUpdate(ctx context.Context, tx *sqlx.Tx, orderID int64) (models.Order, error) {
	query := `
		SELECT id, status, user_id
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`
	var order models.Order
	err := tx.GetContext(ctx, &order, query, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Order{}, models.ErrOrderNotFound
		}
		return models.Order{}, err
	}
	return order, nil
}

func (r *OrderRepository) GetOrderByStatusForUpdate(ctx context.Context, tx *sqlx.Tx, status models.OrderStatus) (int64, uuid.UUID, error) {
	query := `
		SELECT id, user_id
		FROM orders 
		WHERE status = $1
		ORDER BY created_at ASC 
		LIMIT 1 
		FOR UPDATE SKIP LOCKED
	`
	var res struct {
		ID     int64     `db:"id"`
		UserID uuid.UUID `db:"user_id"`
	}
	err := tx.GetContext(ctx, &res, query, status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, uuid.Nil, models.ErrOrderNotFound
		}
		return 0, uuid.Nil, err
	}
	return res.ID, res.UserID, nil
}

func (r *OrderRepository) UpdateStatusWithinTx(ctx context.Context, tx *sqlx.Tx, orderID int64, status models.OrderStatus) error {
	query := `
		UPDATE orders
		SET status = $1,
			modified_at = NOW()
		WHERE id = $2
	`
	_, err := tx.ExecContext(ctx, query, status, orderID)
	return err
}

func (r *OrderRepository) UpdateOrderAccrualWithinTx(ctx context.Context, tx *sqlx.Tx, orderID int64, status models.OrderStatus, accrual float64) error {
	query := `
		UPDATE orders
		SET status = $1,
			reward_points = $2,
			modified_at = NOW()
		WHERE id = $3
	`
	_, err := tx.ExecContext(ctx, query, status, accrual, orderID)
	return err
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, orderID int64, status models.OrderStatus) error {
	query := `
		UPDATE orders
		SET status = $1,
			modified_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, status, orderID)
	return err
}

func (r *OrderRepository) UpdateUserBalanceWithinTx(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, amount float64) error {
	query := `
		UPDATE users
		SET balance = balance + $1
		WHERE id = $2
	`
	res, err := tx.ExecContext(ctx, query, amount, userID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return models.ErrUserNotFound
	}

	return nil
}
