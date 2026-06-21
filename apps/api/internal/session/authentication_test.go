package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

const (
	authenticationTestUserID = "00000000-0000-0000-0000-000000000301"

	authenticationTestSessionID = "00000000-0000-0000-0000-000000000302"
)

func TestServiceAuthenticateAccessTokenValidatesActiveSession(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		21,
		17,
		0,
		0,
		123456789,
		time.UTC,
	)

	principal := authenticationTestPrincipal()

	accessTokens :=
		&authenticationTestAccessTokenProvider{
			verifiedPrincipal: principal,
		}

	sessionStore :=
		&authenticationTestSessionStore{
			state: store.SessionState{
				UserID: principal.UserID,
				TokenFamilyID: principal.
					SessionID,
				ExpiresAt: fixedTime.Add(
					24 * time.Hour,
				),
			},
		}

	service := newAuthenticationTestService(
		sessionStore,
		accessTokens,
		fixedTime,
	)

	result, err := service.AuthenticateAccessToken(
		context.Background(),
		"synthetic-access-token",
	)
	if err != nil {
		t.Fatalf(
			"authenticate access token: %v",
			err,
		)
	}

	if result != principal {
		t.Fatalf(
			"principal = %+v, want %+v",
			result,
			principal,
		)
	}

	if accessTokens.verifyCalls != 1 {
		t.Fatalf(
			"Verify() calls = %d, want 1",
			accessTokens.verifyCalls,
		)
	}

	if accessTokens.lastTokenValue !=
		"synthetic-access-token" {
		t.Fatal(
			"access-token provider received the wrong token",
		)
	}

	if sessionStore.getCalls != 1 {
		t.Fatalf(
			"GetActiveState() calls = %d, want 1",
			sessionStore.getCalls,
		)
	}

	if sessionStore.lastUserID !=
		principal.UserID {
		t.Fatalf(
			"state user ID = %q, want %q",
			sessionStore.lastUserID,
			principal.UserID,
		)
	}

	if sessionStore.lastSessionID !=
		principal.SessionID {
		t.Fatalf(
			"state session ID = %q, want %q",
			sessionStore.lastSessionID,
			principal.SessionID,
		)
	}

	expectedNow := fixedTime.
		UTC().
		Truncate(time.Microsecond)

	if !sessionStore.lastNow.Equal(expectedNow) {
		t.Fatalf(
			"state time = %v, want %v",
			sessionStore.lastNow,
			expectedNow,
		)
	}
}

func TestServiceAuthenticateAccessTokenRejectsInvalidToken(
	t *testing.T,
) {
	accessTokens :=
		&authenticationTestAccessTokenProvider{
			verifyErr: ErrAccessTokenInvalid,
		}

	sessionStore :=
		&authenticationTestSessionStore{}

	service := newAuthenticationTestService(
		sessionStore,
		accessTokens,
		time.Now().UTC(),
	)

	_, err := service.AuthenticateAccessToken(
		context.Background(),
		"invalid-access-token",
	)

	if !errors.Is(
		err,
		ErrAccessTokenInvalid,
	) {
		t.Fatalf(
			"expected ErrAccessTokenInvalid, got %v",
			err,
		)
	}

	if sessionStore.getCalls != 0 {
		t.Fatal(
			"session store was called for an invalid token",
		)
	}
}

func TestServiceAuthenticateAccessTokenRejectsInactiveSession(
	t *testing.T,
) {
	accessTokens :=
		&authenticationTestAccessTokenProvider{
			verifiedPrincipal: authenticationTestPrincipal(),
		}

	sessionStore :=
		&authenticationTestSessionStore{
			stateErr: store.ErrNotFound,
		}

	service := newAuthenticationTestService(
		sessionStore,
		accessTokens,
		time.Now().UTC(),
	)

	_, err := service.AuthenticateAccessToken(
		context.Background(),
		"synthetic-access-token",
	)

	if !errors.Is(
		err,
		ErrAccessTokenInvalid,
	) {
		t.Fatalf(
			"expected ErrAccessTokenInvalid, got %v",
			err,
		)
	}
}

