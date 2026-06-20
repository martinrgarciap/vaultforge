package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

const dummyPasswordForLoginTiming = "vaultforge synthetic login timing password"

type UserStore interface {
	Create(
		ctx context.Context,
		user *store.User,
	) error

	GetByEmail(
		ctx context.Context,
		email string,
	) (*store.User, error)
}

type Account struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type RegisterInput struct {
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type Service struct {
	users  UserStore
	hasher PasswordHasher
}

func NewService(
	users UserStore,
	hasher PasswordHasher,
) *Service {
	return &Service{
		users:  users,
		hasher: hasher,
	}
}

func (service *Service) Register(
	ctx context.Context,
	input RegisterInput,
) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, err
	}

	if service == nil ||
		service.users == nil ||
		service.hasher == nil {
		return Account{},
			fmt.Errorf(
				"register account: %w",
				ErrAuthenticationUnavailable,
			)
	}

	normalizedEmail, err := NormalizeEmail(
		input.Email,
	)
	if err != nil {
		return Account{}, err
	}

	normalizedPassword, err :=
		NormalizeAndValidatePassword(
			input.Password,
		)
	if err != nil {
		return Account{}, err
	}

	passwordHash, err := service.hasher.Hash(
		ctx,
		normalizedPassword,
	)
	if err != nil {
		return Account{},
			mapPasswordHashError(err)
	}

	if passwordHash.Encoded == "" ||
		passwordHash.Algorithm == "" {
		return Account{},
			fmt.Errorf(
				"register account: %w",
				ErrAuthenticationUnavailable,
			)
	}

	user := &store.User{
		Email:             normalizedEmail,
		PasswordHash:      passwordHash.Encoded,
		PasswordAlgorithm: passwordHash.Algorithm,
	}

	err = service.users.Create(
		ctx,
		user,
	)
	if err != nil {
		return Account{},
			mapCreateAccountError(err)
	}

	return accountFromStoreUser(user), nil
}

func (service *Service) Login(
	ctx context.Context,
	input LoginInput,
) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, err
	}

	if service == nil ||
		service.users == nil ||
		service.hasher == nil {
		return Account{},
			fmt.Errorf(
				"login account: %w",
				ErrAuthenticationUnavailable,
			)
	}

	normalizedEmail, emailErr := NormalizeEmail(
		input.Email,
	)
	normalizedPassword, passwordErr :=
		NormalizeAndValidatePassword(
			input.Password,
		)

	if emailErr != nil || passwordErr != nil {
		if err := service.consumeDummyHashCost(ctx); err != nil {
			return Account{}, err
		}

		return Account{}, ErrInvalidCredentials
	}

	user, err := service.users.GetByEmail(
		ctx,
		normalizedEmail,
	)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			if err := service.consumeDummyHashCost(
				ctx,
			); err != nil {
				return Account{}, err
			}

			return Account{}, ErrInvalidCredentials

		case errors.Is(err, context.Canceled),
			errors.Is(err, context.DeadlineExceeded):
			return Account{}, err

		default:
			return Account{},
				fmt.Errorf(
					"login account: %w",
					ErrAuthenticationUnavailable,
				)
		}
	}

	if user == nil {
		return Account{},
			fmt.Errorf(
				"login account: %w",
				ErrAuthenticationUnavailable,
			)
	}

	err = service.hasher.Verify(
		ctx,
		normalizedPassword,
		PasswordHash{
			Encoded:   user.PasswordHash,
			Algorithm: user.PasswordAlgorithm,
		},
	)
	if err != nil {
		return Account{},
			mapPasswordVerifyError(err)
	}

	if user.Status != "active" {
		return Account{}, ErrInvalidCredentials
	}

	return accountFromStoreUser(user), nil
}

func (service *Service) consumeDummyHashCost(
	ctx context.Context,
) error {
	_, err := service.hasher.Hash(
		ctx,
		dummyPasswordForLoginTiming,
	)
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"login account: %w",
			ErrAuthenticationUnavailable,
		)
	}
}

func accountFromStoreUser(
	user *store.User,
) Account {
	return Account{
		ID:        user.ID,
		Email:     user.Email,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func mapPasswordHashError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	case errors.Is(err, ErrPasswordInvalidUTF8),
		errors.Is(err, ErrPasswordTooShort),
		errors.Is(err, ErrPasswordTooLong):
		return err

	default:
		return fmt.Errorf(
			"register account: %w",
			ErrAuthenticationUnavailable,
		)
	}
}

func mapPasswordVerifyError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	case errors.Is(err, ErrPasswordMismatch),
		errors.Is(err, ErrPasswordInvalidUTF8),
		errors.Is(err, ErrPasswordTooShort),
		errors.Is(err, ErrPasswordTooLong):
		return ErrInvalidCredentials

	default:
		return fmt.Errorf(
			"login account: %w",
			ErrAuthenticationUnavailable,
		)
	}
}

func mapCreateAccountError(
	err error,
) error {
	switch {
	case errors.Is(err, store.ErrDuplicateEmail):
		return ErrEmailUnavailable

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"register account: %w",
			ErrAuthenticationUnavailable,
		)
	}
}
