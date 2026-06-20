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

func TestSessionStoreRevokeOwnedFamily(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"revoke-owned-session@example.com",
	)

	now := time.Now().
		UTC().
		Truncate(time.Microsecond)

	session := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x71},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf(
			"create session: %v",
			err,
		)
	}

	revokedAt := session.CreatedAt.Add(
		time.Second,
	)

	err := sessionStore.RevokeOwnedFamily(
		context.Background(),
		user.ID,
		session.TokenFamilyID,
		revokedAt,
	)
	if err != nil {
		t.Fatalf(
			"revoke owned session family: %v",
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
			"expected revoked session to return ErrNotFound, got %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var storedRevokedAt time.Time

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT revoked_at
			FROM sessions
			WHERE token_family_id = $1::uuid
		`,
		session.TokenFamilyID,
	).Scan(&storedRevokedAt)
	if err != nil {
		t.Fatalf(
			"read revoked session: %v",
			err,
		)
	}

	if !storedRevokedAt.Equal(revokedAt) {
		t.Fatalf(
			"stored revocation time = %v, want %v",
			storedRevokedAt,
			revokedAt,
		)
	}
}

func TestSessionStoreRevokeOwnedFamilyRejectsOtherUser(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	owner := createSessionTestUser(
		t,
		userStore,
		"revoke-session-owner@example.com",
	)

	otherUser := createSessionTestUser(
		t,
		userStore,
		"revoke-session-other@example.com",
	)

	now := time.Now().
		UTC().
		Truncate(time.Microsecond)

	session := &Session{
		UserID: owner.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x72},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf(
			"create session: %v",
			err,
		)
	}

	err := sessionStore.RevokeOwnedFamily(
		context.Background(),
		otherUser.ID,
		session.TokenFamilyID,
		now.Add(time.Second),
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}

	_, err = sessionStore.GetActiveState(
		context.Background(),
		owner.ID,
		session.TokenFamilyID,
		now,
	)
	if err != nil {
		t.Fatalf(
			"owner session should remain active: %v",
			err,
		)
	}
}

func TestSessionStoreRevokeOwnedFamilyRejectsUnknownFamily(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"revoke-unknown-session@example.com",
	)

	err := sessionStore.RevokeOwnedFamily(
		context.Background(),
		user.ID,
		"00000000-0000-0000-0000-000000000001",
		time.Now().UTC(),
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestSessionStoreRevokeOwnedFamilyRejectsAlreadyRevokedFamily(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"revoke-twice-session@example.com",
	)

	session := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x73},
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
		t.Fatalf(
			"create session: %v",
			err,
		)
	}

	firstRevocation := session.CreatedAt.Add(
		time.Second,
	)

	if err := sessionStore.RevokeOwnedFamily(
		context.Background(),
		user.ID,
		session.TokenFamilyID,
		firstRevocation,
	); err != nil {
		t.Fatalf(
			"first family revocation: %v",
			err,
		)
	}

	err := sessionStore.RevokeOwnedFamily(
		context.Background(),
		user.ID,
		session.TokenFamilyID,
		firstRevocation.Add(time.Second),
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestSessionStoreRevokeOwnedFamilyHonorsCanceledContext(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := sessionStore.RevokeOwnedFamily(
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

func TestSessionStoreRevokeOwnedFamilyMapsDatabaseFailureSafely(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	err := sessionStore.RevokeOwnedFamily(
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
			"session revocation exposed raw PostgreSQL details",
		)
	}
}

func TestSessionStoreRevokeAllForUser(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"revoke-all-sessions@example.com",
	)

	otherUser := createSessionTestUser(
		t,
		userStore,
		"revoke-all-other-user@example.com",
	)

	now := time.Now().
		UTC().
		Truncate(time.Microsecond)

	firstSession := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x81},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		firstSession,
	); err != nil {
		t.Fatalf(
			"create first session: %v",
			err,
		)
	}

	secondSession := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x82},
			sha256.Size,
		),
		ExpiresAt: now.Add(48 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		secondSession,
	); err != nil {
		t.Fatalf(
			"create second session: %v",
			err,
		)
	}

	otherSession := &Session{
		UserID: otherUser.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x83},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		otherSession,
	); err != nil {
		t.Fatalf(
			"create other-user session: %v",
			err,
		)
	}

	revokedAt := secondSession.CreatedAt.Add(
		time.Second,
	)

	if err := sessionStore.RevokeAllForUser(
		context.Background(),
		user.ID,
		revokedAt,
	); err != nil {
		t.Fatalf(
			"revoke all user sessions: %v",
			err,
		)
	}

	_, err := sessionStore.GetActiveState(
		context.Background(),
		user.ID,
		firstSession.TokenFamilyID,
		revokedAt,
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected first session to be revoked, got %v",
			err,
		)
	}

	_, err = sessionStore.GetActiveState(
		context.Background(),
		user.ID,
		secondSession.TokenFamilyID,
		revokedAt,
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected second session to be revoked, got %v",
			err,
		)
	}

	_, err = sessionStore.GetActiveState(
		context.Background(),
		otherUser.ID,
		otherSession.TokenFamilyID,
		revokedAt,
	)
	if err != nil {
		t.Fatalf(
			"other user's session should remain active: %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var revokedCount int

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT count(*)
			FROM sessions
			WHERE user_id = $1::uuid
			  AND revoked_at = $2
		`,
		user.ID,
		revokedAt,
	).Scan(&revokedCount)
	if err != nil {
		t.Fatalf(
			"count revoked sessions: %v",
			err,
		)
	}

	if revokedCount != 2 {
		t.Fatalf(
			"revoked session count = %d, want 2",
			revokedCount,
		)
	}
}

