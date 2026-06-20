package session

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

func TestServiceRefreshRotatesTokenAndIssuesAccessToken(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		20,
		22,
		0,
		0,
		0,
		time.UTC,
	)

	currentRefreshToken :=
		newLoginTestRefreshToken(t, 0x81)

	replacementRefreshToken :=
		newLoginTestRefreshToken(t, 0x82)

	currentDigest, err :=
		currentRefreshToken.Digest()
	if err != nil {
		t.Fatalf(
			"digest current refresh token: %v",
			err,
		)
	}

	replacementDigest, err :=
		replacementRefreshToken.Digest()
	if err != nil {
		t.Fatalf(
			"digest replacement refresh token: %v",
			err,
		)
	}

	refreshExpiresAt := fixedTime.Add(
		24 * time.Hour,
	)

	sessionStore := &refreshTestSessionStore{
		rotation: store.SessionRotation{
			ID:            "row-456",
			UserID:        "user-123",
			TokenFamilyID: "family-789",
			CreatedAt: fixedTime.Add(
				time.Second,
			),
			ExpiresAt: refreshExpiresAt,
		},
	}

	refreshTokens :=
		&loginTestRefreshTokenProvider{
			token: replacementRefreshToken,
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
		nil,
		sessionStore,
		refreshTokens,
		accessTokens,
		DefaultTokenLifetimes(),
	)

	service.now = func() time.Time {
		return fixedTime
	}

	result, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: currentRefreshToken.Value(),
		},
	)
	if err != nil {
		t.Fatalf(
			"refresh session: %v",
			err,
		)
	}

	if refreshTokens.calls != 1 {
		t.Fatalf(
			"refresh-token generator calls = %d, want 1",
			refreshTokens.calls,
		)
	}

	if sessionStore.rotateCalls != 1 {
		t.Fatalf(
			"RotateRefreshToken() calls = %d, want 1",
			sessionStore.rotateCalls,
		)
	}

	if !bytes.Equal(
		sessionStore.currentRefreshTokenHash,
		currentDigest.Bytes(),
	) {
		t.Fatal(
			"current refresh-token digest did not match",
		)
	}

	if !bytes.Equal(
		sessionStore.replacementRefreshTokenHash,
		replacementDigest.Bytes(),
	) {
		t.Fatal(
			"replacement refresh-token digest did not match",
		)
	}

	if !sessionStore.rotatedAt.Equal(fixedTime) {
		t.Fatalf(
			"rotation time = %v, want %v",
			sessionStore.rotatedAt,
			fixedTime,
		)
	}

	expectedPrincipal := Principal{
		UserID:    "user-123",
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

	if result.AccessToken.Value() !=
		"synthetic-access-token" {
		t.Fatal(
			"returned access token did not match issued token",
		)
	}

	if result.RefreshToken.Value() !=
		replacementRefreshToken.Value() {
		t.Fatal(
			"returned refresh token did not match replacement token",
		)
	}

	if !result.RefreshTokenExpiresAt.Equal(
		refreshExpiresAt,
	) {
		t.Fatalf(
			"refresh expiration = %v, want %v",
			result.RefreshTokenExpiresAt,
			refreshExpiresAt,
		)
	}

	if sessionStore.revokeCalls != 0 {
		t.Fatalf(
			"unexpected family revocations = %d",
			sessionStore.revokeCalls,
		)
	}
}

func TestServiceRefreshRejectsMalformedToken(
	t *testing.T,
) {
	sessionStore := &refreshTestSessionStore{}

	refreshTokens :=
		&loginTestRefreshTokenProvider{}

	accessTokens :=
		&loginTestAccessTokenProvider{}

	service := NewService(
		nil,
		sessionStore,
		refreshTokens,
		accessTokens,
		DefaultTokenLifetimes(),
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: "malformed-refresh-token",
		},
	)

	if !errors.Is(
		err,
		ErrRefreshTokenInvalid,
	) {
		t.Fatalf(
			"expected ErrRefreshTokenInvalid, got %v",
			err,
		)
	}

	if refreshTokens.calls != 0 {
		t.Fatalf(
			"refresh-token generator calls = %d, want 0",
			refreshTokens.calls,
		)
	}

	if sessionStore.rotateCalls != 0 {
		t.Fatalf(
			"rotation calls = %d, want 0",
			sessionStore.rotateCalls,
		)
	}

	if accessTokens.issueCalls != 0 {
		t.Fatalf(
			"access-token issuance calls = %d, want 0",
			accessTokens.issueCalls,
		)
	}
}

