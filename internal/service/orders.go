package service

import (
	"context"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/service/lunh"
	"github.com/gofrs/uuid/v5"
)

type OrderRepository interface {
	GetOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error)
	CreateOrder(ctx context.Context, userID uuid.UUID, orderID int) error
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

func (s *OrderService) CreateOrder(ctx context.Context, userID uuid.UUID, orderID int) error {
	if lunh.Valid(orderID) == false {
		return models.ErrInvalidOrderNumber
	}
	return s.repo.CreateOrder(ctx, userID, orderID)
}
