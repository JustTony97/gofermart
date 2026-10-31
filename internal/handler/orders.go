package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/middlewares"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
)

type OrderService interface {
	GetOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error)
	CreateOrder(ctx context.Context, userID uuid.UUID, orderID int) error
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
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.Encode(orders)
}

func (h *OrderHandler) UploadOrder(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		http.Error(w, "invalid content type, expected text/plain", http.StatusUnsupportedMediaType)
		return
	}

	var orderID int

	bodyBytes, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	orderID, err := strconv.Atoi(strings.TrimSpace(string(bodyBytes)))
	if err != nil {
		http.Error(w, "invalid order id format", http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	userID, ok := middlewares.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	err = h.svc.CreateOrder(ctx, userID, orderID)

	if err != nil {
		if errors.Is(err, models.ErrInvalidOrderNumber) {
			http.Error(w, "incorrect oreder number", http.StatusUnprocessableEntity)
			return
		} else if errors.Is(err, models.ErrOrderAlreadyExists) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
}
