package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

func newTestAccessTokenManager() *session.AccessTokenManager {
	seed := bytes.Repeat(
		[]byte{0x42},
		ed25519.SeedSize,
	)

	privateKey := ed25519.NewKeyFromSeed(seed)

	manager, err := session.NewAccessTokenManager(
		"vaultforge-test-issuer",
		"vaultforge-test-audience",
		"test-ed25519-v1",
		privateKey,
		session.DefaultTokenLifetimes(),
	)
	if err != nil {
		panic(
			"create test access-token manager: " +
				err.Error(),
		)
	}

	return manager
}

func newTestSessionService() *session.Service {
	return session.NewService(
		nil,
		nil,
		nil,
		newTestAccessTokenManager(),
		session.DefaultTokenLifetimes(),
	)
}

func newTestLoginSessionService(
	authenticator session.Authenticator,
) *session.Service {
	return session.NewService(
		authenticator,
		&testLoginSessionStore{},
		session.NewRefreshTokenGenerator(),
		newTestAccessTokenManager(),
		session.DefaultTokenLifetimes(),
	)
}

type testLoginSessionStore struct{}

func (
	sessionStore *testLoginSessionStore,
) Create(
	_ context.Context,
	storedSession *store.Session,
) error {
	storedSession.ID =
		"00000000-0000-0000-0000-000000000101"

	storedSession.TokenFamilyID =
		"00000000-0000-0000-0000-000000000102"

	storedSession.CreatedAt = time.Now().UTC()

	return nil
}

func (
	sessionStore *testLoginSessionStore,
) RevokeOwnedFamily(
	_ context.Context,
	_ string,
	_ string,
	_ time.Time,
) error {
	return nil
}
