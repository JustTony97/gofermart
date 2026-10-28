package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/middlewares"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
)

type OrderService interface {
	GetOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error)
}

type OrderHandler struct {
	svc OrderService
	cfg *config.Config
}

func NewOrderHandler(s OrderService, c *config.Config) *OrderHandler {
	return &OrderHandler{svc: s, cfg: c}
}

func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := middlewares.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	orders, err := h.svc.GetOrders(ctx, userID)
	if err != nil {
		logger.Log.Error(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(orders)
}
func (h *OrderHandler) UploadOrder(w http.ResponseWriter, r *http.Request) {
}
