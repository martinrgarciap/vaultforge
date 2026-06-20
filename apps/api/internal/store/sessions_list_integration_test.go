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

func TestSessionStoreListActive(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"list-active-sessions@example.com",
	)

	now := time.Now().
		UTC().
		Truncate(time.Microsecond)

	firstUserAgent := "Thunder Client"
	secondUserAgent := "Firefox"

	firstSession := &Session{
		UserID: user.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x61},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
		UserAgent: &firstUserAgent,
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
			[]byte{0x62},
			sha256.Size,
		),
		ExpiresAt: now.Add(48 * time.Hour),
		UserAgent: &secondUserAgent,
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

	summaries, err := sessionStore.ListActive(
		context.Background(),
		user.ID,
		now,
	)
	if err != nil {
		t.Fatalf(
			"list active sessions: %v",
			err,
		)
	}

	if len(summaries) != 2 {
		t.Fatalf(
			"active session count = %d, want 2",
			len(summaries),
		)
	}

	summariesByFamily := make(
		map[string]SessionSummary,
		len(summaries),
	)

	for _, summary := range summaries {
		summariesByFamily[summary.TokenFamilyID] = summary
	}

	firstSummary, found :=
		summariesByFamily[firstSession.TokenFamilyID]
	if !found {
		t.Fatal(
			"first active session was not listed",
		)
	}

	if firstSummary.UserAgent !=
		firstUserAgent {
		t.Fatalf(
			"first user agent = %q, want %q",
			firstSummary.UserAgent,
			firstUserAgent,
		)
	}

	if !firstSummary.ExpiresAt.Equal(
		firstSession.ExpiresAt,
	) {
		t.Fatalf(
			"first expiry = %v, want %v",
			firstSummary.ExpiresAt,
			firstSession.ExpiresAt,
		)
	}

	secondSummary, found :=
		summariesByFamily[secondSession.TokenFamilyID]
	if !found {
		t.Fatal(
			"second active session was not listed",
		)
	}

	if secondSummary.UserAgent !=
		secondUserAgent {
		t.Fatalf(
			"second user agent = %q, want %q",
			secondSummary.UserAgent,
			secondUserAgent,
		)
	}
}

func TestSessionStoreListActiveFiltersSessions(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	owner := createSessionTestUser(
		t,
		userStore,
		"list-session-owner@example.com",
	)

	otherUser := createSessionTestUser(
		t,
		userStore,
		"list-session-other@example.com",
	)

	now := time.Now().
		UTC().
		Truncate(time.Microsecond)

	activeSession := &Session{
		UserID: owner.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x63},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		activeSession,
	); err != nil {
		t.Fatalf(
			"create active session: %v",
			err,
		)
	}

	revokedSession := &Session{
		UserID: owner.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x64},
			sha256.Size,
		),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		revokedSession,
	); err != nil {
		t.Fatalf(
			"create revoked session: %v",
			err,
		)
	}

	expiringSession := &Session{
		UserID: owner.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x65},
			sha256.Size,
		),
		ExpiresAt: now.Add(time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		expiringSession,
	); err != nil {
		t.Fatalf(
			"create expiring session: %v",
			err,
		)
	}

	otherSession := &Session{
		UserID: otherUser.ID,
		RefreshTokenHash: bytes.Repeat(
			[]byte{0x66},
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

	revokedAt := revokedSession.CreatedAt.Add(
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
		revokedSession.TokenFamilyID,
	)
	if err != nil {
		t.Fatalf(
			"revoke session: %v",
			err,
		)
	}

	summaries, err := sessionStore.ListActive(
		context.Background(),
		owner.ID,
		now.Add(2*time.Hour),
	)
	if err != nil {
		t.Fatalf(
			"list filtered sessions: %v",
			err,
		)
	}

	if len(summaries) != 1 {
		t.Fatalf(
			"active session count = %d, want 1",
			len(summaries),
		)
	}

	if summaries[0].TokenFamilyID !=
		activeSession.TokenFamilyID {
		t.Fatalf(
			"listed family ID = %q, want %q",
			summaries[0].TokenFamilyID,
			activeSession.TokenFamilyID,
		)
	}
}

func TestSessionStoreListActiveReturnsEmptySlice(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"list-empty-sessions@example.com",
	)

	summaries, err := sessionStore.ListActive(
		context.Background(),
		user.ID,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf(
			"list active sessions: %v",
			err,
		)
	}

	if summaries == nil {
		t.Fatal(
			"expected an empty non-nil slice",
		)
	}

	if len(summaries) != 0 {
		t.Fatalf(
			"active session count = %d, want 0",
			len(summaries),
		)
	}
}

func TestSessionStoreListActiveHonorsCanceledContext(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := sessionStore.ListActive(
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

func TestSessionStoreListActiveMapsDatabaseFailureSafely(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	_, err := sessionStore.ListActive(
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
			"session listing exposed raw PostgreSQL details",
		)
	}
}
