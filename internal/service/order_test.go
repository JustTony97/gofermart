package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/service"
	"github.com/JustTony97/gofermart/internal/service/mocks"
	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	logger.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	os.Exit(m.Run())
}

func TestOrderService_CreateOrder(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	validOrderID := int64(12345678903)
	invalidOrderID := int64(12345678901)

	t.Run("success with valid luhn number", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)

		repo.EXPECT().
			CreateOrder(mock.Anything, userID, validOrderID).
			Return(nil).
			Once()

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		err := svc.CreateOrder(context.Background(), userID, validOrderID)

		require.NoError(t, err)
	})

	t.Run("fails on invalid luhn number without calling repo", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		err := svc.CreateOrder(context.Background(), userID, invalidOrderID)

		require.ErrorIs(t, err, models.ErrInvalidOrderNumber)
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)
		dbErr := errors.New("connection refused")

		repo.EXPECT().
			CreateOrder(mock.Anything, userID, validOrderID).
			Return(dbErr).
			Once()

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		err := svc.CreateOrder(context.Background(), userID, validOrderID)

		require.ErrorIs(t, err, dbErr)
	})
}

func TestOrderService_GetOrders(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	expected := []models.Order{
		{
			ID:     12345678903,
			Status: string(models.StatusProcessed),
			UserID: userID,
		},
	}

	repo := mocks.NewMockOrderRepository(t)
	txManager := mocks.NewMockTxManager(t)

	repo.EXPECT().
		GetOrders(mock.Anything, userID).
		Return(expected, nil).
		Once()

	svc := service.NewOrderService(repo, &config.Config{}, txManager)
	got, err := svc.GetOrders(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestOrderService_ProcessNewOrder(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	orderID := int64(12345678903)

	t.Run("moves order from NEW to PROCESSING", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)

		txManager.EXPECT().
			BeginFunc(mock.Anything, mock.AnythingOfType("func(*sqlx.Tx) error")).
			RunAndReturn(func(ctx context.Context, fn func(*sqlx.Tx) error) error {
				return fn(nil)
			}).
			Once()

		repo.EXPECT().
			GetOrderByStatusForUpdate(mock.Anything, (*sqlx.Tx)(nil), models.StatusNew).
			Return(orderID, userID, nil).
			Once()

		repo.EXPECT().
			UpdateStatusWithinTx(mock.Anything, (*sqlx.Tx)(nil), orderID, models.StatusProcessing).
			Return(nil).
			Once()

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		gotID, err := svc.ProcessNewOrder(context.Background())

		require.NoError(t, err)
		require.Equal(t, orderID, gotID)
	})

	t.Run("returns error when no NEW orders found", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)
		noRowsErr := errors.New("sql: no rows in result set")

		txManager.EXPECT().
			BeginFunc(mock.Anything, mock.AnythingOfType("func(*sqlx.Tx) error")).
			RunAndReturn(func(ctx context.Context, fn func(*sqlx.Tx) error) error {
				return fn(nil)
			}).
			Once()

		repo.EXPECT().
			GetOrderByStatusForUpdate(mock.Anything, (*sqlx.Tx)(nil), models.StatusNew).
			Return(int64(0), uuid.Nil, noRowsErr).
			Once()

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		gotID, err := svc.ProcessNewOrder(context.Background())

		require.ErrorIs(t, err, noRowsErr)
		require.Zero(t, gotID)
	})
}

