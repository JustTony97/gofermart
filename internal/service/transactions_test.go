package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/service"
	"github.com/JustTony97/gofermart/internal/service/mocks"
	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTransactionService_Withdraw(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	validOrderID := int64(12345678903)
	invalidOrderID := int64(12345678901)
	withdrawAmount := 150.5

	setupTx := func(txManager *mocks.MockTxManager) {
		txManager.EXPECT().
			BeginFunc(mock.Anything, mock.AnythingOfType("func(*sqlx.Tx) error")).
			RunAndReturn(func(ctx context.Context, fn func(*sqlx.Tx) error) error {
				return fn(nil)
			}).
			Once()
	}

	t.Run("successfully withdraws funds and records transaction", func(t *testing.T) {
		repo := mocks.NewMockTransactionRepository(t)
		txManager := mocks.NewMockTxManager(t)

		setupTx(txManager)

		repo.EXPECT().
			GetUserBalanceForUpdateWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID).
			Return(500.0, nil).
			Once()

		repo.EXPECT().
			UpdateUserBalanceWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID, withdrawAmount).
			Return(nil).
			Once()

		repo.EXPECT().
			RecordTransactionWithinTx(mock.Anything, (*sqlx.Tx)(nil), validOrderID, userID, withdrawAmount).
			Return(nil).
			Once()

		svc := service.NewTransactionService(repo, txManager)
		err := svc.Withdraw(context.Background(), userID, validOrderID, withdrawAmount)

		require.NoError(t, err)
	})

	t.Run("fails on invalid luhn order number without starting tx", func(t *testing.T) {
		repo := mocks.NewMockTransactionRepository(t)
		txManager := mocks.NewMockTxManager(t)

		svc := service.NewTransactionService(repo, txManager)
		err := svc.Withdraw(context.Background(), userID, invalidOrderID, withdrawAmount)

		require.ErrorIs(t, err, models.ErrInvalidOrderNumber)
	})

	t.Run("returns ErrInsufficientFunds when balance is less than amount", func(t *testing.T) {
		repo := mocks.NewMockTransactionRepository(t)
		txManager := mocks.NewMockTxManager(t)

		setupTx(txManager)

		repo.EXPECT().
			GetUserBalanceForUpdateWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID).
			Return(100.0, nil).
			Once()

		svc := service.NewTransactionService(repo, txManager)
		err := svc.Withdraw(context.Background(), userID, validOrderID, withdrawAmount)

		require.ErrorIs(t, err, models.ErrInsufficientFunds)
	})

	t.Run("propagates error when getting balance fails", func(t *testing.T) {
		repo := mocks.NewMockTransactionRepository(t)
		txManager := mocks.NewMockTxManager(t)
		dbErr := errors.New("db error")

		setupTx(txManager)

		repo.EXPECT().
			GetUserBalanceForUpdateWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID).
			Return(0.0, dbErr).
			Once()

		svc := service.NewTransactionService(repo, txManager)
		err := svc.Withdraw(context.Background(), userID, validOrderID, withdrawAmount)

		require.ErrorIs(t, err, dbErr)
	})

	t.Run("propagates error when updating balance fails", func(t *testing.T) {
		repo := mocks.NewMockTransactionRepository(t)
		txManager := mocks.NewMockTxManager(t)
		updateErr := errors.New("update balance error")

		setupTx(txManager)

		repo.EXPECT().
			GetUserBalanceForUpdateWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID).
			Return(500.0, nil).
			Once()

		repo.EXPECT().
			UpdateUserBalanceWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID, withdrawAmount).
			Return(updateErr).
			Once()

		svc := service.NewTransactionService(repo, txManager)
		err := svc.Withdraw(context.Background(), userID, validOrderID, withdrawAmount)

		require.ErrorIs(t, err, updateErr)
	})

	t.Run("propagates error when recording transaction fails", func(t *testing.T) {
		repo := mocks.NewMockTransactionRepository(t)
		txManager := mocks.NewMockTxManager(t)
		recordErr := errors.New("record tx error")

		setupTx(txManager)

		repo.EXPECT().
			GetUserBalanceForUpdateWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID).
			Return(500.0, nil).
			Once()

		repo.EXPECT().
			UpdateUserBalanceWithinTx(mock.Anything, (*sqlx.Tx)(nil), userID, withdrawAmount).
			Return(nil).
			Once()

		repo.EXPECT().
			RecordTransactionWithinTx(mock.Anything, (*sqlx.Tx)(nil), validOrderID, userID, withdrawAmount).
			Return(recordErr).
			Once()

		svc := service.NewTransactionService(repo, txManager)
		err := svc.Withdraw(context.Background(), userID, validOrderID, withdrawAmount)

		require.ErrorIs(t, err, recordErr)
	})
}

func TestTransactionService_GetBalance(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	expected := models.UserBalanceResponse{
		Current:   350.5,
		Withdrawn: 100.0,
	}

	repo := mocks.NewMockTransactionRepository(t)
	txManager := mocks.NewMockTxManager(t)

	repo.EXPECT().
		GetUserBalance(mock.Anything, userID).
		Return(expected, nil).
		Once()

	svc := service.NewTransactionService(repo, txManager)
	got, err := svc.GetBalance(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestTransactionService_GetWithdrawals(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	expected := []models.WithdrawalResponse{
		{
			Order: 12345678903,
			Sum:   150.5,
		},
	}

	repo := mocks.NewMockTransactionRepository(t)
	txManager := mocks.NewMockTxManager(t)

	repo.EXPECT().
		GetWithdrawals(mock.Anything, userID).
		Return(expected, nil).
		Once()

	svc := service.NewTransactionService(repo, txManager)
	got, err := svc.GetWithdrawals(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, expected, got)
}
