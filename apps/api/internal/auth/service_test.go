package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

const registrationPassword = "correct horse battery staple"

func TestServiceRegister(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		19,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Minute)

	userStore := &fakeUserStore{
		createdID:        "user-123",
		createdStatus:    "active",
		createdCreatedAt: createdAt,
		createdUpdatedAt: updatedAt,
	}
	passwordHasher := &fakePasswordHasher{
		hashResult: PasswordHash{
			Encoded:   "encoded-password-hash",
			Algorithm: AlgorithmArgon2id,
		},
	}

	service := NewService(
		userStore,
		passwordHasher,
	)

	decomposedPassword := strings.Repeat(
		"e\u0301",
		MinPasswordLength,
	)
	wantNormalizedPassword := strings.Repeat(
		"é",
		MinPasswordLength,
	)

	account, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:    "  Martin.Test+Vault@Example.COM  ",
			Password: decomposedPassword,
		},
	)
	if err != nil {
		t.Fatalf(
			"Register() returned unexpected error: %v",
			err,
		)
	}

	if passwordHasher.hashCalls != 1 {
		t.Fatalf(
			"Hash() calls = %d, want 1",
			passwordHasher.hashCalls,
		)
	}

	if passwordHasher.lastHashedPassword !=
		wantNormalizedPassword {
		t.Fatalf(
			"Hash() password was not normalized",
		)
	}

	if userStore.createCalls != 1 {
		t.Fatalf(
			"Create() calls = %d, want 1",
			userStore.createCalls,
		)
	}

	if userStore.createdUser == nil {
		t.Fatal(
			"Create() did not receive a user",
		)
	}

	if userStore.createdUser.Email !=
		"martin.test+vault@example.com" {
		t.Fatalf(
			"stored email = %q",
			userStore.createdUser.Email,
		)
	}

	if userStore.createdUser.PasswordHash !=
		"encoded-password-hash" {
		t.Fatal(
			"stored password hash does not match hasher result",
		)
	}

	if userStore.createdUser.PasswordAlgorithm !=
		AlgorithmArgon2id {
		t.Fatal(
			"stored password algorithm does not match hasher result",
		)
	}

	if account.ID != "user-123" ||
		account.Email !=
			"martin.test+vault@example.com" ||
		account.Status != "active" ||
		!account.CreatedAt.Equal(createdAt) ||
		!account.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"Register() account = %+v",
			account,
		)
	}
}

func TestServiceRegisterRejectsInvalidEmail(
	t *testing.T,
) {
	t.Parallel()

	userStore := &fakeUserStore{}
	passwordHasher := &fakePasswordHasher{}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:    "not-an-email",
			Password: registrationPassword,
		},
	)

	if !errors.Is(err, ErrEmailInvalid) {
		t.Fatalf(
			"Register() error = %v, want %v",
			err,
			ErrEmailInvalid,
		)
	}

	if passwordHasher.hashCalls != 0 {
		t.Fatal(
			"Hash() was called for an invalid email",
		)
	}

	if userStore.createCalls != 0 {
		t.Fatal(
			"Create() was called for an invalid email",
		)
	}
}

func TestServiceRegisterRejectsInvalidPassword(
	t *testing.T,
) {
	t.Parallel()

	userStore := &fakeUserStore{}
	passwordHasher := &fakePasswordHasher{}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:    "martin@example.com",
			Password: "too short",
		},
	)

	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf(
			"Register() error = %v, want %v",
			err,
			ErrPasswordTooShort,
		)
	}

	if passwordHasher.hashCalls != 0 {
		t.Fatal(
			"Hash() was called for an invalid password",
		)
	}

	if userStore.createCalls != 0 {
		t.Fatal(
			"Create() was called for an invalid password",
		)
	}
}

func TestServiceRegisterMapsDuplicateEmail(
	t *testing.T,
) {
	t.Parallel()

	userStore := &fakeUserStore{
		createErr: store.ErrDuplicateEmail,
	}
	passwordHasher := &fakePasswordHasher{
		hashResult: PasswordHash{
			Encoded:   "encoded-password-hash",
			Algorithm: AlgorithmArgon2id,
		},
	}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:    "martin@example.com",
			Password: registrationPassword,
		},
	)

	if !errors.Is(err, ErrEmailUnavailable) {
		t.Fatalf(
			"Register() error = %v, want %v",
			err,
			ErrEmailUnavailable,
		)
	}
}

func TestServiceRegisterMapsHasherFailureSafely(
	t *testing.T,
) {
	t.Parallel()

	userStore := &fakeUserStore{}
	passwordHasher := &fakePasswordHasher{
		hashErr: errors.New(
			"synthetic sensitive hasher detail",
		),
	}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:    "martin@example.com",
			Password: registrationPassword,
		},
	)

	if !errors.Is(
		err,
		ErrAuthenticationUnavailable,
	) {
		t.Fatalf(
			"Register() error = %v, want %v",
			err,
			ErrAuthenticationUnavailable,
		)
	}

	if strings.Contains(
		err.Error(),
		"synthetic sensitive hasher detail",
	) {
		t.Fatal(
			"Register() exposed the underlying hasher error",
		)
	}

	if userStore.createCalls != 0 {
		t.Fatal(
			"Create() was called after hashing failed",
		)
	}
}

func TestServiceRegisterMapsStoreFailureSafely(
	t *testing.T,
) {
	t.Parallel()

	userStore := &fakeUserStore{
		createErr: store.ErrDatabase,
	}
	passwordHasher := &fakePasswordHasher{
		hashResult: PasswordHash{
			Encoded:   "encoded-password-hash",
			Algorithm: AlgorithmArgon2id,
		},
	}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:    "martin@example.com",
			Password: registrationPassword,
		},
	)

	if !errors.Is(
		err,
		ErrAuthenticationUnavailable,
	) {
		t.Fatalf(
			"Register() error = %v, want %v",
			err,
			ErrAuthenticationUnavailable,
		)
	}
}

type fakeUserStore struct {
	createErr error

	createCalls int
	createdUser *store.User

	createdID        string
	createdStatus    string
	createdCreatedAt time.Time
	createdUpdatedAt time.Time
}

func (userStore *fakeUserStore) Create(
	_ context.Context,
	user *store.User,
) error {
	userStore.createCalls++

	if userStore.createErr != nil {
		return userStore.createErr
	}

	user.ID = userStore.createdID
	user.Status = userStore.createdStatus
	user.CreatedAt = userStore.createdCreatedAt
	user.UpdatedAt = userStore.createdUpdatedAt

	copiedUser := *user
	userStore.createdUser = &copiedUser

	return nil
}

func (userStore *fakeUserStore) GetByEmail(
	_ context.Context,
	_ string,
) (*store.User, error) {
	return nil, store.ErrNotFound
}

type fakePasswordHasher struct {
	hashResult PasswordHash
	hashErr    error

	hashCalls          int
	lastHashedPassword string
}

func (hasher *fakePasswordHasher) Hash(
	_ context.Context,
	password string,
) (PasswordHash, error) {
	hasher.hashCalls++
	hasher.lastHashedPassword = password

	if hasher.hashErr != nil {
		return PasswordHash{}, hasher.hashErr
	}

	return hasher.hashResult, nil
}

func (hasher *fakePasswordHasher) Verify(
	_ context.Context,
	_ string,
	_ PasswordHash,
) error {
	return nil
}
