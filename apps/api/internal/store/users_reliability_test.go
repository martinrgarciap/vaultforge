package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestUserStoreCreateHonorsCanceledContext(
	t *testing.T,
) {
	t.Parallel()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := (*UserStore)(nil).Create(
		ctx,
		&User{},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}

func TestUserStoreGetByEmailHonorsExpiredDeadline(
	t *testing.T,
) {
	t.Parallel()

	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(-time.Second),
	)
	defer cancel()

	_, err := (*UserStore)(nil).GetByEmail(
		ctx,
		"person@example.test",
	)

	if !errors.Is(
		err,
		context.DeadlineExceeded,
	) {
		t.Fatalf(
			"GetByEmail() error = %v, want %v",
			err,
			context.DeadlineExceeded,
		)
	}
}

func TestUserStoreRejectsUnavailableDatabase(
	t *testing.T,
) {
	t.Parallel()

	store := NewUserStore(nil)

	createErr := store.Create(
		context.Background(),
		&User{},
	)
	if !errors.Is(createErr, ErrDatabase) {
		t.Fatalf(
			"Create() error = %v, want %v",
			createErr,
			ErrDatabase,
		)
	}

	_, getErr := store.GetByEmail(
		context.Background(),
		"person@example.test",
	)
	if !errors.Is(getErr, ErrDatabase) {
		t.Fatalf(
			"GetByEmail() error = %v, want %v",
			getErr,
			ErrDatabase,
		)
	}
}

func TestUserStoreCreateRejectsNilUser(
	t *testing.T,
) {
	t.Parallel()

	store := NewUserStore(nil)

	err := store.Create(
		context.Background(),
		nil,
	)
	if !errors.Is(err, ErrDatabase) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			ErrDatabase,
		)
	}
}
