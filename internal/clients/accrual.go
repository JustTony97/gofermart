package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/JustTony97/gofermart/internal/models"
)

type AccrualServiceClient struct {
	baseURL string
	client  *http.Client
}

func NewAccrualServiceClient(baseURL string) *AccrualServiceClient {
	return &AccrualServiceClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *AccrualServiceClient) GetOrder(ctx context.Context, orderNumber string) (models.GetAccrualResponse, error) {
	url := c.baseURL + "/api/orders/" + orderNumber
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return models.GetAccrualResponse{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return models.GetAccrualResponse{}, err
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var result models.GetAccrualResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return models.GetAccrualResponse{}, err
		}
		return result, nil
	case http.StatusNoContent:
		return models.GetAccrualResponse{}, &models.ErrOrderNotRegisteredInAccrual{OrderID: orderNumber}

	case http.StatusTooManyRequests:
		retryAfterStr := resp.Header.Get("Retry-After")
		seconds := 60
		if retryAfterStr != "" {
			if s, err := strconv.Atoi(retryAfterStr); err == nil {
				seconds = s
			}
		}
		return models.GetAccrualResponse{}, &models.ErrTooManyRequests{RetryAfter: time.Duration(seconds) * time.Second}

	default:
		return models.GetAccrualResponse{}, fmt.Errorf("unexpected accrual status code: %d", resp.StatusCode)
	}
}
