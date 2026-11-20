package models

import (
	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID
}
