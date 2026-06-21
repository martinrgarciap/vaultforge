package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

const (
	managementTestUserID = "00000000-0000-0000-0000-000000000201"

	managementTestCurrentSessionID = "00000000-0000-0000-0000-000000000202"

	managementTestOtherSessionID = "00000000-0000-0000-0000-000000000203"
)

func TestServiceListSessionsMarksCurrentSession(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		21,
		12,
		0,
		0,
		123456000,
		time.UTC,
	)

	sessionStore := &managementTestSessionStore{
		summaries: []store.SessionSummary{
			{
				TokenFamilyID: managementTestCurrentSessionID,
				UserAgent:     "Thunder Client",
				CreatedAt: fixedTime.Add(
					-time.Hour,
				),
				ExpiresAt: fixedTime.Add(
					24 * time.Hour,
				),
			},
			{
				TokenFamilyID: managementTestOtherSessionID,
				UserAgent:     "Firefox",
				CreatedAt: fixedTime.Add(
					-2 * time.Hour,
				),
				ExpiresAt: fixedTime.Add(
					48 * time.Hour,
				),
			},
		},
	}

	service := newManagementTestService(
		sessionStore,
		fixedTime,
	)

	principal := managementTestPrincipal()

	sessions, err := service.ListSessions(
		context.Background(),
		principal,
	)
	if err != nil {
		t.Fatalf(
			"list sessions: %v",
			err,
		)
	}

	if sessionStore.listCalls != 1 {
		t.Fatalf(
			"ListActive() calls = %d, want 1",
			sessionStore.listCalls,
		)
	}

	if sessionStore.listUserID !=
		managementTestUserID {
		t.Fatalf(
			"listed user ID = %q",
			sessionStore.listUserID,
		)
	}

	if !sessionStore.listNow.Equal(
		fixedTime,
	) {
		t.Fatalf(
			"list time = %v, want %v",
			sessionStore.listNow,
			fixedTime,
		)
	}

	if len(sessions) != 2 {
		t.Fatalf(
			"session count = %d, want 2",
			len(sessions),
		)
	}

	if sessions[0].ID !=
		managementTestCurrentSessionID {
		t.Fatalf(
			"first session ID = %q",
			sessions[0].ID,
		)
	}

	if !sessions[0].Current {
		t.Fatal(
			"current session was not marked current",
		)
	}

	if sessions[1].Current {
		t.Fatal(
			"other session was marked current",
		)
	}

	if sessions[1].UserAgent != "Firefox" {
		t.Fatalf(
			"other user agent = %q",
			sessions[1].UserAgent,
		)
	}
}

func TestServiceListSessionsReturnsEmptySlice(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		21,
		13,
		0,
		0,
		0,
		time.UTC,
	)

	service := newManagementTestService(
		&managementTestSessionStore{},
		fixedTime,
	)

	sessions, err := service.ListSessions(
		context.Background(),
		managementTestPrincipal(),
	)
	if err != nil {
		t.Fatalf(
			"list empty sessions: %v",
			err,
		)
	}

	if sessions == nil {
		t.Fatal(
			"expected an empty non-nil slice",
		)
	}

	if len(sessions) != 0 {
		t.Fatalf(
			"session count = %d, want 0",
			len(sessions),
		)
	}
}

func TestServiceListSessionsRejectsInvalidStoreResult(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		21,
		14,
		0,
		0,
		0,
		time.UTC,
	)

	sessionStore := &managementTestSessionStore{
		summaries: []store.SessionSummary{
			{
				ExpiresAt: fixedTime.Add(
					time.Hour,
				),
			},
		},
	}

	service := newManagementTestService(
		sessionStore,
		fixedTime,
	)

	_, err := service.ListSessions(
		context.Background(),
		managementTestPrincipal(),
	)

	if !errors.Is(
		err,
		ErrSessionUnavailable,
	) {
		t.Fatalf(
			"expected ErrSessionUnavailable, got %v",
			err,
		)
	}
}