func TestServiceAuthenticateAccessTokenMapsStoreFailureSafely(
	t *testing.T,
) {
	accessTokens :=
		&authenticationTestAccessTokenProvider{
			verifiedPrincipal: authenticationTestPrincipal(),
		}

	sessionStore :=
		&authenticationTestSessionStore{
			stateErr: store.ErrDatabase,
		}

	service := newAuthenticationTestService(
		sessionStore,
		accessTokens,
		time.Now().UTC(),
	)

	_, err := service.AuthenticateAccessToken(
		context.Background(),
		"synthetic-access-token",
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

func TestServiceAuthenticateAccessTokenMapsVerifierFailureSafely(
	t *testing.T,
) {
	accessTokens :=
		&authenticationTestAccessTokenProvider{
			verifyErr: ErrAccessTokenUnavailable,
		}

	service := newAuthenticationTestService(
		&authenticationTestSessionStore{},
		accessTokens,
		time.Now().UTC(),
	)

	_, err := service.AuthenticateAccessToken(
		context.Background(),
		"synthetic-access-token",
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

func TestServiceAuthenticateAccessTokenRejectsInvalidState(
	t *testing.T,
) {
	principal := authenticationTestPrincipal()

	accessTokens :=
		&authenticationTestAccessTokenProvider{
			verifiedPrincipal: principal,
		}

	sessionStore :=
		&authenticationTestSessionStore{
			state: store.SessionState{
				UserID:        principal.UserID,
				TokenFamilyID: "00000000-0000-0000-0000-000000000399",
				ExpiresAt: time.Now().
					UTC().
					Add(24 * time.Hour),
			},
		}

	service := newAuthenticationTestService(
		sessionStore,
		accessTokens,
		time.Now().UTC(),
	)

	_, err := service.AuthenticateAccessToken(
		context.Background(),
		"synthetic-access-token",
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

func TestServiceAuthenticateAccessTokenHonorsCanceledContext(
	t *testing.T,
) {
	accessTokens :=
		&authenticationTestAccessTokenProvider{}

	sessionStore :=
		&authenticationTestSessionStore{}

	service := newAuthenticationTestService(
		sessionStore,
		accessTokens,
		time.Now().UTC(),
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := service.AuthenticateAccessToken(
		ctx,
		"synthetic-access-token",
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}

	if accessTokens.verifyCalls != 0 {
		t.Fatal(
			"access-token provider was called after cancellation",
		)
	}

	if sessionStore.getCalls != 0 {
		t.Fatal(
			"session store was called after cancellation",
		)
	}
}

func TestServiceAuthenticateAccessTokenRejectsUnavailableDependencies(
	t *testing.T,
) {
	service := NewService(
		nil,
		nil,
		nil,
		nil,
		DefaultTokenLifetimes(),
	)

	_, err := service.AuthenticateAccessToken(
		context.Background(),
		"synthetic-access-token",
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

func authenticationTestPrincipal() Principal {
	return Principal{
		UserID:    authenticationTestUserID,
		SessionID: authenticationTestSessionID,
	}
}

func newAuthenticationTestService(
	sessionStore SessionStore,
	accessTokens AccessTokenProvider,
	fixedTime time.Time,
) *Service {
	service := NewService(
		nil,
		sessionStore,
		nil,
		accessTokens,
		DefaultTokenLifetimes(),
	)

	service.now = func() time.Time {
		return fixedTime
	}

	return service
}

type authenticationTestAccessTokenProvider struct {
	verifiedPrincipal Principal
	verifyErr         error
	verifyCalls       int
	lastTokenValue    string
}

func (provider *authenticationTestAccessTokenProvider) Issue(
	_ context.Context,
	_ Principal,
) (AccessToken, error) {
	return AccessToken{}, nil
}

func (provider *authenticationTestAccessTokenProvider) Verify(
	_ context.Context,
	tokenValue string,
) (Principal, error) {
	provider.verifyCalls++
	provider.lastTokenValue = tokenValue

	if provider.verifyErr != nil {
		return Principal{}, provider.verifyErr
	}

	return provider.verifiedPrincipal, nil
}

type authenticationTestSessionStore struct {
	state    store.SessionState
	stateErr error

	getCalls      int
	lastUserID    string
	lastSessionID string
	lastNow       time.Time
}

func (sessionStore *authenticationTestSessionStore) Create(
	_ context.Context,
	_ *store.Session,
) error {
	return nil
}

func (sessionStore *authenticationTestSessionStore) GetActiveState(
	_ context.Context,
	userID string,
	sessionID string,
	now time.Time,
) (store.SessionState, error) {
	sessionStore.getCalls++
	sessionStore.lastUserID = userID
	sessionStore.lastSessionID = sessionID
	sessionStore.lastNow = now

	if sessionStore.stateErr != nil {
		return store.SessionState{},
			sessionStore.stateErr
	}

	return sessionStore.state, nil
}

func (sessionStore *authenticationTestSessionStore) ListActive(
	_ context.Context,
	_ string,
	_ time.Time,
) ([]store.SessionSummary, error) {
	return make([]store.SessionSummary, 0), nil
}

func (sessionStore *authenticationTestSessionStore) RotateRefreshToken(
	_ context.Context,
	_ []byte,
	_ []byte,
	_ time.Time,
) (store.SessionRotation, error) {
	return store.SessionRotation{}, nil
}

func (sessionStore *authenticationTestSessionStore) RevokeOwnedFamily(
	_ context.Context,
	_ string,
	_ string,
	_ time.Time,
) error {
	return nil
}

func (sessionStore *authenticationTestSessionStore) RevokeAllForUser(
	_ context.Context,
	_ string,
	_ time.Time,
) error {
	return nil
}