func TestSessionStoreRevokeAllForUserIsIdempotent(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"revoke-all-idempotent@example.com",
	)

	session := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x84},
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
		t.Fatalf(
			"create session: %v",
			err,
		)
	}

	firstRevocation := session.CreatedAt.Add(
		time.Second,
	)

	if err := sessionStore.RevokeAllForUser(
		context.Background(),
		user.ID,
		firstRevocation,
	); err != nil {
		t.Fatalf(
			"first revoke-all operation: %v",
			err,
		)
	}

	if err := sessionStore.RevokeAllForUser(
		context.Background(),
		user.ID,
		firstRevocation.Add(time.Second),
	); err != nil {
		t.Fatalf(
			"second revoke-all operation: %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var storedRevokedAt time.Time

	err := testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT revoked_at
			FROM sessions
			WHERE token_family_id = $1::uuid
		`,
		session.TokenFamilyID,
	).Scan(&storedRevokedAt)
	if err != nil {
		t.Fatalf(
			"read revoked session: %v",
			err,
		)
	}

	if !storedRevokedAt.Equal(
		firstRevocation,
	) {
		t.Fatalf(
			"stored revocation time = %v, want %v",
			storedRevokedAt,
			firstRevocation,
		)
	}
}

func TestSessionStoreRevokeAllForUserAllowsNoSessions(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"revoke-all-empty@example.com",
	)

	err := sessionStore.RevokeAllForUser(
		context.Background(),
		user.ID,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf(
			"revoke all with no sessions: %v",
			err,
		)
	}
}

func TestSessionStoreRevokeAllForUserHonorsCanceledContext(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := sessionStore.RevokeAllForUser(
		ctx,
		"00000000-0000-0000-0000-000000000001",
		time.Now().UTC(),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}
}

func TestSessionStoreRevokeAllForUserMapsDatabaseFailureSafely(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	err := sessionStore.RevokeAllForUser(
		context.Background(),
		"not-a-valid-uuid",
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
			"revoke-all exposed raw PostgreSQL details",
		)
	}
}
