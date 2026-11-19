package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/service"
	"github.com/JustTony97/gofermart/internal/service/mocks"
	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestUserService_CreateUser(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: "test-secret-key",
		JWTExp:    time.Hour,
	}
	userID := uuid.Must(uuid.NewV4())

	t.Run("creates user with hashed password and returns valid JWT", func(t *testing.T) {
		repo := mocks.NewMockUserRepository(t)
		req := models.UserAuthRequest{
			Username: "testuser",
			Password: "strongpassword123",
		}

		repo.EXPECT().
			CreateUser(mock.Anything, mock.MatchedBy(func(u models.UserAuthRequest) bool {
				return u.Username == req.Username &&
					u.Password != req.Password &&
					bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) == nil
			})).
			Return(userID, nil).
			Once()

		svc := service.NewUserService(repo, cfg)
		tokenStr, err := svc.CreateUser(context.Background(), req)

		require.NoError(t, err)
		require.NotEmpty(t, tokenStr)

		claims := &models.Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})
		require.NoError(t, err)
		require.True(t, token.Valid)
		require.Equal(t, userID, claims.UserID)
	})

	t.Run("fails when password exceeds 72 bytes", func(t *testing.T) {
		repo := mocks.NewMockUserRepository(t)
		req := models.UserAuthRequest{
			Username: "testuser",
			Password: strings.Repeat("a", 73),
		}

		svc := service.NewUserService(repo, cfg)
		tokenStr, err := svc.CreateUser(context.Background(), req)

		require.ErrorIs(t, err, bcrypt.ErrPasswordTooLong)
		require.Empty(t, tokenStr)
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := mocks.NewMockUserRepository(t)
		repoErr := errors.New("user already exists")
		req := models.UserAuthRequest{
			Username: "testuser",
			Password: "strongpassword123",
		}

		repo.EXPECT().
			CreateUser(mock.Anything, mock.Anything).
			Return(uuid.Nil, repoErr).
			Once()

		svc := service.NewUserService(repo, cfg)
		tokenStr, err := svc.CreateUser(context.Background(), req)

		require.ErrorIs(t, err, repoErr)
		require.Empty(t, tokenStr)
	})
}

func TestUserService_LoginUser(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: "test-secret-key",
		JWTExp:    time.Hour,
	}
	userID := uuid.Must(uuid.NewV4())
	plainPassword := "strongpassword123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.MinCost)
	require.NoError(t, err)

	t.Run("returns valid JWT on correct credentials", func(t *testing.T) {
		repo := mocks.NewMockUserRepository(t)
		req := models.UserAuthRequest{
			Username: "testuser",
			Password: plainPassword,
		}

		repo.EXPECT().
			GetUserCredentialsByUsername(mock.Anything, req.Username).
			Return(models.User{
				ID:       userID,
				Username: req.Username,
				Password: string(hashedPassword),
			}, nil).
			Once()

		svc := service.NewUserService(repo, cfg)
		tokenStr, err := svc.LoginUser(context.Background(), req)

		require.NoError(t, err)
		require.NotEmpty(t, tokenStr)

		claims := &models.Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})
		require.NoError(t, err)
		require.True(t, token.Valid)
		require.Equal(t, userID, claims.UserID)
	})

	t.Run("returns ErrInvalidPassword when password does not match", func(t *testing.T) {
		repo := mocks.NewMockUserRepository(t)
		req := models.UserAuthRequest{
			Username: "testuser",
			Password: "wrongpassword",
		}

		repo.EXPECT().
			GetUserCredentialsByUsername(mock.Anything, req.Username).
			Return(models.User{
				ID:       userID,
				Username: req.Username,
				Password: string(hashedPassword),
			}, nil).
			Once()

		svc := service.NewUserService(repo, cfg)
		tokenStr, err := svc.LoginUser(context.Background(), req)

		require.ErrorIs(t, err, models.ErrInvalidPassword)
		require.Empty(t, tokenStr)
	})

	t.Run("propagates repository error when user not found", func(t *testing.T) {
		repo := mocks.NewMockUserRepository(t)
		repoErr := errors.New("user not found")
		req := models.UserAuthRequest{
			Username: "unknown",
			Password: plainPassword,
		}

		repo.EXPECT().
			GetUserCredentialsByUsername(mock.Anything, req.Username).
			Return(models.User{}, repoErr).
			Once()

		svc := service.NewUserService(repo, cfg)
		tokenStr, err := svc.LoginUser(context.Background(), req)

		require.ErrorIs(t, err, repoErr)
		require.Empty(t, tokenStr)
	})
}
