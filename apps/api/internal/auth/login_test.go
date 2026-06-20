package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

const loginPassword = "correct horse battery staple"

func TestServiceLogin(t *testing.T) {
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

	userStore := &loginFakeUserStore{
		user: &store.User{
			ID:                "user-123",
			Email:             "martin@example.com",
			PasswordHash:      "encoded-password-hash",
			PasswordAlgorithm: AlgorithmArgon2id,
			Status:            "active",
			CreatedAt:         createdAt,
			UpdatedAt:         updatedAt,
		},
	}
	passwordHasher := &loginFakePasswordHasher{}

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

	account, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "  Martin@Example.COM  ",
			Password: decomposedPassword,
		},
	)
	if err != nil {
		t.Fatalf(
			"Login() returned unexpected error: %v",
			err,
		)
	}

	if userStore.getCalls != 1 {
		t.Fatalf(
			"GetByEmail() calls = %d, want 1",
			userStore.getCalls,
		)
	}

	if userStore.lastEmail != "martin@example.com" {
		t.Fatalf(
			"GetByEmail() email = %q, want %q",
			userStore.lastEmail,
			"martin@example.com",
		)
	}

	if passwordHasher.verifyCalls != 1 {
		t.Fatalf(
			"Verify() calls = %d, want 1",
			passwordHasher.verifyCalls,
		)
	}

	if passwordHasher.lastVerifiedPassword !=
		wantNormalizedPassword {
		t.Fatal(
			"Verify() password was not normalized",
		)
	}

	if passwordHasher.lastVerifiedHash !=
		(PasswordHash{
			Encoded:   "encoded-password-hash",
			Algorithm: AlgorithmArgon2id,
		}) {
		t.Fatalf(
			"Verify() password hash = %+v",
			passwordHasher.lastVerifiedHash,
		)
	}

	if passwordHasher.hashCalls != 0 {
		t.Fatalf(
			"Hash() calls = %d, want 0",
			passwordHasher.hashCalls,
		)
	}

	if account.ID != "user-123" ||
		account.Email != "martin@example.com" ||
		account.Status != "active" ||
		!account.CreatedAt.Equal(createdAt) ||
		!account.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"Login() account = %+v",
			account,
		)
	}
}

func TestServiceLoginUnknownEmailUsesDummyHash(
	t *testing.T,
) {
	t.Parallel()

	userStore := &loginFakeUserStore{
		getErr: store.ErrNotFound,
	}
	passwordHasher := &loginFakePasswordHasher{}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "unknown@example.com",
			Password: loginPassword,
		},
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"Login() error = %v, want %v",
			err,
			ErrInvalidCredentials,
		)
	}

	if passwordHasher.hashCalls != 1 {
		t.Fatalf(
			"Hash() calls = %d, want 1",
			passwordHasher.hashCalls,
		)
	}

	if passwordHasher.lastHashedPassword !=
		dummyPasswordForLoginTiming {
		t.Fatal(
			"Hash() did not receive the synthetic timing password",
		)
	}

	if passwordHasher.verifyCalls != 0 {
		t.Fatalf(
			"Verify() calls = %d, want 0",
			passwordHasher.verifyCalls,
		)
	}
}

func TestServiceLoginIncorrectPassword(
	t *testing.T,
) {
	t.Parallel()

	userStore := &loginFakeUserStore{
		user: activeLoginTestUser(),
	}
	passwordHasher := &loginFakePasswordHasher{
		verifyErr: ErrPasswordMismatch,
	}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "martin@example.com",
			Password: loginPassword,
		},
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"Login() error = %v, want %v",
			err,
			ErrInvalidCredentials,
		)
	}

	if passwordHasher.verifyCalls != 1 {
		t.Fatalf(
			"Verify() calls = %d, want 1",
			passwordHasher.verifyCalls,
		)
	}
}

func TestServiceLoginUnknownAndIncorrectMatch(
	t *testing.T,
) {
	t.Parallel()

	unknownService := NewService(
		&loginFakeUserStore{
			getErr: store.ErrNotFound,
		},
		&loginFakePasswordHasher{},
	)

	_, unknownErr := unknownService.Login(
		context.Background(),
		LoginInput{
			Email:    "unknown@example.com",
			Password: loginPassword,
		},
	)

	incorrectService := NewService(
		&loginFakeUserStore{
			user: activeLoginTestUser(),
		},
		&loginFakePasswordHasher{
			verifyErr: ErrPasswordMismatch,
		},
	)

	_, incorrectErr := incorrectService.Login(
		context.Background(),
		LoginInput{
			Email:    "martin@example.com",
			Password: loginPassword,
		},
	)

	if unknownErr != ErrInvalidCredentials {
		t.Fatalf(
			"unknown account error = %v",
			unknownErr,
		)
	}

	if incorrectErr != ErrInvalidCredentials {
		t.Fatalf(
			"incorrect password error = %v",
			incorrectErr,
		)
	}

	if unknownErr.Error() != incorrectErr.Error() {
		t.Fatalf(
			"login errors differ: %q and %q",
			unknownErr,
			incorrectErr,
		)
	}
}

