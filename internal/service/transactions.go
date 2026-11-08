package service

import (
	"context"
	"fmt"

	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/service/lunh"
	"github.com/gofrs/uuid/v5"
	"github.com/jmoiron/sqlx"
)

type TransactionRepository interface {
	GetUserBalanceForUpdateWithinTx(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID) (float64, error)
	UpdateUserBalanceWithinTx(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, amount float64) error
	RecordTransactionWithinTx(ctx context.Context, tx *sqlx.Tx, orderID int64, userID uuid.UUID, amount float64) error

	// Новые контракты для репозитория
	GetUserBalance(ctx context.Context, userID uuid.UUID) (models.UserBalanceResponse, error)
	GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]models.WithdrawalResponse, error)
}

type TransactionService struct {
	repo      TransactionRepository
	txManager TxManager
}

func NewTransactionService(r TransactionRepository, tx TxManager) *TransactionService {
	return &TransactionService{
		repo:      r,
		txManager: tx,
	}
}

func (s *TransactionService) Withdraw(ctx context.Context, userID uuid.UUID, orderID int64, amount float64) error {
	if !lunh.Valid(orderID) {
		return models.ErrInvalidOrderNumber
	}

	err := s.txManager.BeginFunc(ctx, func(tx *sqlx.Tx) error {
		balance, err := s.repo.GetUserBalanceForUpdateWithinTx(ctx, tx, userID)
		if err != nil {
			return err
		}

		if balance < amount {
			return models.ErrInsufficientFunds
		}

		if err := s.repo.UpdateUserBalanceWithinTx(ctx, tx, userID, amount); err != nil {
			return fmt.Errorf("updating balance failed: %w", err)
		}

		if err := s.repo.RecordTransactionWithinTx(ctx, tx, orderID, userID, amount); err != nil {
			return fmt.Errorf("recording transaction failed: %w", err)
		}

		return nil
	})

	return err
}

func (s *TransactionService) GetBalance(ctx context.Context, userID uuid.UUID) (models.UserBalanceResponse, error) {
	return s.repo.GetUserBalance(ctx, userID)
}

func (s *TransactionService) GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]models.WithdrawalResponse, error) {
	return s.repo.GetWithdrawals(ctx, userID)
}
