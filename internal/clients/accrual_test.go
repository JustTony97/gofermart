package clients_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/JustTony97/gofermart/internal/clients"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	logger.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	os.Exit(m.Run())
}

func TestAccrualServiceClient_GetOrder(t *testing.T) {
	t.Run("200 OK returns parsed accrual response", func(t *testing.T) {
		accrual := 500.5
		expected := models.GetAccrualResponse{
			Order:   "12345678903",
			Status:  models.StatusProcessed,
			Accrual: &accrual,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/api/orders/12345678903", r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(expected)
		}))
		defer srv.Close()

		client := clients.NewAccrualServiceClient(srv.URL)
		got, err := client.GetOrder(context.Background(), "12345678903")

		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("200 OK returns error on invalid JSON body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{broken-json`))
		}))
		defer srv.Close()

		client := clients.NewAccrualServiceClient(srv.URL)
		_, err := client.GetOrder(context.Background(), "12345678903")

		require.Error(t, err)
	})

	t.Run("204 No Content returns ErrOrderNotRegisteredInAccrual", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := clients.NewAccrualServiceClient(srv.URL)
		_, err := client.GetOrder(context.Background(), "12345678903")

		var targetErr *models.ErrOrderNotRegisteredInAccrual
		require.ErrorAs(t, err, &targetErr)
		require.Equal(t, "12345678903", targetErr.OrderID)
	})

	t.Run("429 Too Many Requests parses valid Retry-After header", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "15")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		client := clients.NewAccrualServiceClient(srv.URL)
		_, err := client.GetOrder(context.Background(), "12345678903")

		var rateLimitErr *models.ErrTooManyRequests
		require.ErrorAs(t, err, &rateLimitErr)
		require.Equal(t, 15*time.Second, rateLimitErr.RetryAfter)
	})

	t.Run("429 Too Many Requests falls back to 60s when header is missing or invalid", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "invalid-number")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		client := clients.NewAccrualServiceClient(srv.URL)
		_, err := client.GetOrder(context.Background(), "12345678903")

		var rateLimitErr *models.ErrTooManyRequests
		require.ErrorAs(t, err, &rateLimitErr)
		require.Equal(t, 60*time.Second, rateLimitErr.RetryAfter)
	})

	t.Run("500 Internal Server Error returns ErrAccrualServerError", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal failure"))
		}))
		defer srv.Close()

		client := clients.NewAccrualServiceClient(srv.URL)
		_, err := client.GetOrder(context.Background(), "12345678903")

		var serverErr *models.ErrAccrualServerError
		require.ErrorAs(t, err, &serverErr)
		require.Equal(t, http.StatusInternalServerError, serverErr.StatusCode)
		require.Equal(t, "internal failure", serverErr.Text)
	})

	t.Run("400 Bad Request returns unexpected status code error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("bad request"))
		}))
		defer srv.Close()

		client := clients.NewAccrualServiceClient(srv.URL)
		_, err := client.GetOrder(context.Background(), "12345678903")

		require.ErrorContains(t, err, "unexpected accrual status code 400: bad request")
	})

	t.Run("returns error on invalid baseURL or network failure", func(t *testing.T) {
		client := clients.NewAccrualServiceClient("://bad-url")
		_, err := client.GetOrder(context.Background(), "12345678903")
		require.Error(t, err)
	})
}
