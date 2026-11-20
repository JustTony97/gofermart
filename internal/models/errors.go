package models

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrUserAlreadyExists               = errors.New("username already exists")
	ErrUserNotFound                    = errors.New("username not found")
	ErrInvalidPassword                 = errors.New("invalid password")
	ErrOrderAlreadyExists              = errors.New("orders already exists")
	ErrInvalidOrderNumber              = errors.New("invalid order number")
	ErrOrderNotFound                   = errors.New("order not found")
	ErrInsufficientFunds               = errors.New("insufficient funds on the account")
	ErrOrderAlreadyExistsByAnotherUser = errors.New("order already exists by another user")
)

type ErrTooManyRequests struct {
	RetryAfter time.Duration
}

type ErrOrderNotRegisteredInAccrual struct {
	OrderID string
}

func (e *ErrTooManyRequests) Error() string {
	return fmt.Sprintf("accrual service rate limit exceeded, retry after %s seconds", e.RetryAfter.String())
}

func (e *ErrOrderNotRegisteredInAccrual) Error() string {
	return fmt.Sprintf("order id=%s not registered in accrual", e.OrderID)
}

type ErrAccrualServerError struct {
	StatusCode int
	Text       string
}

func (e *ErrAccrualServerError) Error() string {
	return fmt.Sprintf("accrual server error (status %d): %s", e.StatusCode, e.Text)
}