func TestServiceListSessionsMapsStoreFailureSafely(
	t *testing.T,
) {
	sessionStore := &managementTestSessionStore{
		listErr: store.ErrDatabase,
	}

	service := newManagementTestService(
		sessionStore,
		time.Now().UTC(),
	)

	_, err := service.ListSessions(
		context.Background(),
		managementTestPrincipal(),
	)

	if !errors.Is(
		err,
		ErrSessionUnavailable,
	) {
		t.Fatalf(
			"expected ErrSessionUnavailable, got %v",
			err,
		)
	}
}

func TestServiceLogoutCurrentRevokesCurrentFamily(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		21,
		15,
		0,
		0,
		987654000,
		time.UTC,
	)

	sessionStore := &managementTestSessionStore{}

	service := newManagementTestService(
		sessionStore,
		fixedTime,
	)

	err := service.LogoutCurrent(
		context.Background(),
		managementTestPrincipal(),
	)
	if err != nil {
		t.Fatalf(
			"logout current session: %v",
			err,
		)
	}

	if sessionStore.revokeCalls != 1 {
		t.Fatalf(
			"RevokeOwnedFamily() calls = %d, want 1",
			sessionStore.revokeCalls,
		)
	}

	if sessionStore.revokedUserID !=
		managementTestUserID {
		t.Fatalf(
			"revoked user ID = %q",
			sessionStore.revokedUserID,
		)
	}

	if sessionStore.revokedSessionID !=
		managementTestCurrentSessionID {
		t.Fatalf(
			"revoked session ID = %q",
			sessionStore.revokedSessionID,
		)
	}

	if !sessionStore.revokedAt.Equal(
		fixedTime,
	) {
		t.Fatalf(
			"revocation time = %v, want %v",
			sessionStore.revokedAt,
			fixedTime,
		)
	}
}

func TestServiceLogoutCurrentIsIdempotent(
	t *testing.T,
) {
	sessionStore := &managementTestSessionStore{
		revokeErr: store.ErrNotFound,
	}

	service := newManagementTestService(
		sessionStore,
		time.Now().UTC(),
	)

	err := service.LogoutCurrent(
		context.Background(),
		managementTestPrincipal(),
	)
	if err != nil {
		t.Fatalf(
			"idempotent logout returned: %v",
			err,
		)
	}
}

func TestServiceRevokeSessionMapsNotFound(
	t *testing.T,
) {
	sessionStore := &managementTestSessionStore{
		revokeErr: store.ErrNotFound,
	}

	service := newManagementTestService(
		sessionStore,
		time.Now().UTC(),
	)

	err := service.RevokeSession(
		context.Background(),
		managementTestPrincipal(),
		managementTestOtherSessionID,
	)

	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf(
			"expected ErrSessionNotFound, got %v",
			err,
		)
	}
}

func TestServiceLogoutAllRevokesAllUserSessions(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		21,
		16,
		0,
		0,
		0,
		time.UTC,
	)

	sessionStore := &managementTestSessionStore{}

	service := newManagementTestService(
		sessionStore,
		fixedTime,
	)

	err := service.LogoutAll(
		context.Background(),
		managementTestPrincipal(),
	)
	if err != nil {
		t.Fatalf(
			"logout all sessions: %v",
			err,
		)
	}

	if sessionStore.revokeAllCalls != 1 {
		t.Fatalf(
			"RevokeAllForUser() calls = %d, want 1",
			sessionStore.revokeAllCalls,
		)
	}

	if sessionStore.revokedAllUserID !=
		managementTestUserID {
		t.Fatalf(
			"revoke-all user ID = %q",
			sessionStore.revokedAllUserID,
		)
	}

	if !sessionStore.revokedAllAt.Equal(
		fixedTime,
	) {
		t.Fatalf(
			"revoke-all time = %v, want %v",
			sessionStore.revokedAllAt,
			fixedTime,
		)
	}
}