func TestServiceLoginRejectsDisabledUser(
	t *testing.T,
) {
	t.Parallel()

	user := activeLoginTestUser()
	user.Status = "disabled"

	userStore := &loginFakeUserStore{
		user: user,
	}
	passwordHasher := &loginFakePasswordHasher{}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "martin@example.com",
			Password: loginPassword,
		},
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"Login() error = %v, want %v",
			err,
			ErrInvalidCredentials,
		)
	}

	if passwordHasher.verifyCalls != 1 {
		t.Fatalf(
			"Verify() calls = %d, want 1",
			passwordHasher.verifyCalls,
		)
	}
}

func TestServiceLoginInvalidInputUsesDummyHash(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name  string
		input LoginInput
	}{
		{
			name: "invalid email",
			input: LoginInput{
				Email:    "not-an-email",
				Password: loginPassword,
			},
		},
		{
			name: "invalid password",
			input: LoginInput{
				Email:    "martin@example.com",
				Password: "too short",
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			userStore := &loginFakeUserStore{}
			passwordHasher :=
				&loginFakePasswordHasher{}

			service := NewService(
				userStore,
				passwordHasher,
			)

			_, err := service.Login(
				context.Background(),
				test.input,
			)

			if !errors.Is(
				err,
				ErrInvalidCredentials,
			) {
				t.Fatalf(
					"Login() error = %v, want %v",
					err,
					ErrInvalidCredentials,
				)
			}

			if userStore.getCalls != 0 {
				t.Fatalf(
					"GetByEmail() calls = %d, want 0",
					userStore.getCalls,
				)
			}

			if passwordHasher.hashCalls != 1 {
				t.Fatalf(
					"Hash() calls = %d, want 1",
					passwordHasher.hashCalls,
				)
			}

			if passwordHasher.verifyCalls != 0 {
				t.Fatalf(
					"Verify() calls = %d, want 0",
					passwordHasher.verifyCalls,
				)
			}
		})
	}
}

func TestServiceLoginMapsStoreFailureSafely(
	t *testing.T,
) {
	t.Parallel()

	userStore := &loginFakeUserStore{
		getErr: store.ErrDatabase,
	}
	passwordHasher := &loginFakePasswordHasher{}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "martin@example.com",
			Password: loginPassword,
		},
	)

	if !errors.Is(
		err,
		ErrAuthenticationUnavailable,
	) {
		t.Fatalf(
			"Login() error = %v, want %v",
			err,
			ErrAuthenticationUnavailable,
		)
	}

	if passwordHasher.hashCalls != 0 ||
		passwordHasher.verifyCalls != 0 {
		t.Fatal(
			"password hasher was called after the store failed",
		)
	}
}

func TestServiceLoginMapsHasherFailureSafely(
	t *testing.T,
) {
	t.Parallel()

	userStore := &loginFakeUserStore{
		user: activeLoginTestUser(),
	}
	passwordHasher := &loginFakePasswordHasher{
		verifyErr: errors.New(
			"synthetic sensitive hasher detail",
		),
	}

	service := NewService(
		userStore,
		passwordHasher,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "martin@example.com",
			Password: loginPassword,
		},
	)

	if !errors.Is(
		err,
		ErrAuthenticationUnavailable,
	) {
		t.Fatalf(
			"Login() error = %v, want %v",
			err,
			ErrAuthenticationUnavailable,
		)
	}

	if strings.Contains(
		err.Error(),
		"synthetic sensitive hasher detail",
	) {
		t.Fatal(
			"Login() exposed the underlying hasher error",
		)
	}
}

func activeLoginTestUser() *store.User {
	return &store.User{
		ID:                "user-123",
		Email:             "martin@example.com",
		PasswordHash:      "encoded-password-hash",
		PasswordAlgorithm: AlgorithmArgon2id,
		Status:            "active",
	}
}

type loginFakeUserStore struct {
	user   *store.User
	getErr error

	getCalls  int
	lastEmail string
}

func (userStore *loginFakeUserStore) Create(
	_ context.Context,
	_ *store.User,
) error {
	return nil
}

func (userStore *loginFakeUserStore) GetByEmail(
	_ context.Context,
	email string,
) (*store.User, error) {
	userStore.getCalls++
	userStore.lastEmail = email

	if userStore.getErr != nil {
		return nil, userStore.getErr
	}

	return userStore.user, nil
}

type loginFakePasswordHasher struct {
	hashErr   error
	verifyErr error

	hashCalls   int
	verifyCalls int

	lastHashedPassword   string
	lastVerifiedPassword string
	lastVerifiedHash     PasswordHash
}

func (hasher *loginFakePasswordHasher) Hash(
	_ context.Context,
	password string,
) (PasswordHash, error) {
	hasher.hashCalls++
	hasher.lastHashedPassword = password

	if hasher.hashErr != nil {
		return PasswordHash{}, hasher.hashErr
	}

	return PasswordHash{
		Encoded:   "discarded-dummy-hash",
		Algorithm: "fake",
	}, nil
}

func (hasher *loginFakePasswordHasher) Verify(
	_ context.Context,
	password string,
	passwordHash PasswordHash,
) error {
	hasher.verifyCalls++
	hasher.lastVerifiedPassword = password
	hasher.lastVerifiedHash = passwordHash

	return hasher.verifyErr
}
