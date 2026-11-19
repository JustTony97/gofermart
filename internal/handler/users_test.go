package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/handler"
	"github.com/JustTony97/gofermart/internal/handler/mocks"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_Register(t *testing.T) {
	cfg := &config.Config{JWTExp: time.Hour}
	validReq := models.UserAuthRequest{
		Username: "testuser",
		Password: "secretpassword",
	}

	t.Run("200 OK sets Authorization cookie on success", func(t *testing.T) {
		svc := mocks.NewMockUserService(t)
		h := handler.NewUserHandler(svc, cfg)

		svc.EXPECT().
			CreateUser(mock.Anything, validReq).
			Return("jwt-token-123", nil).
			Once()

		body, err := json.Marshal(validReq)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		var authCookie *http.Cookie
		for _, c := range res.Cookies() {
			if c.Name == "Authorization" {
				authCookie = c
				break
			}
		}
		require.NotNil(t, authCookie)
		require.Equal(t, "jwt-token-123", authCookie.Value)
		require.True(t, authCookie.HttpOnly)
	})

	t.Run("400 Bad Request on invalid JSON", func(t *testing.T) {
		svc := mocks.NewMockUserService(t)
		h := handler.NewUserHandler(svc, cfg)

		req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(`{bad-json`))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("409 Conflict when user already exists", func(t *testing.T) {
		svc := mocks.NewMockUserService(t)
		h := handler.NewUserHandler(svc, cfg)

		svc.EXPECT().
			CreateUser(mock.Anything, validReq).
			Return("", models.ErrUserAlreadyExists).
			Once()

		body, _ := json.Marshal(validReq)
		req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("500 Internal Server Error on unexpected error", func(t *testing.T) {
		svc := mocks.NewMockUserService(t)
		h := handler.NewUserHandler(svc, cfg)

		svc.EXPECT().
			CreateUser(mock.Anything, validReq).
			Return("", errors.New("db failure")).
			Once()

		body, _ := json.Marshal(validReq)
		req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestUserHandler_Login(t *testing.T) {
	cfg := &config.Config{JWTExp: time.Hour}
	validReq := models.UserAuthRequest{
		Username: "testuser",
		Password: "secretpassword",
	}

	t.Run("200 OK sets Authorization cookie on valid login", func(t *testing.T) {
		svc := mocks.NewMockUserService(t)
		h := handler.NewUserHandler(svc, cfg)

		svc.EXPECT().
			LoginUser(mock.Anything, validReq).
			Return("jwt-login-token", nil).
			Once()

		body, _ := json.Marshal(validReq)
		req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.NotEmpty(t, res.Cookies())
		require.Equal(t, "jwt-login-token", res.Cookies()[0].Value)
	})

	t.Run("400 Bad Request on invalid JSON", func(t *testing.T) {
		svc := mocks.NewMockUserService(t)
		h := handler.NewUserHandler(svc, cfg)

		req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBufferString(`not-json`))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("401 Unauthorized on ErrUserNotFound or ErrInvalidPassword", func(t *testing.T) {
		for _, authErr := range []error{models.ErrUserNotFound, models.ErrInvalidPassword} {
			svc := mocks.NewMockUserService(t)
			h := handler.NewUserHandler(svc, cfg)

			svc.EXPECT().
				LoginUser(mock.Anything, validReq).
				Return("", authErr).
				Once()

			body, _ := json.Marshal(validReq)
			req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			h.Login(rec, req)

			require.Equal(t, http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("500 Internal Server Error on unexpected error", func(t *testing.T) {
		svc := mocks.NewMockUserService(t)
		h := handler.NewUserHandler(svc, cfg)

		svc.EXPECT().
			LoginUser(mock.Anything, validReq).
			Return("", errors.New("unexpected error")).
			Once()

		body, _ := json.Marshal(validReq)
		req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
