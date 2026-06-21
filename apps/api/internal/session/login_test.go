package session

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

func TestServiceLoginCreatesSessionAndTokens(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		20,
		20,
		0,
		0,
		0,
		time.UTC,
	)

	lifetimes, err := NewTokenLifetimes(
		10*time.Minute,
		24*time.Hour,
		30*time.Second,
	)
	if err != nil {
		t.Fatalf(
			"create token lifetimes: %v",
			err,
		)
	}

	account := auth.Account{
		ID:     "user-123",
		Email:  "martin@example.com",
		Status: "active",
	}

	refreshToken :=
		newLoginTestRefreshToken(t, 0x71)

	expectedDigest, err :=
		refreshToken.Digest()
	if err != nil {
		t.Fatalf(
			"digest refresh token: %v",
			err,
		)
	}

	authenticator := &loginTestAuthenticator{
		account: account,
	}

	sessionStore := &loginTestSessionStore{
		createdID:       "row-456",
		createdFamilyID: "family-789",
		createdAt:       fixedTime,
	}

	refreshTokens :=
		&loginTestRefreshTokenProvider{
			token: refreshToken,
		}

	accessTokens :=
		&loginTestAccessTokenProvider{
			issuedToken: AccessToken{
				value: "synthetic-access-token",
				expiresAt: fixedTime.Add(
					10 * time.Minute,
				),
			},
		}

	service := NewService(
		authenticator,
		sessionStore,
		refreshTokens,
		accessTokens,
		lifetimes,
	)

	service.now = func() time.Time {
		return fixedTime
	}

	result, err := service.Login(
		context.Background(),
		LoginInput{
			Email:     "martin@example.com",
			Password:  "correct horse battery staple",
			UserAgent: "  Thunder Client  ",
		},
	)
	if err != nil {
		t.Fatalf(
			"create login session: %v",
			err,
		)
	}

	if result.Account != account {
		t.Fatalf(
			"account = %+v, want %+v",
			result.Account,
			account,
		)
	}

	if result.AccessToken.Value() !=
		"synthetic-access-token" {
		t.Fatalf(
			"access token = %q",
			result.AccessToken.Value(),
		)
	}

	if result.RefreshToken.Value() !=
		refreshToken.Value() {
		t.Fatal(
			"returned refresh token did not match generated token",
		)
	}

	expectedRefreshExpiry := fixedTime.Add(
		24 * time.Hour,
	)

	if !result.RefreshTokenExpiresAt.Equal(
		expectedRefreshExpiry,
	) {
		t.Fatalf(
			"refresh expiry = %v, want %v",
			result.RefreshTokenExpiresAt,
			expectedRefreshExpiry,
		)
	}

	if authenticator.calls != 1 {
		t.Fatalf(
			"authenticator calls = %d, want 1",
			authenticator.calls,
		)
	}

	if refreshTokens.calls != 1 {
		t.Fatalf(
			"refresh generator calls = %d, want 1",
			refreshTokens.calls,
		)
	}

	if sessionStore.createCalls != 1 {
		t.Fatalf(
			"session Create() calls = %d, want 1",
			sessionStore.createCalls,
		)
	}

	if !bytes.Equal(
		sessionStore.createdSession.
			RefreshTokenHash,
		expectedDigest.Bytes(),
	) {
		t.Fatal(
			"stored refresh-token digest did not match",
		)
	}

	if !sessionStore.createdSession.
		ExpiresAt.Equal(expectedRefreshExpiry) {
		t.Fatalf(
			"stored expiry = %v, want %v",
			sessionStore.createdSession.ExpiresAt,
			expectedRefreshExpiry,
		)
	}

	if sessionStore.createdSession.UserAgent ==
		nil ||
		*sessionStore.createdSession.UserAgent !=
			"Thunder Client" {
		t.Fatalf(
			"stored user agent = %v",
			sessionStore.createdSession.UserAgent,
		)
	}

	if accessTokens.issueCalls != 1 {
		t.Fatalf(
			"access-token Issue() calls = %d, want 1",
			accessTokens.issueCalls,
		)
	}

	expectedPrincipal := Principal{
		UserID:    account.ID,
		SessionID: "family-789",
	}

	if accessTokens.lastPrincipal !=
		expectedPrincipal {
		t.Fatalf(
			"issued principal = %+v, want %+v",
			accessTokens.lastPrincipal,
			expectedPrincipal,
		)
	}

	if sessionStore.revokeCalls != 0 {
		t.Fatalf(
			"unexpected revocation calls = %d",
			sessionStore.revokeCalls,
		)
	}
}

