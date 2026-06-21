package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const vaultDeleteTestCorrelationID = "00000000-0000-0000-0000-000000000701"

func TestVaultStoreDeleteOwnedCommitsDeletionAndAuditEvent(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-delete-owner@example.com",
	)

	createdVault := createVaultReadTestVault(
		t,
		vaultStore,
		owner.ID,
		"Delete Vault",
		"vault-delete-create",
	)

	itemID := createVaultDeleteTestItem(
		t,
		createdVault.ID,
	)

	err := vaultStore.DeleteOwned(
		context.Background(),
		vaultdomain.DeleteStoreInput{
			OwnerID:       owner.ID,
			VaultID:       createdVault.ID,
			CorrelationID: vaultDeleteTestCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf(
			"delete owned vault: %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var (
		vaultCount       int
		itemCount        int
		itemVersionCount int
		deleteEventCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT count(*)
					FROM vaults
					WHERE id = $1::uuid
				),
				(
					SELECT count(*)
					FROM vault_items
					WHERE id = $2::uuid
				),
				(
					SELECT count(*)
					FROM item_versions
					WHERE vault_item_id = $2::uuid
				),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE event_type = 'vault.deleted'
					  AND aggregate_id = $1::uuid
				)
		`,
		createdVault.ID,
		itemID,
	).Scan(
		&vaultCount,
		&itemCount,
		&itemVersionCount,
		&deleteEventCount,
	)
	if err != nil {
		t.Fatal(
			"inspect deleted vault records",
		)
	}

	if vaultCount != 0 {
		t.Fatalf(
			"vault count = %d, want 0",
			vaultCount,
		)
	}

	if itemCount != 0 {
		t.Fatalf(
			"vault item count = %d, want 0",
			itemCount,
		)
	}

	if itemVersionCount != 0 {
		t.Fatalf(
			"item version count = %d, want 0",
			itemVersionCount,
		)
	}

	if deleteEventCount != 1 {
		t.Fatalf(
			"delete event count = %d, want 1",
			deleteEventCount,
		)
	}

	var (
		eventType        string
		aggregateType    string
		aggregateID      string
		actorID          string
		correlationID    string
		sanitizedPayload string
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				event_type,
				aggregate_type,
				aggregate_id::text,
				actor_id::text,
				correlation_id,
				sanitized_payload::text
			FROM audit_outbox
			WHERE event_type = 'vault.deleted'
			  AND aggregate_id = $1::uuid
		`,
		createdVault.ID,
	).Scan(
		&eventType,
		&aggregateType,
		&aggregateID,
		&actorID,
		&correlationID,
		&sanitizedPayload,
	)
	if err != nil {
		t.Fatal(
			"read vault deletion audit event",
		)
	}

	if eventType != "vault.deleted" {
		t.Fatalf(
			"event type = %q, want vault.deleted",
			eventType,
		)
	}

	if aggregateType != "vault" {
		t.Fatalf(
			"aggregate type = %q, want vault",
			aggregateType,
		)
	}

	if aggregateID != createdVault.ID {
		t.Fatalf(
			"aggregate ID = %q, want %q",
			aggregateID,
			createdVault.ID,
		)
	}

	if actorID != owner.ID {
		t.Fatalf(
			"actor ID = %q, want %q",
			actorID,
			owner.ID,
		)
	}

	if correlationID !=
		vaultDeleteTestCorrelationID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			correlationID,
			vaultDeleteTestCorrelationID,
		)
	}

	const wantSanitizedPayload = `{"schemaVersion": 1}`

	if sanitizedPayload != wantSanitizedPayload {
		t.Fatalf(
			"sanitized payload = %q, want %q",
			sanitizedPayload,
			wantSanitizedPayload,
		)
	}

	if strings.Contains(
		sanitizedPayload,
		createdVault.Name,
	) {
		t.Fatal(
			"deletion audit payload exposed the vault name",
		)
	}
}

