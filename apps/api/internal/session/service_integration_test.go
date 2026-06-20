package session

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"testing"
)

func TestServiceIssuesAndVerifiesAccessTokenWithRealManager(
	t *testing.T,
) {
	seed := bytes.Repeat(
		[]byte{0x64},
		ed25519.SeedSize,
	)

	tokenConfig, err := NewTokenConfig(
		"vaultforge-service-integration",
		"vaultforge-service-integration",
		"service-integration-ed25519-v1",
		seed,
		DefaultTokenLifetimes(),
	)
	if err != nil {
		t.Fatalf(
			"create token configuration: %v",
			err,
		)
	}

	accessTokenManager, err :=
		tokenConfig.NewAccessTokenManager()
	if err != nil {
		t.Fatalf(
			"create access-token manager: %v",
			err,
		)
	}

	service := NewService(
		nil,
		nil,
		nil,
		accessTokenManager,
		DefaultTokenLifetimes(),
	)

	expectedPrincipal := Principal{
		UserID:    "user-123",
		SessionID: "session-family-456",
	}

	accessToken, err := service.IssueAccessToken(
		context.Background(),
		expectedPrincipal,
	)
	if err != nil {
		t.Fatalf(
			"issue access token: %v",
			err,
		)
	}

	if accessToken.Value() == "" {
		t.Fatal(
			"expected a non-empty access token",
		)
	}

	if accessToken.ExpiresAt().IsZero() {
		t.Fatal(
			"expected an access-token expiration",
		)
	}

	verifiedPrincipal, err :=
		service.VerifyAccessToken(
			context.Background(),
			accessToken.Value(),
		)
	if err != nil {
		t.Fatalf(
			"verify access token: %v",
			err,
		)
	}

	if verifiedPrincipal != expectedPrincipal {
		t.Fatalf(
			"verified principal = %+v, want %+v",
			verifiedPrincipal,
			expectedPrincipal,
		)
	}
}
