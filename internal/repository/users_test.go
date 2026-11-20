package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/JustTony97/gofermart/internal/models"
	"github.com/JustTony97/gofermart/internal/repository"
	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_CreateUser(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	req := models.UserAuthRequest{
		Username: "testuser",
		Password: "hashedpassword",
	}

	t.Run("successfully creates user and returns id", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewUsersRepository(db)

		mock.ExpectQuery(`INSERT INTO users \(username, password\) VALUES \(\$1, \$2\) RETURNING id`).
			WithArgs(req.Username, req.Password).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))

		gotID, err := repo.CreateUser(context.Background(), req)

		require.NoError(t, err)
		require.Equal(t, userID, gotID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrUserAlreadyExists on unique violation 23505", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewUsersRepository(db)

		mock.ExpectQuery(`INSERT INTO users`).
			WithArgs(req.Username, req.Password).
			WillReturnError(&pgconn.PgError{Code: "23505"})

		gotID, err := repo.CreateUser(context.Background(), req)

		require.ErrorIs(t, err, models.ErrUserAlreadyExists)
		require.Equal(t, uuid.Nil, gotID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns generic error on db failure", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewUsersRepository(db)
		dbErr := errors.New("connection failed")

		mock.ExpectQuery(`INSERT INTO users`).
			WithArgs(req.Username, req.Password).
			WillReturnError(dbErr)

		gotID, err := repo.CreateUser(context.Background(), req)

		require.ErrorIs(t, err, dbErr)
		require.Equal(t, uuid.Nil, gotID)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUserRepository_GetUserCredentialsByUsername(t *testing.T) {
	userID := uuid.Must(uuid.NewV4())
	username := "testuser"
	hashedPass := "hashedpassword"

	t.Run("returns user credentials on success", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewUsersRepository(db)

		rows := sqlmock.NewRows([]string{"id", "username", "password"}).
			AddRow(userID, username, hashedPass)

		mock.ExpectQuery(`SELECT id, username, password FROM users WHERE username = \$1`).
			WithArgs(username).
			WillReturnRows(rows)

		got, err := repo.GetUserCredentialsByUsername(context.Background(), username)

		require.NoError(t, err)
		require.Equal(t, userID, got.ID)
		require.Equal(t, username, got.Username)
		require.Equal(t, hashedPass, got.Password)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrUserNotFound on sql.ErrNoRows", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewUsersRepository(db)

		mock.ExpectQuery(`SELECT id, username, password FROM users WHERE username = \$1`).
			WithArgs(username).
			WillReturnError(sql.ErrNoRows)

		_, err := repo.GetUserCredentialsByUsername(context.Background(), username)

		require.ErrorIs(t, err, models.ErrUserNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns generic error on db failure", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := repository.NewUsersRepository(db)
		dbErr := errors.New("select error")

		mock.ExpectQuery(`SELECT id, username, password FROM users WHERE username = \$1`).
			WithArgs(username).
			WillReturnError(dbErr)

		_, err := repo.GetUserCredentialsByUsername(context.Background(), username)

		require.ErrorIs(t, err, dbErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
