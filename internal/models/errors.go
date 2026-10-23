package models

import "errors"

var (
	ErrUserAlreadyExists = errors.New("username already exists")
	ErrUserNotFound      = errors.New("username not found")
	ErrInvalidPassword   = errors.New("invalid password")
)
