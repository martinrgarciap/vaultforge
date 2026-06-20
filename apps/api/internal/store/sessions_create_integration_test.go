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

func TestSessionStoreCreate(t *testing.T) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"session-create@example.com",
	)

	userAgent := "Thunder Client"

	expiresAt := time.Now().
		UTC().
		Truncate(time.Microsecond).
		Add(30 * 24 * time.Hour)

	refreshTokenHash := bytes.Repeat(
		[]byte{0x42},
		sha256.Size,
	)

	session := &Session{
		UserID: user.ID,
		RefreshTokenHash: append(
			[]byte(nil),
			refreshTokenHash...,
		),
		ExpiresAt: expiresAt,
		UserAgent: &userAgent,
	}

	err := sessionStore.Create(
		context.Background(),
		session,
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if session.ID == "" {
		t.Fatal(
			"expected database-generated session ID",
		)
	}

	if session.TokenFamilyID == "" {
		t.Fatal(
			"expected database-generated token family ID",
		)
	}

	if session.CreatedAt.IsZero() {
		t.Fatal(
			"expected database-generated creation timestamp",
		)
	}

	var (
		storedUserID           string
		storedRefreshTokenHash []byte
		storedTokenFamilyID    string
		storedExpiresAt        time.Time
		revokedAtIsNull        bool
		storedUserAgent        string
		storedCreatedAt        time.Time
	)

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				user_id::text,
				refresh_token_hash,
				token_family_id::text,
				expires_at,
				revoked_at IS NULL,
				COALESCE(user_agent, ''),
				created_at
			FROM sessions
			WHERE id = $1::uuid
		`,
		session.ID,
	).Scan(
		&storedUserID,
		&storedRefreshTokenHash,
		&storedTokenFamilyID,
		&storedExpiresAt,
		&revokedAtIsNull,
		&storedUserAgent,
		&storedCreatedAt,
	)
	if err != nil {
		t.Fatalf(
			"read stored session: %v",
			err,
		)
	}

	if storedUserID != user.ID {
		t.Fatalf(
			"stored user ID = %q, want %q",
			storedUserID,
			user.ID,
		)
	}

	if !bytes.Equal(
		storedRefreshTokenHash,
		refreshTokenHash,
	) {
		t.Fatal(
			"stored refresh-token digest did not match",
		)
	}

	if storedTokenFamilyID !=
		session.TokenFamilyID {
		t.Fatalf(
			"stored token family ID = %q, want %q",
			storedTokenFamilyID,
			session.TokenFamilyID,
		)
	}

	if !storedExpiresAt.Equal(expiresAt) {
		t.Fatalf(
			"stored expiry = %v, want %v",
			storedExpiresAt,
			expiresAt,
		)
	}

	if !revokedAtIsNull {
		t.Fatal(
			"new session should not be revoked",
		)
	}

	if storedUserAgent != userAgent {
		t.Fatalf(
			"stored user agent = %q, want %q",
			storedUserAgent,
			userAgent,
		)
	}

	if !storedCreatedAt.Equal(
		session.CreatedAt,
	) {
		t.Fatal(
			"returned creation timestamp did not match PostgreSQL",
		)
	}
}

func TestSessionStoreCreateAllowsMissingUserAgent(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"session-no-agent@example.com",
	)

	session := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x43},
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

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var userAgentIsNull bool

	err := testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT user_agent IS NULL
			FROM sessions
			WHERE id = $1::uuid
		`,
		session.ID,
	).Scan(&userAgentIsNull)
	if err != nil {
		t.Fatalf(
			"read session user agent: %v",
			err,
		)
	}

	if !userAgentIsNull {
		t.Fatal(
			"expected missing user agent to be stored as NULL",
		)
	}
}

func TestSessionStoreCreateMapsDatabaseFailureSafely(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	const internalIdentifier = "sessions_user_id_fkey"

	session := &Session{
		UserID: "00000000-0000-0000-0000-000000000001",
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x44},
			sha256.Size,
		),
		ExpiresAt: time.Now().
			UTC().
			Add(24 * time.Hour),
	}

	err := sessionStore.Create(
		context.Background(),
		session,
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf(
			"expected ErrDatabase, got %v",
			err,
		)
	}

	if strings.Contains(
		err.Error(),
		internalIdentifier,
	) {
		t.Fatal(
			"session store error exposed a database constraint",
		)
	}

	if strings.Contains(
		err.Error(),
		"foreign key",
	) {
		t.Fatal(
			"session store error exposed raw PostgreSQL details",
		)
	}
}

func TestSessionStoreCreateHonorsCanceledContext(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"session-context@example.com",
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := sessionStore.Create(
		ctx,
		&Session{
			UserID: user.ID,
			RefreshTokenHash: bytes.Repeat(
				[]byte{0x45},
				sha256.Size,
			),
			ExpiresAt: time.Now().
				UTC().
				Add(24 * time.Hour),
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}
}
