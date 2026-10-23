package service

import (
	"context"
	"time"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user models.UserAuthRequest) (uuid.UUID, error)
	GetUserCredentialsByUsername(ctx context.Context, username string) (models.User, error)
}

type UserService struct {
	cfg  config.Config
	repo UserRepository
}

func NewUserService(r UserRepository, c config.Config) *UserService {
	return &UserService{repo: r, cfg: c}
}

func (s *UserService) CreateUser(ctx context.Context, user models.UserAuthRequest) (uuid.UUID, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, err
	}
	user.Password = string(hashedPassword)
	return s.repo.CreateUser(ctx, user)
}

func (s *UserService) LoginUser(ctx context.Context, userAuth models.UserAuthRequest) (string, error) {
	user, err := s.repo.GetUserCredentialsByUsername(ctx, userAuth.Username)
	if err != nil {
		return "", err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(userAuth.Password))
	if err != nil {
		return "", models.ErrInvalidPassword
	}

	return s.generateJWT(user.ID)
}

func (s *UserService) generateJWT(userID uuid.UUID) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, models.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.cfg.JWTExp)),
		},
		UserID: userID,
	})

	return token.SignedString([]byte(s.cfg.JWTSecret))
}
