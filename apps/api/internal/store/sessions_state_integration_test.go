package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSessionStoreGetActiveState(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"active-session-state@example.com",
	)

	now := time.Now().
		UTC().
		Truncate(time.Microsecond)

	session := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x51},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf("create session: %v", err)
	}

	state, err := sessionStore.GetActiveState(
		context.Background(),
		user.ID,
		session.TokenFamilyID,
		now,
	)
	if err != nil {
		t.Fatalf(
			"get active session state: %v",
			err,
		)
	}

	if state.UserID != user.ID {
		t.Fatalf(
			"state user ID = %q, want %q",
			state.UserID,
			user.ID,
		)
	}

	if state.TokenFamilyID !=
		session.TokenFamilyID {
		t.Fatalf(
			"state token family ID = %q, want %q",
			state.TokenFamilyID,
			session.TokenFamilyID,
		)
	}

	if !state.ExpiresAt.Equal(
		session.ExpiresAt,
	) {
		t.Fatalf(
			"state expiry = %v, want %v",
			state.ExpiresAt,
			session.ExpiresAt,
		)
	}
}

func TestSessionStoreGetActiveStateRejectsOtherUser(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	owner := createSessionTestUser(
		t,
		userStore,
		"session-owner@example.com",
	)

	otherUser := createSessionTestUser(
		t,
		userStore,
		"other-session-user@example.com",
	)

	now := time.Now().UTC()

	session := &Session{
		UserID: owner.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x52},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err := sessionStore.GetActiveState(
		context.Background(),
		otherUser.ID,
		session.TokenFamilyID,
		now,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestSessionStoreGetActiveStateRejectsRevokedFamily(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"revoked-session-state@example.com",
	)

	session := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x53},
			sha256.Size,
		),
		ExpiresAt: time.Now().
			UTC().
			Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf("create session: %v", err)
	}

	revokedAt := session.CreatedAt.Add(
		time.Second,
	)

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	_, err := testDatabasePool.Exec(
		queryContext,
		`
			UPDATE sessions
			SET revoked_at = $1
			WHERE token_family_id = $2::uuid
		`,
		revokedAt,
		session.TokenFamilyID,
	)
	if err != nil {
		t.Fatalf(
			"revoke session family: %v",
			err,
		)
	}

	_, err = sessionStore.GetActiveState(
		context.Background(),
		user.ID,
		session.TokenFamilyID,
		revokedAt,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestSessionStoreGetActiveStateRejectsExpiredFamily(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"expired-session-state@example.com",
	)

	now := time.Now().
		UTC().
		Truncate(time.Microsecond)

	session := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x54},
			sha256.Size,
		),
		ExpiresAt: now.Add(time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err := sessionStore.GetActiveState(
		context.Background(),
		user.ID,
		session.TokenFamilyID,
		now.Add(2*time.Hour),
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestSessionStoreGetActiveStateRejectsDisabledUser(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"disabled-session-state@example.com",
	)

	now := time.Now().UTC()

	session := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x55},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf("create session: %v", err)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	_, err := testDatabasePool.Exec(
		queryContext,
		`
			UPDATE users
			SET
				status = 'disabled',
				updated_at = now()
			WHERE id = $1::uuid
		`,
		user.ID,
	)
	if err != nil {
		t.Fatalf("disable user: %v", err)
	}

	_, err = sessionStore.GetActiveState(
		context.Background(),
		user.ID,
		session.TokenFamilyID,
		now,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestSessionStoreGetActiveStateHonorsCanceledContext(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := sessionStore.GetActiveState(
		ctx,
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		time.Now().UTC(),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}
}

func TestSessionStoreGetActiveStateMapsDatabaseFailureSafely(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	_, err := sessionStore.GetActiveState(
		context.Background(),
		"not-a-valid-uuid",
		"also-not-a-valid-uuid",
		time.Now().UTC(),
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf(
			"expected ErrDatabase, got %v",
			err,
		)
	}

	if strings.Contains(
		err.Error(),
		"invalid input syntax",
	) {
		t.Fatal(
			"session lookup exposed raw PostgreSQL details",
		)
	}
}