func TestServiceManagementRejectsInvalidPrincipal(
	t *testing.T,
) {
	sessionStore := &managementTestSessionStore{}

	service := newManagementTestService(
		sessionStore,
		time.Now().UTC(),
	)

	_, err := service.ListSessions(
		context.Background(),
		Principal{},
	)

	if !errors.Is(err, ErrPrincipalInvalid) {
		t.Fatalf(
			"expected ErrPrincipalInvalid, got %v",
			err,
		)
	}

	if sessionStore.listCalls != 0 {
		t.Fatal(
			"store was called for an invalid principal",
		)
	}
}

func TestServiceManagementHonorsCanceledContext(
	t *testing.T,
) {
	sessionStore := &managementTestSessionStore{}

	service := newManagementTestService(
		sessionStore,
		time.Now().UTC(),
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := service.ListSessions(
		ctx,
		managementTestPrincipal(),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}

	if sessionStore.listCalls != 0 {
		t.Fatal(
			"store was called after cancellation",
		)
	}
}

func TestServiceManagementRejectsUnavailableStore(
	t *testing.T,
) {
	service := NewService(
		nil,
		nil,
		nil,
		nil,
		DefaultTokenLifetimes(),
	)

	_, err := service.ListSessions(
		context.Background(),
		managementTestPrincipal(),
	)

	if !errors.Is(
		err,
		ErrSessionUnavailable,
	) {
		t.Fatalf(
			"expected ErrSessionUnavailable, got %v",
			err,
		)
	}
}

func managementTestPrincipal() Principal {
	return Principal{
		UserID:    managementTestUserID,
		SessionID: managementTestCurrentSessionID,
	}
}

func newManagementTestService(
	sessionStore SessionStore,
	fixedTime time.Time,
) *Service {
	service := NewService(
		nil,
		sessionStore,
		nil,
		nil,
		DefaultTokenLifetimes(),
	)

	service.now = func() time.Time {
		return fixedTime
	}

	return service
}

type managementTestSessionStore struct {
	summaries    []store.SessionSummary
	listErr      error
	revokeErr    error
	revokeAllErr error

	listCalls      int
	revokeCalls    int
	revokeAllCalls int

	listUserID string
	listNow    time.Time

	revokedUserID    string
	revokedSessionID string
	revokedAt        time.Time

	revokedAllUserID string
	revokedAllAt     time.Time
}

func (sessionStore *managementTestSessionStore) Create(
	_ context.Context,
	_ *store.Session,
) error {
	return nil
}

func (sessionStore *managementTestSessionStore) ListActive(
	_ context.Context,
	userID string,
	now time.Time,
) ([]store.SessionSummary, error) {
	sessionStore.listCalls++
	sessionStore.listUserID = userID
	sessionStore.listNow = now

	if sessionStore.listErr != nil {
		return nil, sessionStore.listErr
	}

	return append(
		[]store.SessionSummary(nil),
		sessionStore.summaries...,
	), nil
}

func (sessionStore *managementTestSessionStore) RotateRefreshToken(
	_ context.Context,
	_ []byte,
	_ []byte,
	_ time.Time,
) (store.SessionRotation, error) {
	return store.SessionRotation{}, nil
}

func (sessionStore *managementTestSessionStore) RevokeOwnedFamily(
	_ context.Context,
	userID string,
	sessionID string,
	revokedAt time.Time,
) error {
	sessionStore.revokeCalls++
	sessionStore.revokedUserID = userID
	sessionStore.revokedSessionID = sessionID
	sessionStore.revokedAt = revokedAt

	return sessionStore.revokeErr
}

func (sessionStore *managementTestSessionStore) RevokeAllForUser(
	_ context.Context,
	userID string,
	revokedAt time.Time,
) error {
	sessionStore.revokeAllCalls++
	sessionStore.revokedAllUserID = userID
	sessionStore.revokedAllAt = revokedAt

	return sessionStore.revokeAllErr
}

func (sessionStore *managementTestSessionStore) GetActiveState(
	_ context.Context,
	_ string,
	_ string,
	_ time.Time,
) (store.SessionState, error) {
	return store.SessionState{}, nil
}
