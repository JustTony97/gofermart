package worker

import (
	"context"
	"errors"
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
}

func NewAccrualPoller(c *config.Config, s OrderAccrualService, cl service.AccrualClient) *AccrualPoller {
	return &AccrualPoller{
		svc:           s,
		cfg:           c,
		accrualClient: cl,
	}
}

func (p *AccrualPoller) StartPolling(ctx context.Context, workerID int) {
	logger.Log.Infof("Worker id=%d start polling accrual", workerID)
	ticker := time.NewTicker(p.cfg.OrderRegisterInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for {
				if ctx.Err() != nil {
					return
				}
				orderID, err := p.svc.ProcessProcessingOrder(ctx, p.accrualClient)
				if err != nil {
					if errors.Is(err, models.ErrOrderNotFound) {
						break
					}
					var rateLimitErr *models.ErrTooManyRequests
					if errors.As(err, &rateLimitErr) {
						logger.Log.Warnf("Worker id=%d rate limit hit! Sleeping for %v", workerID, rateLimitErr.RetryAfter)
						time.Sleep(rateLimitErr.RetryAfter)
						break
					}
					logger.Log.Error(err)
					break
				}
				logger.Log.Infof("Worker id=%d processed accrual order id=%d", workerID, orderID)
			}
		case <-ctx.Done():
			logger.Log.Infof("Worker id=%d stopping accrual poller...", workerID)
			return
		}
	}
}
