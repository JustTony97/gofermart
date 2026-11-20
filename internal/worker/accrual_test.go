package worker_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	serviceMocks "github.com/JustTony97/gofermart/internal/service/mocks"
	workerMocks "github.com/JustTony97/gofermart/internal/worker/mocks"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/service"
	"github.com/JustTony97/gofermart/internal/worker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	logger.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	os.Exit(m.Run())
}

func TestAccrualPoller_StartPolling(t *testing.T) {
	t.Run("processes orders in batch until ErrOrderNotFound then stops on context cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := workerMocks.NewMockOrderAccrualService(t)
		client := serviceMocks.NewMockAccrualClient(t)

		var calls atomic.Int32
		svc.EXPECT().
			ProcessProcessingOrder(mock.Anything, client).
			RunAndReturn(func(ctx context.Context, cl service.AccrualClient) (int64, error) {
				n := calls.Add(1)
				switch n {
				case 1:
					return 101, nil
				case 2:
					return 102, nil
				default:
					cancel()
					return 0, models.ErrOrderNotFound
				}
			})

		cfg := &config.Config{AccrualPollInterval: 5 * time.Millisecond}
		poller := worker.NewAccrualPoller(cfg, svc, client)

		done := make(chan struct{})
		go func() {
			poller.StartPolling(ctx, 1)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("worker did not stop on context cancellation")
		}

		require.GreaterOrEqual(t, calls.Load(), int32(3))
	})

	t.Run("handles ErrTooManyRequests and waits RetryAfter before next call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := workerMocks.NewMockOrderAccrualService(t)
		client := serviceMocks.NewMockAccrualClient(t)

		var calls atomic.Int32
		var firstCallTime, secondCallTime time.Time
		retryAfter := 40 * time.Millisecond

		svc.EXPECT().
			ProcessProcessingOrder(mock.Anything, client).
			RunAndReturn(func(ctx context.Context, cl service.AccrualClient) (int64, error) {
				n := calls.Add(1)
				if n == 1 {
					firstCallTime = time.Now()
					return 0, &models.ErrTooManyRequests{RetryAfter: retryAfter}
				}
				secondCallTime = time.Now()
				cancel()
				return 0, models.ErrOrderNotFound
			})

		cfg := &config.Config{AccrualPollInterval: 5 * time.Millisecond}
		poller := worker.NewAccrualPoller(cfg, svc, client)

		done := make(chan struct{})
		go func() {
			poller.StartPolling(ctx, 1)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("worker did not stop")
		}

		require.Equal(t, int32(2), calls.Load())
		require.GreaterOrEqual(t, secondCallTime.Sub(firstCallTime), retryAfter-5*time.Millisecond)
	})

	t.Run("stops immediately if context is canceled while waiting for retryAfter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := workerMocks.NewMockOrderAccrualService(t)
		client := serviceMocks.NewMockAccrualClient(t)

		svc.EXPECT().
			ProcessProcessingOrder(mock.Anything, client).
			RunAndReturn(func(ctx context.Context, cl service.AccrualClient) (int64, error) {
				go func() {
					time.Sleep(15 * time.Millisecond)
					cancel()
				}()
				return 0, &models.ErrTooManyRequests{RetryAfter: 5 * time.Second}
			}).
			Once()

		cfg := &config.Config{AccrualPollInterval: 5 * time.Millisecond}
		poller := worker.NewAccrualPoller(cfg, svc, client)

		start := time.Now()
		done := make(chan struct{})
		go func() {
			poller.StartPolling(ctx, 1)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("worker did not stop during retryAfter sleep")
		}

		require.Less(t, time.Since(start), 200*time.Millisecond)
	})

	t.Run("logs generic error and breaks inner loop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := workerMocks.NewMockOrderAccrualService(t)
		client := serviceMocks.NewMockAccrualClient(t)

		var calls atomic.Int32
		svc.EXPECT().
			ProcessProcessingOrder(mock.Anything, client).
			RunAndReturn(func(ctx context.Context, cl service.AccrualClient) (int64, error) {
				calls.Add(1)
				cancel()
				return 0, errors.New("unexpected db error")
			})

		cfg := &config.Config{AccrualPollInterval: 5 * time.Millisecond}
		poller := worker.NewAccrualPoller(cfg, svc, client)

		done := make(chan struct{})
		go func() {
			poller.StartPolling(ctx, 1)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("worker did not stop")
		}

		require.Equal(t, int32(1), calls.Load())
	})

	t.Run("stops immediately when context is already canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		svc := workerMocks.NewMockOrderAccrualService(t)
		client := serviceMocks.NewMockAccrualClient(t)

		cfg := &config.Config{AccrualPollInterval: time.Hour}
		poller := worker.NewAccrualPoller(cfg, svc, client)

		done := make(chan struct{})
		go func() {
			poller.StartPolling(ctx, 1)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("worker did not exit immediately")
		}
	})
}
