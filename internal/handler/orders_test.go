package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/handler"
	"github.com/JustTony97/gofermart/internal/handler/mocks"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/middlewares"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	logger.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	os.Exit(m.Run())
}

func TestOrderHandler_GetOrders(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	cfg := &config.Config{}

	t.Run("401 Unauthorized when userID missing in context", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
		rec := httptest.NewRecorder()

		h.GetOrders(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("200 OK returns JSON list of orders", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		expected := []models.Order{
			{ID: 12345678903, Status: string(models.StatusProcessed)},
		}

		svc.EXPECT().
			GetOrders(mock.Anything, userID).
			Return(expected, nil).
			Once()

		req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
		req = req.WithContext(middlewares.ContextWithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		h.GetOrders(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var got []models.Order
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
		require.Equal(t, expected, got)
	})

	t.Run("204 No Content when user has no orders", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		svc.EXPECT().
			GetOrders(mock.Anything, userID).
			Return([]models.Order{}, nil).
			Once()

		req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
		req = req.WithContext(middlewares.ContextWithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		h.GetOrders(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("500 Internal Server Error on service failure", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		svc.EXPECT().
			GetOrders(mock.Anything, userID).
			Return(nil, errors.New("db error")).
			Once()

		req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
		req = req.WithContext(middlewares.ContextWithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		h.GetOrders(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestOrderHandler_UploadOrder(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	orderID := int64(12345678903)
	cfg := &config.Config{}

	t.Run("415 Unsupported Media Type when Content-Type is not text/plain", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("12345678903"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UploadOrder(rec, req)

		require.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
	})

	t.Run("400 Bad Request on non-numeric order body", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("not-a-number"))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		h.UploadOrder(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("401 Unauthorized when userID missing in context", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("12345678903"))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		h.UploadOrder(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("202 Accepted when new order is uploaded", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		svc.EXPECT().
			CreateOrder(mock.Anything, userID, orderID).
			Return(nil).
			Once()

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("12345678903"))
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(middlewares.ContextWithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		h.UploadOrder(rec, req)

		require.Equal(t, http.StatusAccepted, rec.Code)
	})

	t.Run("200 OK when order already uploaded by same user", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		svc.EXPECT().
			CreateOrder(mock.Anything, userID, orderID).
			Return(models.ErrOrderAlreadyExists).
			Once()

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("12345678903"))
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(middlewares.ContextWithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		h.UploadOrder(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("409 Conflict when order already uploaded by another user", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		svc.EXPECT().
			CreateOrder(mock.Anything, userID, orderID).
			Return(models.ErrOrderAlreadyExistsByAnotherUser).
			Once()

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("12345678903"))
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(middlewares.ContextWithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		h.UploadOrder(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("422 Unprocessable Entity on invalid order number", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		svc.EXPECT().
			CreateOrder(mock.Anything, userID, orderID).
			Return(models.ErrInvalidOrderNumber).
			Once()

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("12345678903"))
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(middlewares.ContextWithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		h.UploadOrder(rec, req)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("500 Internal Server Error on unexpected error", func(t *testing.T) {
		svc := mocks.NewMockOrderService(t)
		h := handler.NewOrderHandler(svc, cfg)

		svc.EXPECT().
			CreateOrder(mock.Anything, userID, orderID).
			Return(errors.New("db error")).
			Once()

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("12345678903"))
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(middlewares.ContextWithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		h.UploadOrder(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
