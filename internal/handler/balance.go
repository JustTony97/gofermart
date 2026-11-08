package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/middlewares"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
)

type TransactionService interface {
	Withdraw(ctx context.Context, userID uuid.UUID, orderID int64, amount float64) error
	GetBalance(ctx context.Context, userID uuid.UUID) (models.UserBalanceResponse, error)
	GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]models.WithdrawalResponse, error)
}

type BalanceHandler struct {
	svc TransactionService
	cfg *config.Config
}

func NewBalanceHandler(s TransactionService, c *config.Config) *BalanceHandler {
	return &BalanceHandler{svc: s, cfg: c}
}

func (b *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := middlewares.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	balanceResponse, err := b.svc.GetBalance(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		logger.Log.Error(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(balanceResponse)
}

func (b *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		http.Error(w, "invalid content type, expected application/json", http.StatusBadRequest)
		return
	}

	var req models.WithdrawalRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	userID, ok := middlewares.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	err := b.svc.Withdraw(ctx, userID, req.OrderID, req.Sum)
	if err != nil {
		if errors.Is(err, models.ErrInvalidOrderNumber) {
			http.Error(w, "incorrect order number", http.StatusUnprocessableEntity)
			return
		} else if errors.Is(err, models.ErrInsufficientFunds) {
			http.Error(w, "insufficient funds", http.StatusPaymentRequired)
			return
		} else if errors.Is(err, models.ErrUserNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		} else {
			logger.Log.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (b *BalanceHandler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := middlewares.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	withdrawals, err := b.svc.GetWithdrawals(ctx, userID)
	if err != nil {
		logger.Log.Error(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(withdrawals)
}
