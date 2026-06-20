package store

import (
	"context"
	"testing"
)

func newIntegrationTestSessionStores(
	t *testing.T,
) (*SessionStore, *UserStore) {
	t.Helper()

	if testDatabasePool == nil {
		t.Skip(
			"TEST_DATABASE_URL is not configured",
		)
	}

	resetIntegrationTestTables(t)

	return NewSessionStore(testDatabasePool),
		NewUserStore(testDatabasePool)
}

func createSessionTestUser(
	t *testing.T,
	userStore *UserStore,
	email string,
) *User {
	t.Helper()

	user := &User{
		Email:             email,
		PasswordHash:      "dummy-session-test-hash",
		PasswordAlgorithm: "test",
	}

	if err := userStore.Create(
		context.Background(),
		user,
	); err != nil {
		t.Fatalf(
			"create session test user: %v",
			err,
		)
	}

	return user
}
