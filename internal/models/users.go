package models

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Password  string    `json:"-" db:"password"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	Balance   float64   `json:"balance" db:"balance"`
}

type UserAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
