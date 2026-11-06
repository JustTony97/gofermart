package models

import "time"

type WithdrawalRequest struct {
	OrderID int64   `json:"order,string"`
	Sum     float64 `json:"sum"`
}

type WithdrawalResponse struct {
	Order       int64     `json:"order,string" db:"order"`
	Sum         float64   `json:"sum" db:"sum"`
	ProcessedAt time.Time `json:"processed_at" db:"processed_at"`
}
