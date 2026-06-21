package store

import (
	"context"
	"testing"
)

type vaultItemIntegrationFixture struct {
	store        *VaultStore
	ownerID      string
	otherOwnerID string
	vaultID      string
	otherVaultID string
}

func newVaultItemIntegrationFixture(t *testing.T) vaultItemIntegrationFixture {
	t.Helper()

	if testDatabasePool == nil {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	resetIntegrationTestTables(t)

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var fixture vaultItemIntegrationFixture

	fixture.store = NewVaultStore(testDatabasePool)

	err := testDatabasePool.QueryRow(
		queryContext,
		`
			INSERT INTO users (
				email,
				password_hash,
				password_algorithm
			)
			VALUES (
				'vault-item-owner@example.com',
				'dummy-vault-item-owner-hash',
				'test'
			)
			RETURNING id::text
		`,
	).Scan(&fixture.ownerID)
	if err != nil {
		t.Fatalf("create vault item owner: %v", err)
	}

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			INSERT INTO users (
				email,
				password_hash,
				password_algorithm
			)
			VALUES (
				'vault-item-other@example.com',
				'dummy-vault-item-other-hash',
				'test'
			)
			RETURNING id::text
		`,
	).Scan(&fixture.otherOwnerID)
	if err != nil {
		t.Fatalf("create other vault item owner: %v", err)
	}

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			INSERT INTO vaults (
				owner_id,
				name
			)
			VALUES (
				$1::uuid,
				'Vault Item Test Vault'
			)
			RETURNING id::text
		`,
		fixture.ownerID,
	).Scan(&fixture.vaultID)
	if err != nil {
		t.Fatalf("create vault item test vault: %v", err)
	}

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			INSERT INTO vaults (
				owner_id,
				name
			)
			VALUES (
				$1::uuid,
				'Other Vault Item Test Vault'
			)
			RETURNING id::text
		`,
		fixture.otherOwnerID,
	).Scan(&fixture.otherVaultID)
	if err != nil {
		t.Fatalf("create other vault item test vault: %v", err)
	}

	return fixture
}