func TestServiceLoginStopsAfterInvalidCredentials(
	t *testing.T,
) {
	authenticator := &loginTestAuthenticator{
		err: auth.ErrInvalidCredentials,
	}

	sessionStore :=
		&loginTestSessionStore{}

	refreshTokens :=
		&loginTestRefreshTokenProvider{}

	accessTokens :=
		&loginTestAccessTokenProvider{}

	service := NewService(
		authenticator,
		sessionStore,
		refreshTokens,
		accessTokens,
		DefaultTokenLifetimes(),
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "unknown@example.com",
			Password: "incorrect password",
		},
	)

	if !errors.Is(
		err,
		auth.ErrInvalidCredentials,
	) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}

	if refreshTokens.calls != 0 ||
		sessionStore.createCalls != 0 ||
		accessTokens.issueCalls != 0 {
		t.Fatal(
			"login continued after invalid credentials",
		)
	}
}

func TestServiceLoginMapsSessionCreationFailure(
	t *testing.T,
) {
	authenticator := &loginTestAuthenticator{
		account: auth.Account{
			ID:     "user-123",
			Status: "active",
		},
	}

	sessionStore :=
		&loginTestSessionStore{
			createErr: errors.New(
				"synthetic database detail",
			),
		}

	refreshTokens :=
		&loginTestRefreshTokenProvider{
			token: newLoginTestRefreshToken(
				t,
				0x72,
			),
		}

	accessTokens :=
		&loginTestAccessTokenProvider{}

	service := NewService(
		authenticator,
		sessionStore,
		refreshTokens,
		accessTokens,
		DefaultTokenLifetimes(),
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{},
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

	if accessTokens.issueCalls != 0 {
		t.Fatal(
			"access token was issued after session creation failed",
		)
	}
}

func TestServiceLoginRevokesSessionWhenAccessTokenIssuanceFails(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		20,
		21,
		0,
		0,
		0,
		time.UTC,
	)

	authenticator := &loginTestAuthenticator{
		account: auth.Account{
			ID:     "user-123",
			Status: "active",
		},
	}

	sessionStore := &loginTestSessionStore{
		createdID:       "row-456",
		createdFamilyID: "family-789",
		createdAt: fixedTime.Add(
			time.Second,
		),
	}

	refreshTokens :=
		&loginTestRefreshTokenProvider{
			token: newLoginTestRefreshToken(
				t,
				0x73,
			),
		}

	accessTokens :=
		&loginTestAccessTokenProvider{
			issueErr: errors.New(
				"synthetic signing detail",
			),
		}

	service := NewService(
		authenticator,
		sessionStore,
		refreshTokens,
		accessTokens,
		DefaultTokenLifetimes(),
	)

	service.now = func() time.Time {
		return fixedTime
	}

	_, err := service.Login(
		context.Background(),
		LoginInput{},
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

	if sessionStore.revokeCalls != 1 {
		t.Fatalf(
			"RevokeOwnedFamily() calls = %d, want 1",
			sessionStore.revokeCalls,
		)
	}

	if sessionStore.revokedUserID !=
		"user-123" {
		t.Fatalf(
			"revoked user ID = %q",
			sessionStore.revokedUserID,
		)
	}

	if sessionStore.revokedFamilyID !=
		"family-789" {
		t.Fatalf(
			"revoked family ID = %q",
			sessionStore.revokedFamilyID,
		)
	}

	expectedRevokedAt := fixedTime.Add(time.Second)

	if !sessionStore.revokedAt.Equal(expectedRevokedAt) {
		t.Fatalf(
			"revocation time = %v, want %v",
			sessionStore.revokedAt,
			expectedRevokedAt,
		)
	}
}

func TestServiceLoginRejectsUnavailableDependencies(
	t *testing.T,
) {
	service := NewService(
		nil,
		nil,
		nil,
		nil,
		DefaultTokenLifetimes(),
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{},
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

func TestServiceLoginHonorsCanceledContext(
	t *testing.T,
) {
	authenticator := &loginTestAuthenticator{}
	sessionStore := &loginTestSessionStore{}
	refreshTokens :=
		&loginTestRefreshTokenProvider{}
	accessTokens :=
		&loginTestAccessTokenProvider{}

	service := NewService(
		authenticator,
		sessionStore,
		refreshTokens,
		accessTokens,
		DefaultTokenLifetimes(),
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := service.Login(
		ctx,
		LoginInput{},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}

	if authenticator.calls != 0 {
		t.Fatalf(
			"authenticator calls = %d, want 0",
			authenticator.calls,
		)
	}
}

func newLoginTestRefreshToken(
	t *testing.T,
	fill byte,
) RefreshToken {
	t.Helper()

	encoded := base64.RawURLEncoding.
		EncodeToString(
			bytes.Repeat(
				[]byte{fill},
				refreshTokenEntropyBytes,
			),
		)

	token, err := ParseRefreshToken(
		refreshTokenPrefix + encoded,
	)
	if err != nil {
		t.Fatalf(
			"create test refresh token: %v",
			err,
		)
	}

	return token
}

type loginTestAuthenticator struct {
	account auth.Account
	err     error
	calls   int
	input   auth.LoginInput
}

func (authenticator *loginTestAuthenticator) Login(
	_ context.Context,
	input auth.LoginInput,
) (auth.Account, error) {
	authenticator.calls++
	authenticator.input = input

	if authenticator.err != nil {
		return auth.Account{},
			authenticator.err
	}

	return authenticator.account, nil
}

type loginTestRefreshTokenProvider struct {
	token RefreshToken
	err   error
	calls int
}

func (
	provider *loginTestRefreshTokenProvider,
) Generate(
	_ context.Context,
) (RefreshToken, error) {
	provider.calls++

	if provider.err != nil {
		return RefreshToken{},
			provider.err
	}

	return provider.token, nil
}

type loginTestAccessTokenProvider struct {
	issuedToken AccessToken
	issueErr    error

	issueCalls    int
	verifyCalls   int
	lastPrincipal Principal
}

func (
	provider *loginTestAccessTokenProvider,
) Issue(
	_ context.Context,
	principal Principal,
) (AccessToken, error) {
	provider.issueCalls++
	provider.lastPrincipal = principal

	if provider.issueErr != nil {
		return AccessToken{},
			provider.issueErr
	}

	return provider.issuedToken, nil
}

func (provider *loginTestAccessTokenProvider) Verify(
	_ context.Context,
	_ string,
) (Principal, error) {
	provider.verifyCalls++

	return Principal{}, nil
}

type loginTestSessionStore struct {
	createErr error
	revokeErr error

	createdID       string
	createdFamilyID string
	createdAt       time.Time

	createCalls int
	revokeCalls int

	createdSession  store.Session
	revokedUserID   string
	revokedFamilyID string
	revokedAt       time.Time
}

func (sessionStore *loginTestSessionStore) Create(
	_ context.Context,
	session *store.Session,
) error {
	sessionStore.createCalls++

	if sessionStore.createErr != nil {
		return sessionStore.createErr
	}

	session.ID = sessionStore.createdID
	session.TokenFamilyID =
		sessionStore.createdFamilyID
	session.CreatedAt =
		sessionStore.createdAt

	sessionStore.createdSession = *session
	sessionStore.createdSession.
		RefreshTokenHash = append(
		[]byte(nil),
		session.RefreshTokenHash...,
	)

	return nil
}

func (sessionStore *loginTestSessionStore) RotateRefreshToken(
	_ context.Context,
	_ []byte,
	_ []byte,
	_ time.Time,
) (store.SessionRotation, error) {
	return store.SessionRotation{}, nil
}

func (sessionStore *loginTestSessionStore) RevokeOwnedFamily(
	_ context.Context,
	userID string,
	tokenFamilyID string,
	revokedAt time.Time,
) error {
	sessionStore.revokeCalls++
	sessionStore.revokedUserID = userID
	sessionStore.revokedFamilyID =
		tokenFamilyID
	sessionStore.revokedAt = revokedAt

	return sessionStore.revokeErr
}

func (sessionStore *loginTestSessionStore) ListActive(
	_ context.Context,
	_ string,
	_ time.Time,
) ([]store.SessionSummary, error) {
	return make([]store.SessionSummary, 0), nil
}

func (sessionStore *loginTestSessionStore) RevokeAllForUser(
	_ context.Context,
	_ string,
	_ time.Time,
) error {
	return nil
}

func (sessionStore *loginTestSessionStore) GetActiveState(
	_ context.Context,
	_ string,
	_ string,
	_ time.Time,
) (store.SessionState, error) {
	return store.SessionState{}, nil
}
