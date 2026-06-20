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

func TestSessionStoreRotateRefreshToken(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"rotate-refresh-token@example.com",
	)

	userAgent := "Thunder Client"

	currentHash := bytes.Repeat(
		[]byte{0x91},
		sha256.Size,
	)

	replacementHash := bytes.Repeat(
		[]byte{0x92},
		sha256.Size,
	)
	expiresAt := time.Now().
		UTC().
		Add(30 * 24 * time.Hour).
		Truncate(time.Microsecond)

	session := &Session{
		UserID:           user.ID,
		RefreshTokenHash: currentHash,
		ExpiresAt:        expiresAt,
		UserAgent:        &userAgent,
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf(
			"create original session: %v",
			err,
		)
	}

	rotatedAt := session.CreatedAt.Add(
		time.Second,
	)

	rotation, err :=
		sessionStore.RotateRefreshToken(
			context.Background(),
			currentHash,
			replacementHash,
			rotatedAt,
		)
	if err != nil {
		t.Fatalf(
			"rotate refresh token: %v",
			err,
		)
	}

	if rotation.ID == "" {
		t.Fatal(
			"expected replacement session ID",
		)
	}

	if rotation.ID == session.ID {
		t.Fatal(
			"replacement reused the original row ID",
		)
	}

	if rotation.UserID != user.ID {
		t.Fatalf(
			"rotation user ID = %q, want %q",
			rotation.UserID,
			user.ID,
		)
	}

	if rotation.TokenFamilyID !=
		session.TokenFamilyID {
		t.Fatalf(
			"rotation family ID = %q, want %q",
			rotation.TokenFamilyID,
			session.TokenFamilyID,
		)
	}

	if rotation.UserAgent != userAgent {
		t.Fatalf(
			"rotation user agent = %q, want %q",
			rotation.UserAgent,
			userAgent,
		)
	}

	if !rotation.ExpiresAt.Equal(
		expiresAt,
	) {
		t.Fatalf(
			"rotation expiry = %v, want %v",
			rotation.ExpiresAt,
			session.ExpiresAt,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var (
		storedOldRevokedAt time.Time
		storedNewFamilyID  string
		storedNewExpiresAt time.Time
		newIsRevoked       bool
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT revoked_at
			FROM sessions
			WHERE id = $1::uuid
		`,
		session.ID,
	).Scan(&storedOldRevokedAt)
	if err != nil {
		t.Fatalf(
			"read old session row: %v",
			err,
		)
	}

	if !storedOldRevokedAt.Equal(
		rotatedAt,
	) {
		t.Fatalf(
			"old revocation time = %v, want %v",
			storedOldRevokedAt,
			rotatedAt,
		)
	}

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				token_family_id::text,
				expires_at,
				revoked_at IS NOT NULL
			FROM sessions
			WHERE refresh_token_hash = $1
		`,
		replacementHash,
	).Scan(
		&storedNewFamilyID,
		&storedNewExpiresAt,
		&newIsRevoked,
	)
	if err != nil {
		t.Fatalf(
			"read replacement session row: %v",
			err,
		)
	}

	if storedNewFamilyID !=
		session.TokenFamilyID {
		t.Fatalf(
			"stored replacement family ID = %q, want %q",
			storedNewFamilyID,
			session.TokenFamilyID,
		)
	}

	if !storedNewExpiresAt.Equal(
		expiresAt,
	) {
		t.Fatalf(
			"stored replacement expiry = %v, want %v",
			storedNewExpiresAt,
			session.ExpiresAt,
		)
	}

	if newIsRevoked {
		t.Fatal(
			"replacement session should be active",
		)
	}
}

func TestSessionStoreRotateRefreshTokenRollsBackOnInsertFailure(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"rotate-rollback@example.com",
	)

	currentHash := bytes.Repeat(
		[]byte{0x93},
		sha256.Size,
	)

	session := &Session{
		UserID:           user.ID,
		RefreshTokenHash: currentHash,
		ExpiresAt: time.Now().
			UTC().
			Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf(
			"create original session: %v",
			err,
		)
	}

	rotatedAt := session.CreatedAt.Add(
		time.Second,
	)

	_, err := sessionStore.RotateRefreshToken(
		context.Background(),
		currentHash,
		[]byte{},
		rotatedAt,
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf(
			"expected ErrDatabase, got %v",
			err,
		)
	}

	if strings.Contains(
		err.Error(),
		"sessions_refresh_token_hash_not_empty",
	) {
		t.Fatal(
			"rotation error exposed a database constraint",
		)
	}

	_, err = sessionStore.GetActiveState(
		context.Background(),
		user.ID,
		session.TokenFamilyID,
		rotatedAt,
	)
	if err != nil {
		t.Fatalf(
			"original session should remain active after rollback: %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var revokedAtIsNull bool

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT revoked_at IS NULL
			FROM sessions
			WHERE id = $1::uuid
		`,
		session.ID,
	).Scan(&revokedAtIsNull)
	if err != nil {
		t.Fatalf(
			"read original session: %v",
			err,
		)
	}

	if !revokedAtIsNull {
		t.Fatal(
			"failed rotation should not revoke the original session",
		)
	}
}

func TestSessionStoreRotateRefreshTokenDetectsReplayAndRevokesFamily(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"refresh-replay@example.com",
	)

	currentHash := bytes.Repeat(
		[]byte{0xa1},
		sha256.Size,
	)

	replacementHash := bytes.Repeat(
		[]byte{0xa2},
		sha256.Size,
	)

	replayReplacementHash := bytes.Repeat(
		[]byte{0xa3},
		sha256.Size,
	)

	session := &Session{
		UserID:           user.ID,
		RefreshTokenHash: currentHash,
		ExpiresAt: time.Now().
			UTC().
			Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf(
			"create original session: %v",
			err,
		)
	}

	firstRotationAt := session.CreatedAt.Add(
		time.Second,
	)

	rotation, err :=
		sessionStore.RotateRefreshToken(
			context.Background(),
			currentHash,
			replacementHash,
			firstRotationAt,
		)
	if err != nil {
		t.Fatalf(
			"first refresh rotation: %v",
			err,
		)
	}

	replayDetectedAt := firstRotationAt.Add(
		time.Second,
	)

	_, err = sessionStore.RotateRefreshToken(
		context.Background(),
		currentHash,
		replayReplacementHash,
		replayDetectedAt,
	)

	if !errors.Is(
		err,
		ErrSessionReplayDetected,
	) {
		t.Fatalf(
			"expected ErrSessionReplayDetected, got %v",
			err,
		)
	}

	_, err = sessionStore.GetActiveState(
		context.Background(),
		user.ID,
		rotation.TokenFamilyID,
		replayDetectedAt,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected replayed family to be inactive, got %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var replacementRevokedAt time.Time

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT revoked_at
			FROM sessions
			WHERE refresh_token_hash = $1
		`,
		replacementHash,
	).Scan(&replacementRevokedAt)
	if err != nil {
		t.Fatalf(
			"read replacement session: %v",
			err,
		)
	}

	if !replacementRevokedAt.Equal(
		replayDetectedAt,
	) {
		t.Fatalf(
			"replacement revocation time = %v, want %v",
			replacementRevokedAt,
			replayDetectedAt,
		)
	}

	var replayReplacementCount int

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT count(*)
			FROM sessions
			WHERE refresh_token_hash = $1
		`,
		replayReplacementHash,
	).Scan(&replayReplacementCount)
	if err != nil {
		t.Fatalf(
			"count replay replacement rows: %v",
			err,
		)
	}

	if replayReplacementCount != 0 {
		t.Fatalf(
			"replay replacement row count = %d, want 0",
			replayReplacementCount,
		)
	}
}

func TestSessionStoreRotateRefreshTokenRejectsDisabledUserAndRevokesFamily(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"disabled-refresh-user@example.com",
	)

	currentHash := bytes.Repeat(
		[]byte{0xa4},
		sha256.Size,
	)

	replacementHash := bytes.Repeat(
		[]byte{0xa5},
		sha256.Size,
	)

	session := &Session{
		UserID:           user.ID,
		RefreshTokenHash: currentHash,
		ExpiresAt: time.Now().
			UTC().
			Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf(
			"create original session: %v",
			err,
		)
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
		t.Fatalf(
			"disable session user: %v",
			err,
		)
	}

	rotatedAt := session.CreatedAt.Add(
		time.Second,
	)

	_, err = sessionStore.RotateRefreshToken(
		context.Background(),
		currentHash,
		replacementHash,
		rotatedAt,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}

	var storedRevokedAt time.Time

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT revoked_at
			FROM sessions
			WHERE id = $1::uuid
		`,
		session.ID,
	).Scan(&storedRevokedAt)
	if err != nil {
		t.Fatalf(
			"read disabled-user session: %v",
			err,
		)
	}

	if !storedRevokedAt.Equal(rotatedAt) {
		t.Fatalf(
			"stored revocation time = %v, want %v",
			storedRevokedAt,
			rotatedAt,
		)
	}

	var replacementCount int

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT count(*)
			FROM sessions
			WHERE refresh_token_hash = $1
		`,
		replacementHash,
	).Scan(&replacementCount)
	if err != nil {
		t.Fatalf(
			"count disabled-user replacement rows: %v",
			err,
		)
	}

	if replacementCount != 0 {
		t.Fatalf(
			"replacement row count = %d, want 0",
			replacementCount,
		)
	}
}

func TestSessionStoreRotateRefreshTokenRejectsExpiredToken(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"expired-refresh-token@example.com",
	)

	currentHash := bytes.Repeat(
		[]byte{0xa6},
		sha256.Size,
	)

	replacementHash := bytes.Repeat(
		[]byte{0xa7},
		sha256.Size,
	)

	now := time.Now().
		UTC().
		Truncate(time.Microsecond)

	session := &Session{
		UserID:           user.ID,
		RefreshTokenHash: currentHash,
		ExpiresAt:        now.Add(time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf(
			"create expiring session: %v",
			err,
		)
	}

	_, err := sessionStore.RotateRefreshToken(
		context.Background(),
		currentHash,
		replacementHash,
		now.Add(2*time.Hour),
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var originalRevoked bool

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT revoked_at IS NOT NULL
			FROM sessions
			WHERE id = $1::uuid
		`,
		session.ID,
	).Scan(&originalRevoked)
	if err != nil {
		t.Fatalf(
			"read expired session: %v",
			err,
		)
	}

	if originalRevoked {
		t.Fatal(
			"expired-token rejection should not alter the original row",
		)
	}

	var replacementCount int

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT count(*)
			FROM sessions
			WHERE refresh_token_hash = $1
		`,
		replacementHash,
	).Scan(&replacementCount)
	if err != nil {
		t.Fatalf(
			"count expired-token replacement rows: %v",
			err,
		)
	}

	if replacementCount != 0 {
		t.Fatalf(
			"replacement row count = %d, want 0",
			replacementCount,
		)
	}
}

func TestSessionStoreRotateRefreshTokenRejectsUnknownToken(
	t *testing.T,
) {
	sessionStore, _ :=
		newIntegrationTestSessionStores(t)

	_, err := sessionStore.RotateRefreshToken(
		context.Background(),
		bytes.Repeat(
			[]byte{0xa8},
			sha256.Size,
		),
		bytes.Repeat(
			[]byte{0xa9},
			sha256.Size,
		),
		time.Now().UTC(),
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestSessionStoreRotateRefreshTokenSerializesConcurrentUse(
	t *testing.T,
) {
	sessionStore, userStore :=
		newIntegrationTestSessionStores(t)

	user := createSessionTestUser(
		t,
		userStore,
		"concurrent-refresh@example.com",
	)

	currentHash := bytes.Repeat(
		[]byte{0xb1},
		sha256.Size,
	)

	firstReplacementHash := bytes.Repeat(
		[]byte{0xb2},
		sha256.Size,
	)

	secondReplacementHash := bytes.Repeat(
		[]byte{0xb3},
		sha256.Size,
	)

	session := &Session{
		UserID:           user.ID,
		RefreshTokenHash: currentHash,
		ExpiresAt: time.Now().
			UTC().
			Add(24 * time.Hour),
	}

	if err := sessionStore.Create(
		context.Background(),
		session,
	); err != nil {
		t.Fatalf(
			"create original session: %v",
			err,
		)
	}

	rotatedAt := session.CreatedAt.Add(
		time.Second,
	)

	type rotationAttempt struct {
		name            string
		replacementHash []byte
		rotation        SessionRotation
		err             error
	}

	start := make(chan struct{})

	results := make(
		chan rotationAttempt,
		2,
	)

	runRotation := func(
		name string,
		replacementHash []byte,
	) {
		<-start

		rotation, err :=
			sessionStore.RotateRefreshToken(
				context.Background(),
				currentHash,
				replacementHash,
				rotatedAt,
			)

		results <- rotationAttempt{
			name:            name,
			replacementHash: replacementHash,
			rotation:        rotation,
			err:             err,
		}
	}

	go runRotation(
		"first",
		firstReplacementHash,
	)

	go runRotation(
		"second",
		secondReplacementHash,
	)

	close(start)

	receiveResult := func() rotationAttempt {
		t.Helper()

		select {
		case result := <-results:
			return result

		case <-time.After(10 * time.Second):
			t.Fatal(
				"timed out waiting for concurrent rotation",
			)

			return rotationAttempt{}
		}
	}

	attempts := []rotationAttempt{
		receiveResult(),
		receiveResult(),
	}

	var (
		successfulAttempt *rotationAttempt
		replayedAttempt   *rotationAttempt
	)

	for index := range attempts {
		attempt := &attempts[index]

		switch {
		case attempt.err == nil:
			if successfulAttempt != nil {
				t.Fatal(
					"both concurrent refresh attempts succeeded",
				)
			}

			successfulAttempt = attempt

		case errors.Is(
			attempt.err,
			ErrSessionReplayDetected,
		):
			if replayedAttempt != nil {
				t.Fatal(
					"both concurrent refresh attempts reported replay",
				)
			}

			replayedAttempt = attempt

		default:
			t.Fatalf(
				"%s rotation returned unexpected error: %v",
				attempt.name,
				attempt.err,
			)
		}
	}

	if successfulAttempt == nil {
		t.Fatal(
			"expected one concurrent rotation to succeed",
		)
	}

	if replayedAttempt == nil {
		t.Fatal(
			"expected one concurrent rotation to detect replay",
		)
	}

	if successfulAttempt.rotation.TokenFamilyID !=
		session.TokenFamilyID {
		t.Fatalf(
			"successful rotation family ID = %q, want %q",
			successfulAttempt.rotation.TokenFamilyID,
			session.TokenFamilyID,
		)
	}

	_, err := sessionStore.GetActiveState(
		context.Background(),
		user.ID,
		session.TokenFamilyID,
		rotatedAt,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected concurrent replay to revoke the family, got %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var (
		familyRowCount       int
		activeFamilyRowCount int
		successfulHashCount  int
		replayedHashCount    int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				count(*),
				count(*) FILTER (
					WHERE revoked_at IS NULL
				),
				count(*) FILTER (
					WHERE refresh_token_hash = $2
				),
				count(*) FILTER (
					WHERE refresh_token_hash = $3
				)
			FROM sessions
			WHERE token_family_id = $1::uuid
		`,
		session.TokenFamilyID,
		successfulAttempt.replacementHash,
		replayedAttempt.replacementHash,
	).Scan(
		&familyRowCount,
		&activeFamilyRowCount,
		&successfulHashCount,
		&replayedHashCount,
	)
	if err != nil {
		t.Fatalf(
			"inspect concurrent rotation rows: %v",
			err,
		)
	}

	if familyRowCount != 2 {
		t.Fatalf(
			"family row count = %d, want 2",
			familyRowCount,
		)
	}

	if activeFamilyRowCount != 0 {
		t.Fatalf(
			"active family row count = %d, want 0",
			activeFamilyRowCount,
		)
	}

	if successfulHashCount != 1 {
		t.Fatalf(
			"successful replacement count = %d, want 1",
			successfulHashCount,
		)
	}

	if replayedHashCount != 0 {
		t.Fatalf(
			"replayed replacement count = %d, want 0",
			replayedHashCount,
		)
	}
}
