package worker

import (
	"context"
	"errors"
	"time"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/logger"
	"github.com/JustTony97/gofermart/internal/models"
)

type OrderService interface {
	ProcessNewOrder(ctx context.Context) (int64, error)
}

type OrdersRegisterPoller struct {
	svc OrderService
	cfg *config.Config
}

func NewOrderPoller(c *config.Config, s OrderService) *OrdersRegisterPoller {
	return &OrdersRegisterPoller{
		svc: s,
		cfg: c,
	}
}

func (p *OrdersRegisterPoller) StartPolling(ctx context.Context, workerID int) {
	logger.Log.Infof("Worker id=%d start polling", workerID)
	ticker := time.NewTicker(p.cfg.OrderRegisterInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for {
				if ctx.Err() != nil {
					return
				}
				orderID, err := p.svc.ProcessNewOrder(ctx)
				if err != nil {
					if errors.Is(err, models.ErrOrderNotFound) {
						break
					}
					logger.Log.Error(err)
					break
				}
				logger.Log.Infof("Worker id=%d changed order id=%d status to processing", workerID, orderID)
			}
		case <-ctx.Done():
			logger.Log.Infof("Worker id=%d stopping...", workerID)
			return
		}
	}
}
