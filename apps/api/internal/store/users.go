package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const queryTimeout = 5 * time.Second

type User struct {
	ID                string    `json:"id"`
	Email             string    `json:"email"`
	PasswordHash      string    `json:"-"`
	PasswordAlgorithm string    `json:"-"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type UserStore struct {
	database *pgxpool.Pool
}

func NewUserStore(
	database *pgxpool.Pool,
) *UserStore {
	return &UserStore{
		database: database,
	}
}

func (store *UserStore) Create(
	ctx context.Context,
	user *User,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if store == nil ||
		store.database == nil ||
		user == nil {
		return fmt.Errorf(
			"create user: %w",
			ErrDatabase,
		)
	}

	query := `
		INSERT INTO users (
			email,
			password_hash,
			password_algorithm
		)
		VALUES ($1, $2, $3)
		RETURNING
			id::text,
			status,
			created_at,
			updated_at
	`

	queryContext, cancel := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancel()

	err := store.database.QueryRow(
		queryContext,
		query,
		user.Email,
		user.PasswordHash,
		user.PasswordAlgorithm,
	).Scan(
		&user.ID,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return mapCreateUserError(err)
	}

	return nil
}

func (store *UserStore) GetByEmail(
	ctx context.Context,
	email string,
) (*User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if store == nil ||
		store.database == nil {
		return nil, fmt.Errorf(
			"get user by email: %w",
			ErrDatabase,
		)
	}

	query := `
		SELECT
			id::text,
			email,
			password_hash,
			password_algorithm,
			status,
			created_at,
			updated_at
		FROM users
		WHERE email = $1
	`

	queryContext, cancel := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancel()

	user := &User{}

	err := store.database.QueryRow(
		queryContext,
		query,
		email,
	).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.PasswordAlgorithm,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrNotFound

		case errors.Is(err, context.Canceled),
			errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf(
				"get user by email: %w",
				err,
			)

		default:
			return nil, fmt.Errorf(
				"get user by email: %w",
				ErrDatabase,
			)
		}
	}

	return user, nil
}

func mapCreateUserError(err error) error {
	var postgresError *pgconn.PgError

	switch {
	case errors.As(err, &postgresError) &&
		postgresError.ConstraintName == "users_email_unique":
		return ErrDuplicateEmail

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"create user: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"create user: %w",
			ErrDatabase,
		)
	}
}
