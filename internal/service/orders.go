package service

import (
	"context"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
)

type OrderRepository interface {
	GetOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error)
}

type OrderService struct {
	cfg  *config.Config
	repo OrderRepository
}

func NewOrderService(r OrderRepository, c *config.Config) *OrderService {
	return &OrderService{
		cfg:  c,
		repo: r,
	}
}

func (s *OrderService) GetOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error) {
	return s.repo.GetOrders(ctx, userID)
}
