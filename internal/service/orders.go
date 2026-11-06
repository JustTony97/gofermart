package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/service/lunh"
	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
)

type TxManager interface {
	BeginFunc(ctx context.Context, fn func(*sqlx.Tx) error) error
}
type AccrualClient interface {
	GetOrder(ctx context.Context, orderNumber string) (models.GetAccrualResponse, error)
}

type OrderRepository interface {
	GetOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error)
	CreateOrder(ctx context.Context, userID uuid.UUID, orderID int64) error
	GetOrderByStatusForUpdate(ctx context.Context, tx *sqlx.Tx, status models.OrderStatus) (int64, uuid.UUID, error)
	UpdateStatusWithinTx(ctx context.Context, tx *sqlx.Tx, orderID int64, status models.OrderStatus) error
	UpdateStatus(ctx context.Context, orderID int64, status models.OrderStatus) error
	UpdateOrderAccrualWithinTx(ctx context.Context, tx *sqlx.Tx, orderID int64, status models.OrderStatus, accrual float64) error
	UpdateUserBalanceWithinTx(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, amount float64) error
}

type OrderService struct {
	cfg       *config.Config
	repo      OrderRepository
	txManager TxManager
}

func NewOrderService(r OrderRepository, c *config.Config, tx TxManager) *OrderService {
	return &OrderService{
		cfg:       c,
		repo:      r,
		txManager: tx,
	}
}

func (s *OrderService) GetOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error) {
	return s.repo.GetOrders(ctx, userID)
}

func (s *OrderService) CreateOrder(ctx context.Context, userID uuid.UUID, orderID int64) error {
	if !lunh.Valid(orderID) {
		return models.ErrInvalidOrderNumber
	}
	return s.repo.CreateOrder(ctx, userID, orderID)
}

func (s *OrderService) ProcessNewOrder(ctx context.Context) (int64, error) {
	var capturedOrderID int64

	err := s.txManager.BeginFunc(ctx, func(tx *sqlx.Tx) error {
		orderID, _, err := s.repo.GetOrderByStatusForUpdate(ctx, tx, models.StatusNew)
		if err != nil {
			return err
		}

		capturedOrderID = orderID
		logger.Log.Infof("Start processing order=%d", orderID)

		return s.repo.UpdateStatusWithinTx(ctx, tx, orderID, models.StatusProcessing)
	})

	if err != nil {
		return 0, fmt.Errorf("process new order: transaction failed: %w", err)
	}

	return capturedOrderID, nil
}

func (s *OrderService) ProcessProcessingOrder(ctx context.Context, accrualClient AccrualClient) (int64, error) {
	var capturedOrderID int64

	err := s.txManager.BeginFunc(ctx, func(tx *sqlx.Tx) error {
		orderID, userID, err := s.repo.GetOrderByStatusForUpdate(ctx, tx, models.StatusProcessing)
		if err != nil {
			return err
		}

		capturedOrderID = orderID

		resp, err := accrualClient.GetOrder(ctx, strconv.FormatInt(orderID, 10))
		if err != nil {
			var notRegisteredErr *models.ErrOrderNotRegisteredInAccrual
			if errors.As(err, &notRegisteredErr) {
				if updateErr := s.repo.UpdateStatusWithinTx(ctx, tx, orderID, models.StatusInvalid); updateErr != nil {
					logger.Log.Error(updateErr)
					return updateErr
				}
				logger.Log.Warn(notRegisteredErr)
				return nil
			}
			return err
		}

		var accrualValue float64
		if resp.Accrual != nil {
			accrualValue = *resp.Accrual
		}

		err = s.repo.UpdateOrderAccrualWithinTx(ctx, tx, orderID, resp.Status, accrualValue)
		if err != nil {
			return err
		}

		if resp.Status == models.StatusProcessed && accrualValue > 0 {
			err = s.repo.UpdateUserBalanceWithinTx(ctx, tx, userID, accrualValue)
			if err != nil {
				return fmt.Errorf("failed to update user balance: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return capturedOrderID, nil
}