func TestOrderService_ProcessProcessingOrder(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	orderID := int64(12345678903)
	accrualVal := 450.5

	setupTx := func(txManager *mocks.MockTxManager) {
		txManager.EXPECT().
			BeginFunc(mock.Anything, mock.AnythingOfType("func(*sqlx.Tx) error")).
			RunAndReturn(func(ctx context.Context, fn func(*sqlx.Tx) error) error {
				return fn(nil)
			}).
			Once()
	}

	t.Run("PROCESSED order updates accrual and credits user balance", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)
		client := mocks.NewMockAccrualClient(t)

		setupTx(txManager)

		repo.EXPECT().
			GetOrderByStatusForUpdate(mock.Anything, (*sqlx.Tx)(nil), models.StatusProcessing).
			Return(orderID, userID, nil).
			Once()

		client.EXPECT().
			GetOrder(mock.Anything, "12345678903").
			Return(models.GetAccrualResponse{
				Order:   "12345678903",
				Status:  models.StatusProcessed,
				Accrual: &accrualVal,
			}, nil).
			Once()

		repo.EXPECT().
			UpdateOrderAccrualWithinTx(mock.Anything, (*sqlx.Tx)(nil), orderID, models.StatusProcessed, accrualVal).
			Return(nil).
			Once()

		repo.EXPECT().
			UpdateUserBalanceWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID, accrualVal).
			Return(nil).
			Once()

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		gotID, err := svc.ProcessProcessingOrder(context.Background(), client)

		require.NoError(t, err)
		require.Equal(t, orderID, gotID)
	})

	t.Run("balance update failure rolls back transaction", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)
		client := mocks.NewMockAccrualClient(t)
		balanceErr := errors.New("balance update failed")

		setupTx(txManager)

		repo.EXPECT().
			GetOrderByStatusForUpdate(mock.Anything, (*sqlx.Tx)(nil), models.StatusProcessing).
			Return(orderID, userID, nil).
			Once()

		client.EXPECT().
			GetOrder(mock.Anything, "12345678903").
			Return(models.GetAccrualResponse{
				Order:   "12345678903",
				Status:  models.StatusProcessed,
				Accrual: &accrualVal,
			}, nil).
			Once()

		repo.EXPECT().
			UpdateOrderAccrualWithinTx(mock.Anything, (*sqlx.Tx)(nil), orderID, models.StatusProcessed, accrualVal).
			Return(nil).
			Once()

		repo.EXPECT().
			UpdateUserBalanceWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID, accrualVal).
			Return(balanceErr).
			Once()

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		_, err := svc.ProcessProcessingOrder(context.Background(), client)

		require.ErrorIs(t, err, balanceErr)
	})

	t.Run("unregistered order sets status to INVALID", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)
		client := mocks.NewMockAccrualClient(t)

		setupTx(txManager)

		repo.EXPECT().
			GetOrderByStatusForUpdate(mock.Anything, (*sqlx.Tx)(nil), models.StatusProcessing).
			Return(orderID, userID, nil).
			Once()

		client.EXPECT().
			GetOrder(mock.Anything, "12345678903").
			Return(models.GetAccrualResponse{}, &models.ErrOrderNotRegisteredInAccrual{}).
			Once()

		repo.EXPECT().
			UpdateStatusWithinTx(mock.Anything, (*sqlx.Tx)(nil), orderID, models.StatusInvalid).
			Return(nil).
			Once()

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		gotID, err := svc.ProcessProcessingOrder(context.Background(), client)

		require.NoError(t, err)
		require.Equal(t, orderID, gotID)
	})

	t.Run("retries 3 times on ErrAccrualServerError", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)
		client := mocks.NewMockAccrualClient(t)

		setupTx(txManager)

		repo.EXPECT().
			GetOrderByStatusForUpdate(mock.Anything, (*sqlx.Tx)(nil), models.StatusProcessing).
			Return(orderID, userID, nil).
			Once()

		client.EXPECT().
			GetOrder(mock.Anything, "12345678903").
			Return(models.GetAccrualResponse{}, &models.ErrAccrualServerError{}).
			Times(2)

		client.EXPECT().
			GetOrder(mock.Anything, "12345678903").
			Return(models.GetAccrualResponse{
				Order:  "12345678903",
				Status: models.StatusProcessing,
			}, nil).
			Once()

		repo.EXPECT().
			UpdateOrderAccrualWithinTx(mock.Anything, (*sqlx.Tx)(nil), orderID, models.StatusProcessing, 0.0).
			Return(nil).
			Once()

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		gotID, err := svc.ProcessProcessingOrder(context.Background(), client)

		require.NoError(t, err)
		require.Equal(t, orderID, gotID)
	})

	t.Run("aborts retry immediately on context cancellation", func(t *testing.T) {
		repo := mocks.NewMockOrderRepository(t)
		txManager := mocks.NewMockTxManager(t)
		client := mocks.NewMockAccrualClient(t)

		ctx, cancel := context.WithCancel(context.Background())
		setupTx(txManager)

		repo.EXPECT().
			GetOrderByStatusForUpdate(mock.Anything, (*sqlx.Tx)(nil), models.StatusProcessing).
			Return(orderID, userID, nil).
			Once()

		client.EXPECT().
			GetOrder(mock.Anything, "12345678903").
			RunAndReturn(func(ctx context.Context, orderNumber string) (models.GetAccrualResponse, error) {
				cancel()
				return models.GetAccrualResponse{}, &models.ErrAccrualServerError{}
			}).
			Once()

		svc := service.NewOrderService(repo, &config.Config{}, txManager)
		start := time.Now()
		_, err := svc.ProcessProcessingOrder(ctx, client)

		require.ErrorIs(t, err, context.Canceled)
		require.Less(t, time.Since(start), 200*time.Millisecond)
	})
}
