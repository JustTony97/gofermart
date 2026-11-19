package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/service"
)

type OrderAccrualService interface {
	ProcessProcessingOrder(ctx context.Context, accrualClient service.AccrualClient) (int64, error)
}

type AccrualPoller struct {
	svc           OrderAccrualService
	cfg           *config.Config
	accrualClient service.AccrualClient
	retryAfter    atomic.Int64
}

func NewAccrualPoller(c *config.Config, s OrderAccrualService, cl service.AccrualClient) *AccrualPoller {
	return &AccrualPoller{
		svc:           s,
		cfg:           c,
		accrualClient: cl,
	}
}

func (p *AccrualPoller) StartPolling(ctx context.Context, workerID int) {
	logger.Log.Info("Worker start polling accrual", "worker_id", workerID)
	ticker := time.NewTicker(p.cfg.AccrualPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for {
				if ctx.Err() != nil {
					return
				}

				if wait := time.Until(time.Unix(0, p.retryAfter.Load())); wait > 0 {
					timer := time.NewTimer(wait)
					select {
					case <-timer.C:
					case <-ctx.Done():
						timer.Stop()
						logger.Log.Info("Worker stopping accrual poller...", "worker_id", workerID)
						return
					}
				}

				orderID, err := p.svc.ProcessProcessingOrder(ctx, p.accrualClient)
				if err != nil {
					if errors.Is(err, models.ErrOrderNotFound) {
						break
					}
					var rateLimitErr *models.ErrTooManyRequests
					if errors.As(err, &rateLimitErr) {
						logger.Log.Warn("Rate limit hit, sleeping",
							"worker_id", workerID,
							"retry_after", rateLimitErr.RetryAfter,
						)
						until := time.Now().Add(rateLimitErr.RetryAfter).UnixNano()
						p.retryAfter.Store(until)
						break
					}
					logger.Log.Error(err.Error())
					break
				}
				logger.Log.Info("Processed accrual order",
					"worker_id", workerID,
					"order_id", orderID,
				)

			}
		case <-ctx.Done():
			logger.Log.Info("Worker stopping accrual poller...", "worker_id", workerID)
			return
		}
	}
}
