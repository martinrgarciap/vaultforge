package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceIssueAccessToken(
	t *testing.T,
) {
	expectedPrincipal := Principal{
		UserID:    "user-123",
		SessionID: "session-456",
	}

	expectedToken := AccessToken{
		value: "synthetic-access-token",
		expiresAt: time.Date(
			2026,
			time.June,
			20,
			18,
			0,
			0,
			0,
			time.UTC,
		),
	}

	provider := &serviceTestAccessTokenProvider{
		issuedToken: expectedToken,
	}

	service := newTokenOnlyService(provider)

	token, err := service.IssueAccessToken(
		context.Background(),
		expectedPrincipal,
	)
	if err != nil {
		t.Fatalf(
			"issue access token: %v",
			err,
		)
	}

	if provider.issueCalls != 1 {
		t.Fatalf(
			"Issue() calls = %d, want 1",
			provider.issueCalls,
		)
	}

	if provider.lastPrincipal !=
		expectedPrincipal {
		t.Fatalf(
			"Issue() principal = %+v, want %+v",
			provider.lastPrincipal,
			expectedPrincipal,
		)
	}

	if token.Value() != expectedToken.Value() {
		t.Fatalf(
			"token value = %q, want %q",
			token.Value(),
			expectedToken.Value(),
		)
	}

	if !token.ExpiresAt().Equal(
		expectedToken.ExpiresAt(),
	) {
		t.Fatalf(
			"token expiry = %v, want %v",
			token.ExpiresAt(),
			expectedToken.ExpiresAt(),
		)
	}
}

func TestServiceIssueAccessTokenPreservesError(
	t *testing.T,
) {
	expectedError := errors.New(
		"synthetic token issuance error",
	)

	service := newTokenOnlyService(
		&serviceTestAccessTokenProvider{
			issueErr: expectedError,
		},
	)

	_, err := service.IssueAccessToken(
		context.Background(),
		Principal{
			UserID:    "user-123",
			SessionID: "session-456",
		},
	)

	if !errors.Is(err, expectedError) {
		t.Fatalf(
			"expected issuance error, got %v",
			err,
		)
	}
}

func TestServiceVerifyAccessToken(
	t *testing.T,
) {
	expectedPrincipal := Principal{
		UserID:    "user-123",
		SessionID: "session-456",
	}

	provider := &serviceTestAccessTokenProvider{
		verifiedPrincipal: expectedPrincipal,
	}

	service := newTokenOnlyService(provider)

	principal, err := service.VerifyAccessToken(
		context.Background(),
		"synthetic-access-token",
	)
	if err != nil {
		t.Fatalf(
			"verify access token: %v",
			err,
		)
	}

	if provider.verifyCalls != 1 {
		t.Fatalf(
			"Verify() calls = %d, want 1",
			provider.verifyCalls,
		)
	}

	if provider.lastTokenValue !=
		"synthetic-access-token" {
		t.Fatalf(
			"Verify() token = %q",
			provider.lastTokenValue,
		)
	}

	if principal != expectedPrincipal {
		t.Fatalf(
			"verified principal = %+v, want %+v",
			principal,
			expectedPrincipal,
		)
	}
}

func TestServiceVerifyAccessTokenPreservesError(
	t *testing.T,
) {
	expectedError := errors.New(
		"synthetic token verification error",
	)

	service := newTokenOnlyService(
		&serviceTestAccessTokenProvider{
			verifyErr: expectedError,
		},
	)

	_, err := service.VerifyAccessToken(
		context.Background(),
		"synthetic-access-token",
	)

	if !errors.Is(err, expectedError) {
		t.Fatalf(
			"expected verification error, got %v",
			err,
		)
	}
}

func TestServiceRejectsUnavailableAccessTokenProvider(
	t *testing.T,
) {
	service := newTokenOnlyService(nil)

	_, err := service.IssueAccessToken(
		context.Background(),
		Principal{
			UserID:    "user-123",
			SessionID: "session-456",
		},
	)

	if !errors.Is(
		err,
		ErrAccessTokenUnavailable,
	) {
		t.Fatalf(
			"expected ErrAccessTokenUnavailable, got %v",
			err,
		)
	}

	_, err = service.VerifyAccessToken(
		context.Background(),
		"synthetic-access-token",
	)

	if !errors.Is(
		err,
		ErrAccessTokenUnavailable,
	) {
		t.Fatalf(
			"expected ErrAccessTokenUnavailable, got %v",
			err,
		)
	}
}

func TestServiceRejectsNilReceiver(
	t *testing.T,
) {
	var service *Service

	_, err := service.IssueAccessToken(
		context.Background(),
		Principal{
			UserID:    "user-123",
			SessionID: "session-456",
		},
	)

	if !errors.Is(
		err,
		ErrAccessTokenUnavailable,
	) {
		t.Fatalf(
			"expected ErrAccessTokenUnavailable, got %v",
			err,
		)
	}

	_, err = service.VerifyAccessToken(
		context.Background(),
		"synthetic-access-token",
	)

	if !errors.Is(
		err,
		ErrAccessTokenUnavailable,
	) {
		t.Fatalf(
			"expected ErrAccessTokenUnavailable, got %v",
			err,
		)
	}
}

func TestServiceHonorsCanceledContext(
	t *testing.T,
) {
	provider := &serviceTestAccessTokenProvider{}
	service := newTokenOnlyService(provider)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := service.IssueAccessToken(
		ctx,
		Principal{
			UserID:    "user-123",
			SessionID: "session-456",
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}

	_, err = service.VerifyAccessToken(
		ctx,
		"synthetic-access-token",
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}

	if provider.issueCalls != 0 {
		t.Fatalf(
			"Issue() calls = %d, want 0",
			provider.issueCalls,
		)
	}

	if provider.verifyCalls != 0 {
		t.Fatalf(
			"Verify() calls = %d, want 0",
			provider.verifyCalls,
		)
	}
}

type serviceTestAccessTokenProvider struct {
	issuedToken       AccessToken
	issueErr          error
	verifiedPrincipal Principal
	verifyErr         error

	issueCalls  int
	verifyCalls int

	lastPrincipal  Principal
	lastTokenValue string
}

func (
	provider *serviceTestAccessTokenProvider,
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

func (
	provider *serviceTestAccessTokenProvider,
) Verify(
	_ context.Context,
	tokenValue string,
) (Principal, error) {
	provider.verifyCalls++
	provider.lastTokenValue = tokenValue

	if provider.verifyErr != nil {
		return Principal{},
			provider.verifyErr
	}

	return provider.verifiedPrincipal, nil
}

func newTokenOnlyService(
	provider AccessTokenProvider,
) *Service {
	return NewService(
		nil,
		nil,
		nil,
		provider,
		DefaultTokenLifetimes(),
	)
}