func TestServiceRefreshMapsInvalidStoredTokensGenerically(
	t *testing.T,
) {
	tests := []struct {
		name        string
		rotationErr error
	}{
		{
			name:        "unknown or expired token",
			rotationErr: store.ErrNotFound,
		},
		{
			name: "replayed token",
			rotationErr: store.
				ErrSessionReplayDetected,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			currentRefreshToken :=
				newLoginTestRefreshToken(
					t,
					0x83,
				)

			replacementRefreshToken :=
				newLoginTestRefreshToken(
					t,
					0x84,
				)

			sessionStore :=
				&refreshTestSessionStore{
					rotateErr: test.rotationErr,
				}

			refreshTokens :=
				&loginTestRefreshTokenProvider{
					token: replacementRefreshToken,
				}

			accessTokens :=
				&loginTestAccessTokenProvider{}

			service := NewService(
				nil,
				sessionStore,
				refreshTokens,
				accessTokens,
				DefaultTokenLifetimes(),
			)

			_, err := service.Refresh(
				context.Background(),
				RefreshInput{
					RefreshToken: currentRefreshToken.
						Value(),
				},
			)

			if !errors.Is(
				err,
				ErrRefreshTokenInvalid,
			) {
				t.Fatalf(
					"expected ErrRefreshTokenInvalid, got %v",
					err,
				)
			}

			if accessTokens.issueCalls != 0 {
				t.Fatalf(
					"access-token issuance calls = %d, want 0",
					accessTokens.issueCalls,
				)
			}
		})
	}
}

func TestServiceRefreshMapsStoreFailureSafely(
	t *testing.T,
) {
	currentRefreshToken :=
		newLoginTestRefreshToken(t, 0x85)

	replacementRefreshToken :=
		newLoginTestRefreshToken(t, 0x86)

	sessionStore := &refreshTestSessionStore{
		rotateErr: store.ErrDatabase,
	}

	refreshTokens :=
		&loginTestRefreshTokenProvider{
			token: replacementRefreshToken,
		}

	accessTokens :=
		&loginTestAccessTokenProvider{}

	service := NewService(
		nil,
		sessionStore,
		refreshTokens,
		accessTokens,
		DefaultTokenLifetimes(),
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: currentRefreshToken.Value(),
		},
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
		t.Fatalf(
			"access-token issuance calls = %d, want 0",
			accessTokens.issueCalls,
		)
	}
}

func TestServiceRefreshRevokesFamilyWhenAccessTokenIssuanceFails(
	t *testing.T,
) {
	fixedTime := time.Date(
		2026,
		time.June,
		20,
		23,
		0,
		0,
		0,
		time.UTC,
	)

	replacementCreatedAt := fixedTime.Add(
		time.Second,
	)

	currentRefreshToken :=
		newLoginTestRefreshToken(t, 0x87)

	replacementRefreshToken :=
		newLoginTestRefreshToken(t, 0x88)

	sessionStore := &refreshTestSessionStore{
		rotation: store.SessionRotation{
			ID:            "row-456",
			UserID:        "user-123",
			TokenFamilyID: "family-789",
			CreatedAt:     replacementCreatedAt,
			ExpiresAt: fixedTime.Add(
				24 * time.Hour,
			),
		},
	}

	refreshTokens :=
		&loginTestRefreshTokenProvider{
			token: replacementRefreshToken,
		}

	accessTokens :=
		&loginTestAccessTokenProvider{
			issueErr: errors.New(
				"synthetic signing failure",
			),
		}

	service := NewService(
		nil,
		sessionStore,
		refreshTokens,
		accessTokens,
		DefaultTokenLifetimes(),
	)

	service.now = func() time.Time {
		return fixedTime
	}

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: currentRefreshToken.Value(),
		},
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

	if !sessionStore.revokedAt.Equal(
		replacementCreatedAt,
	) {
		t.Fatalf(
			"revocation time = %v, want %v",
			sessionStore.revokedAt,
			replacementCreatedAt,
		)
	}
}

func TestServiceRefreshRejectsUnavailableDependencies(
	t *testing.T,
) {
	service := NewService(
		nil,
		nil,
		nil,
		nil,
		DefaultTokenLifetimes(),
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{},
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

type refreshTestSessionStore struct {
	rotation  store.SessionRotation
	rotateErr error
	revokeErr error

	rotateCalls int
	revokeCalls int

	currentRefreshTokenHash     []byte
	replacementRefreshTokenHash []byte
	rotatedAt                   time.Time

	revokedUserID   string
	revokedFamilyID string
	revokedAt       time.Time
}

func (
	sessionStore *refreshTestSessionStore,
) Create(
	_ context.Context,
	_ *store.Session,
) error {
	return nil
}

func (
	sessionStore *refreshTestSessionStore,
) RotateRefreshToken(
	_ context.Context,
	currentRefreshTokenHash []byte,
	replacementRefreshTokenHash []byte,
	rotatedAt time.Time,
) (store.SessionRotation, error) {
	sessionStore.rotateCalls++

	sessionStore.currentRefreshTokenHash =
		append(
			[]byte(nil),
			currentRefreshTokenHash...,
		)

	sessionStore.replacementRefreshTokenHash =
		append(
			[]byte(nil),
			replacementRefreshTokenHash...,
		)

	sessionStore.rotatedAt = rotatedAt

	if sessionStore.rotateErr != nil {
		return store.SessionRotation{},
			sessionStore.rotateErr
	}

	return sessionStore.rotation, nil
}

func (
	sessionStore *refreshTestSessionStore,
) RevokeOwnedFamily(
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
