package store

import (
	"context"
	"testing"
)

func newIntegrationTestVaultStores(
	t *testing.T,
) (*VaultStore, *UserStore) {
	t.Helper()

	if testDatabasePool == nil {
		t.Skip(
			"TEST_DATABASE_URL is not configured",
		)
	}

	resetIntegrationTestTables(t)

	return NewVaultStore(testDatabasePool),
		NewUserStore(testDatabasePool)
}

func createVaultTestUser(
	t *testing.T,
	userStore *UserStore,
	email string,
) *User {
	t.Helper()

	user := &User{
		Email:             email,
		PasswordHash:      "dummy-vault-test-password-hash",
		PasswordAlgorithm: "test",
	}

	err := userStore.Create(
		context.Background(),
		user,
	)
	if err != nil {
		t.Fatal(
			"create vault test user",
		)
	}

	return user
}