func TestVaultStoreDeleteOwnedEnforcesOwnership(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-delete-ownership-owner@example.com",
	)

	otherUser := createVaultTestUser(
		t,
		userStore,
		"vault-delete-ownership-other@example.com",
	)

	createdVault := createVaultReadTestVault(
		t,
		vaultStore,
		owner.ID,
		"Owner Vault",
		"vault-delete-ownership-create",
	)

	err := vaultStore.DeleteOwned(
		context.Background(),
		vaultdomain.DeleteStoreInput{
			OwnerID:       otherUser.ID,
			VaultID:       createdVault.ID,
			CorrelationID: vaultDeleteTestCorrelationID,
		},
	)
	if !errors.Is(
		err,
		vaultdomain.ErrVaultNotFound,
	) {
		t.Fatalf(
			"cross-user DeleteOwned() error = %v, want %v",
			err,
			vaultdomain.ErrVaultNotFound,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var (
		vaultCount       int
		deleteEventCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT count(*)
					FROM vaults
					WHERE id = $1::uuid
				),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE event_type = 'vault.deleted'
					  AND aggregate_id = $1::uuid
				)
		`,
		createdVault.ID,
	).Scan(
		&vaultCount,
		&deleteEventCount,
	)
	if err != nil {
		t.Fatal(
			"inspect unauthorized vault deletion",
		)
	}

	if vaultCount != 1 {
		t.Fatalf(
			"vault count = %d, want 1",
			vaultCount,
		)
	}

	if deleteEventCount != 0 {
		t.Fatalf(
			"delete event count = %d, want 0",
			deleteEventCount,
		)
	}

	err = vaultStore.DeleteOwned(
		context.Background(),
		vaultdomain.DeleteStoreInput{
			OwnerID:       owner.ID,
			VaultID:       "00000000-0000-0000-0000-000000009999",
			CorrelationID: vaultDeleteTestCorrelationID,
		},
	)
	if !errors.Is(
		err,
		vaultdomain.ErrVaultNotFound,
	) {
		t.Fatalf(
			"unknown DeleteOwned() error = %v, want %v",
			err,
			vaultdomain.ErrVaultNotFound,
		)
	}
}

func TestVaultStoreDeleteOwnedRollsBackWhenAuditInsertFails(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-delete-rollback@example.com",
	)

	createdVault := createVaultReadTestVault(
		t,
		vaultStore,
		owner.ID,
		"Rollback Vault",
		"vault-delete-rollback-create",
	)

	itemID := createVaultDeleteTestItem(
		t,
		createdVault.ID,
	)

	err := vaultStore.DeleteOwned(
		context.Background(),
		vaultdomain.DeleteStoreInput{
			OwnerID:       owner.ID,
			VaultID:       createdVault.ID,
			CorrelationID: "",
		},
	)
	if !errors.Is(err, ErrDatabase) {
		t.Fatalf(
			"DeleteOwned() error = %v, want %v",
			err,
			ErrDatabase,
		)
	}

	if strings.Contains(
		err.Error(),
		"audit_outbox_correlation_id_not_blank",
	) {
		t.Fatal(
			"delete error exposed a database constraint",
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var (
		vaultCount       int
		itemCount        int
		itemVersionCount int
		deleteEventCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT count(*)
					FROM vaults
					WHERE id = $1::uuid
				),
				(
					SELECT count(*)
					FROM vault_items
					WHERE id = $2::uuid
				),
				(
					SELECT count(*)
					FROM item_versions
					WHERE vault_item_id = $2::uuid
				),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE event_type = 'vault.deleted'
					  AND aggregate_id = $1::uuid
				)
		`,
		createdVault.ID,
		itemID,
	).Scan(
		&vaultCount,
		&itemCount,
		&itemVersionCount,
		&deleteEventCount,
	)
	if err != nil {
		t.Fatal(
			"inspect rolled-back vault deletion",
		)
	}

	if vaultCount != 1 {
		t.Fatalf(
			"vault count after rollback = %d, want 1",
			vaultCount,
		)
	}

	if itemCount != 1 {
		t.Fatalf(
			"item count after rollback = %d, want 1",
			itemCount,
		)
	}

	if itemVersionCount != 1 {
		t.Fatalf(
			"version count after rollback = %d, want 1",
			itemVersionCount,
		)
	}

	if deleteEventCount != 0 {
		t.Fatalf(
			"delete event count after rollback = %d, want 0",
			deleteEventCount,
		)
	}
}

func TestVaultStoreDeleteOwnedHonorsCanceledContext(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-delete-canceled@example.com",
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := vaultStore.DeleteOwned(
		ctx,
		vaultdomain.DeleteStoreInput{
			OwnerID:       owner.ID,
			VaultID:       "00000000-0000-0000-0000-000000009999",
			CorrelationID: vaultDeleteTestCorrelationID,
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"DeleteOwned() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}

func createVaultDeleteTestItem(
	t *testing.T,
	vaultID string,
) string {
	t.Helper()

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var itemID string

	err := testDatabasePool.QueryRow(
		queryContext,
		`
			INSERT INTO vault_items (
				vault_id,
				item_type,
				encrypted_payload,
				nonce
			)
			VALUES (
				$1::uuid,
				'secure_note',
				decode(
					'0011223344556677',
					'hex'
				),
				decode(
					'aabbccddeeff0011',
					'hex'
				)
			)
			RETURNING id::text
		`,
		vaultID,
	).Scan(&itemID)
	if err != nil {
		t.Fatal(
			"create vault deletion test item",
		)
	}

	_, err = testDatabasePool.Exec(
		queryContext,
		`
			INSERT INTO item_versions (
				vault_item_id,
				version,
				item_type,
				encrypted_payload,
				nonce
			)
			VALUES (
				$1::uuid,
				1,
				'secure_note',
				decode(
					'0011223344556677',
					'hex'
				),
				decode(
					'aabbccddeeff0011',
					'hex'
				)
			)
		`,
		itemID,
	)
	if err != nil {
		t.Fatal(
			"create vault deletion test item version",
		)
	}

	return itemID
}
