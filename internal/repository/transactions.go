package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
)

type TransactionRepository struct {
	db *sqlx.DB
}

func NewTransactionRepository(db *sqlx.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) GetUserBalanceForUpdateWithinTx(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID) (float64, error) {
	query := `SELECT balance FROM users WHERE id = $1 FOR UPDATE`
	var balance float64
	err := tx.GetContext(ctx, &balance, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, models.ErrUserNotFound
		}
		return 0, err
	}
	return balance, nil
}

func (r *TransactionRepository) UpdateUserBalanceWithinTx(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, amount float64) error {
	query := `
		UPDATE users 
		SET balance = balance - $1
		WHERE id = $2
	`
	_, err := tx.ExecContext(ctx, query, amount, userID)
	return err
}

func (r *TransactionRepository) RecordTransactionWithinTx(ctx context.Context, tx *sqlx.Tx, orderID int64, userID uuid.UUID, amount float64) error {
	query := `
		INSERT INTO transactions (user_id, order_id, amount, type) 
		VALUES ($1, $2, $3, $4)
	`
	_, err := tx.ExecContext(ctx, query, userID, orderID, amount, "WITHDRAWAL")
	return err
}

func (r *TransactionRepository) GetUserBalance(ctx context.Context, userID uuid.UUID) (models.UserBalanceResponse, error) {
	query := `
		SELECT 
			u.balance as current,
			COALESCE(SUM(t.amount), 0.00) as withdrawn
		FROM users u
		LEFT JOIN transactions t ON t.user_id = u.id AND t.type = 'WITHDRAWAL'
		WHERE u.id = $1
		GROUP BY u.id, u.balance
	`
	var resp models.UserBalanceResponse
	err := r.db.GetContext(ctx, &resp, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.UserBalanceResponse{}, models.ErrUserNotFound
		}
		return models.UserBalanceResponse{}, fmt.Errorf("failed to get user balance mapping: %w", err)
	}
	return resp, nil
}

func (r *TransactionRepository) GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]models.WithdrawalResponse, error) {
	query := `
		SELECT order_id as order, amount as sum, processed_at
		FROM transactions
		WHERE user_id = $1 AND type = 'WITHDRAWAL'
		ORDER BY processed_at ASC
	`
	var withdrawals []models.WithdrawalResponse
	err := r.db.SelectContext(ctx, &withdrawals, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to select withdrawals log: %w", err)
	}
	return withdrawals, nil
}
