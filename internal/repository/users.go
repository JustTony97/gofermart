package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/JustTony97/gofermart/internal/models"
	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUsersRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateUser(ctx context.Context, user models.UserAuthRequest) (uuid.UUID, error) {
	var userID uuid.UUID

	query := `
		INSERT INTO users (username, password) 
		VALUES ($1, $2) 
		RETURNING id
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		user.Username,
		user.Password,
	).Scan(&userID)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, models.ErrUserAlreadyExists
		}
		return uuid.Nil, err
	}

	return userID, nil
}

func (r *UserRepository) GetUserCredentialsByUsername(ctx context.Context, username string) (models.User, error) {
	var user models.User
	query := `
		SELECT id, username, password
		FROM users
		WHERE username = $1
	`
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, models.ErrUserNotFound
		}
		return models.User{}, err
	}
	return user, nil
}
