package repository

import (
	"context"
	"fmt"

	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
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
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query orders by user id: %w", err)
	}
	defer rows.Close()

	var orders []models.Order

	for rows.Next() {
		var order models.Order
		err = rows.Scan(
			&order.ID,
			&order.Status,
			&order.Accrual,
			&order.UploadedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}

		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return orders, nil
}
