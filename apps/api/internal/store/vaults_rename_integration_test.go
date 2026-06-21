package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const vaultRenameTestCorrelationID = "00000000-0000-0000-0000-000000000601"

func TestVaultStoreRenameOwnedCommitsVaultAndAuditEvent(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-rename-owner@example.com",
	)

	createdVault := createVaultReadTestVault(
		t,
		vaultStore,
		owner.ID,
		"Original Vault",
		"vault-rename-create",
	)

	originalUpdatedAt := time.Date(
		2026,
		time.June,
		21,
		9,
		0,
		0,
		0,
		time.UTC,
	)

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	_, err := testDatabasePool.Exec(
		queryContext,
		`
			UPDATE vaults
			SET updated_at = $2
			WHERE id = $1::uuid
		`,
		createdVault.ID,
		originalUpdatedAt,
	)
	if err != nil {
		t.Fatal(
			"set original vault timestamp",
		)
	}

	renamedVault, err := vaultStore.RenameOwned(
		context.Background(),
		vaultdomain.RenameStoreInput{
			OwnerID:       owner.ID,
			VaultID:       createdVault.ID,
			Name:          "Renamed Vault",
			CorrelationID: vaultRenameTestCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf(
			"rename owned vault: %v",
			err,
		)
	}

	if renamedVault.ID != createdVault.ID {
		t.Fatalf(
			"renamed vault ID = %q, want %q",
			renamedVault.ID,
			createdVault.ID,
		)
	}

	if renamedVault.OwnerID != owner.ID {
		t.Fatalf(
			"renamed owner ID = %q, want %q",
			renamedVault.OwnerID,
			owner.ID,
		)
	}

	if renamedVault.Name != "Renamed Vault" {
		t.Fatalf(
			"renamed vault name = %q, want Renamed Vault",
			renamedVault.Name,
		)
	}

	if !renamedVault.UpdatedAt.After(
		originalUpdatedAt,
	) {
		t.Fatalf(
			"renamed update time = %v, want after %v",
			renamedVault.UpdatedAt,
			originalUpdatedAt,
		)
	}

	var (
		storedName      string
		storedUpdatedAt time.Time
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				name,
				updated_at
			FROM vaults
			WHERE id = $1::uuid
		`,
		createdVault.ID,
	).Scan(
		&storedName,
		&storedUpdatedAt,
	)
	if err != nil {
		t.Fatal(
			"read renamed vault",
		)
	}

	if storedName != "Renamed Vault" {
		t.Fatalf(
			"stored name = %q, want Renamed Vault",
			storedName,
		)
	}

	if !storedUpdatedAt.Equal(
		renamedVault.UpdatedAt,
	) {
		t.Fatal(
			"returned update time did not match PostgreSQL",
		)
	}

	var (
		eventType        string
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
				aggregate_id::text,
				actor_id::text,
				correlation_id,
				sanitized_payload::text
			FROM audit_outbox
			WHERE event_type = 'vault.renamed'
			  AND aggregate_id = $1::uuid
		`,
		createdVault.ID,
	).Scan(
		&eventType,
		&aggregateID,
		&actorID,
		&correlationID,
		&sanitizedPayload,
	)
	if err != nil {
		t.Fatal(
			"read vault rename audit event",
		)
	}

	if eventType != "vault.renamed" {
		t.Fatalf(
			"event type = %q, want vault.renamed",
			eventType,
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
		vaultRenameTestCorrelationID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			correlationID,
			vaultRenameTestCorrelationID,
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
		"Original Vault",
	) ||
		strings.Contains(
			sanitizedPayload,
			"Renamed Vault",
		) {
		t.Fatal(
			"rename audit payload exposed a vault name",
		)
	}
}

func TestVaultStoreRenameOwnedEnforcesOwnership(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-rename-ownership-owner@example.com",
	)

	otherUser := createVaultTestUser(
		t,
		userStore,
		"vault-rename-ownership-other@example.com",
	)

	createdVault := createVaultReadTestVault(
		t,
		vaultStore,
		owner.ID,
		"Owner Vault",
		"vault-rename-ownership-create",
	)

	_, err := vaultStore.RenameOwned(
		context.Background(),
		vaultdomain.RenameStoreInput{
			OwnerID:       otherUser.ID,
			VaultID:       createdVault.ID,
			Name:          "Unauthorized Rename",
			CorrelationID: vaultRenameTestCorrelationID,
		},
	)
	if !errors.Is(
		err,
		vaultdomain.ErrVaultNotFound,
	) {
		t.Fatalf(
			"cross-user RenameOwned() error = %v, want %v",
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
		storedName       string
		renameEventCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT name
					FROM vaults
					WHERE id = $1::uuid
				),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE event_type = 'vault.renamed'
					  AND aggregate_id = $1::uuid
				)
		`,
		createdVault.ID,
	).Scan(
		&storedName,
		&renameEventCount,
	)
	if err != nil {
		t.Fatal(
			"inspect unauthorized rename result",
		)
	}

	if storedName != "Owner Vault" {
		t.Fatalf(
			"stored name = %q, want Owner Vault",
			storedName,
		)
	}

	if renameEventCount != 0 {
		t.Fatalf(
			"rename event count = %d, want 0",
			renameEventCount,
		)
	}
}

func TestVaultStoreRenameOwnedRollsBackWhenAuditInsertFails(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-rename-rollback@example.com",
	)

	createdVault := createVaultReadTestVault(
		t,
		vaultStore,
		owner.ID,
		"Original Vault",
		"vault-rename-rollback-create",
	)

	_, err := vaultStore.RenameOwned(
		context.Background(),
		vaultdomain.RenameStoreInput{
			OwnerID:       owner.ID,
			VaultID:       createdVault.ID,
			Name:          "Should Roll Back",
			CorrelationID: "",
		},
	)
	if !errors.Is(err, ErrDatabase) {
		t.Fatalf(
			"RenameOwned() error = %v, want %v",
			err,
			ErrDatabase,
		)
	}

	if strings.Contains(
		err.Error(),
		"audit_outbox_correlation_id_not_blank",
	) {
		t.Fatal(
			"rename error exposed a database constraint",
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var (
		storedName       string
		storedUpdatedAt  time.Time
		renameEventCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT name
					FROM vaults
					WHERE id = $1::uuid
				),
				(
					SELECT updated_at
					FROM vaults
					WHERE id = $1::uuid
				),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE event_type = 'vault.renamed'
					  AND aggregate_id = $1::uuid
				)
		`,
		createdVault.ID,
	).Scan(
		&storedName,
		&storedUpdatedAt,
		&renameEventCount,
	)
	if err != nil {
		t.Fatal(
			"inspect rolled-back vault rename",
		)
	}

	if storedName != "Original Vault" {
		t.Fatalf(
			"stored name after rollback = %q, want Original Vault",
			storedName,
		)
	}

	if !storedUpdatedAt.Equal(
		createdVault.UpdatedAt,
	) {
		t.Fatal(
			"vault timestamp changed despite transaction rollback",
		)
	}

	if renameEventCount != 0 {
		t.Fatalf(
			"rename event count after rollback = %d, want 0",
			renameEventCount,
		)
	}
}

func TestVaultStoreRenameOwnedHonorsCanceledContext(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-rename-canceled@example.com",
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := vaultStore.RenameOwned(
		ctx,
		vaultdomain.RenameStoreInput{
			OwnerID:       owner.ID,
			VaultID:       "00000000-0000-0000-0000-000000009999",
			Name:          "Canceled Rename",
			CorrelationID: vaultRenameTestCorrelationID,
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"RenameOwned() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}
