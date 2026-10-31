package models

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type Order struct {
	ID         int       `json:"number" db:"id"`
	Status     string    `json:"status" db:"status"`
	UserID     uuid.UUID `json:"-" db:"user_id"`
	Accrual    *int      `json:"accrual,omitempty" db:"reward_points"`
	UploadedAt time.Time `json:"uploaded_at" db:"created_at"`
	ModifiedAt time.Time `json:"-" db:"modified_at"`
}

type OrderStatus string

const (
	StatusNew        OrderStatus = "NEW"
	StatusProcessing OrderStatus = "PROCESSING"
	StatusInvalid    OrderStatus = "INVALID"
	StatusProcessed  OrderStatus = "PROCESSED"
)
