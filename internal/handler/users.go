package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/models"
)

type UserService interface {
	CreateUser(ctx context.Context, user models.UserAuthRequest) (string, error)
	LoginUser(ctx context.Context, user models.UserAuthRequest) (string, error)
}

type UserHandler struct {
	svc UserService
	cfg *config.Config
}

func NewUserHandler(s UserService, c *config.Config) *UserHandler {
	return &UserHandler{svc: s, cfg: c}
}

func (u *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var userAuth models.UserAuthRequest
	err := json.NewDecoder(r.Body).Decode(&userAuth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	token, err := u.svc.CreateUser(ctx, userAuth)
	if err != nil {
		if errors.Is(err, models.ErrUserAlreadyExists) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	u.setAuthCookie(w, token)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (u *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var userAuth models.UserAuthRequest
	err := json.NewDecoder(r.Body).Decode(&userAuth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token, err := u.svc.LoginUser(r.Context(), userAuth)

	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) || errors.Is(err, models.ErrInvalidPassword) {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u.setAuthCookie(w, token)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (u *UserHandler) setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "Authorization",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(u.cfg.JWTExp),
	})
}
