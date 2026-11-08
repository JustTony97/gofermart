package middlewares

import (
	"context"
	"net/http"

	"github.com/JustTony97/gofermart/internal/config"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v4"
)

type key string

const userIDKey key = "userID"

type AuthMiddleware struct {
	cfg config.Config
}

func NewAuthMiddleware(cfg config.Config) *AuthMiddleware {
	return &AuthMiddleware{cfg: cfg}
}

func (am *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := am.getUserIDFromCookie(r)

		if uid.IsNil() {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (am *AuthMiddleware) getUserIDFromCookie(r *http.Request) uuid.UUID {
	cookie, err := r.Cookie("Authorization")
	if err != nil {
		return uuid.Nil
	}

	claims := &models.Claims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(am.cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return uuid.Nil
	}

	return claims.UserID
}

func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDKey).(uuid.UUID)
	return userID, ok
}

func ContextWithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
