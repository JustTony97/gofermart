package worker_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	workerMocks "github.com/JustTony97/gofermart/internal/worker/mocks"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/worker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrdersRegisterPoller_StartPolling(t *testing.T) {
	t.Run("processes new orders until ErrOrderNotFound then stops on context cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := workerMocks.NewMockOrderService(t)

		var calls atomic.Int32
		svc.EXPECT().
			ProcessNewOrder(mock.Anything).
			RunAndReturn(func(ctx context.Context) (int64, error) {
				n := calls.Add(1)
				switch n {
				case 1:
					return 201, nil
				case 2:
					return 202, nil
				default:
					cancel()
					return 0, models.ErrOrderNotFound
				}
			})

		cfg := &config.Config{OrderRegisterInterval: 5 * time.Millisecond}
		poller := worker.NewOrderPoller(cfg, svc)

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

	t.Run("logs generic error and breaks inner loop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := workerMocks.NewMockOrderService(t)

		var calls atomic.Int32
		svc.EXPECT().
			ProcessNewOrder(mock.Anything).
			RunAndReturn(func(ctx context.Context) (int64, error) {
				calls.Add(1)
				cancel()
				return 0, errors.New("unexpected db error")
			})

		cfg := &config.Config{OrderRegisterInterval: 5 * time.Millisecond}
		poller := worker.NewOrderPoller(cfg, svc)

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

		svc := workerMocks.NewMockOrderService(t)

		cfg := &config.Config{OrderRegisterInterval: time.Hour}
		poller := worker.NewOrderPoller(cfg, svc)

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
