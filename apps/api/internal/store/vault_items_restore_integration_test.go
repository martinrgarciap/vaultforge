package store

import (
	"bytes"
	"context"
	"errors"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const vaultItemRestoreCorrelationID = "vault-item-restore-integration-test"

func TestVaultStoreRestoreItemPersistsActiveStateVersionAndAudit(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	deletedItem, err := fixture.store.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   vaultItemDeleteCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf("SoftDeleteItem() error = %v", err)
	}

	restoredItem, err := fixture.store.RestoreItem(
		context.Background(),
		vaultdomain.RestoreItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   vaultItemRestoreCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf("RestoreItem() error = %v", err)
	}

	if restoredItem.ID != createdItem.ID {
		t.Fatalf("item ID = %q, want %q", restoredItem.ID, createdItem.ID)
	}

	if restoredItem.VaultID != fixture.vaultID {
		t.Fatalf("vault ID = %q, want %q", restoredItem.VaultID, fixture.vaultID)
	}

	if restoredItem.Deleted() {
		t.Fatal("restored item remained deleted")
	}

	if restoredItem.Version != 2 {
		t.Fatalf("restored version = %d, want 2", restoredItem.Version)
	}

	if restoredItem.UpdatedAt.Before(deletedItem.UpdatedAt) {
		t.Fatalf(
			"restored updated time = %v, deleted updated time = %v",
			restoredItem.UpdatedAt,
			deletedItem.UpdatedAt,
		)
	}

	if restoredItem.Type != createdItem.Type {
		t.Fatalf("restored type = %q, want %q", restoredItem.Type, createdItem.Type)
	}

	if !bytes.Equal(restoredItem.Payload, createdItem.Payload) {
		t.Fatal("restored payload did not match the original payload")
	}

	activeItem, err := fixture.store.GetItem(
		context.Background(),
		vaultdomain.GetItemStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			ItemID:  createdItem.ID,
		},
	)
	if err != nil {
		t.Fatalf("active GetItem() after restore error = %v", err)
	}

	if activeItem.Deleted() {
		t.Fatal("active lookup returned restored item as deleted")
	}

	_, err = fixture.store.GetItem(
		context.Background(),
		vaultdomain.GetItemStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			ItemID:  createdItem.ID,
			State:   vaultdomain.ItemListStateDeleted,
		},
	)
	if !errors.Is(err, vaultdomain.ErrItemNotFound) {
		t.Fatalf("deleted GetItem() error = %v, want %v", err, vaultdomain.ErrItemNotFound)
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		versionCount     int
		versionOneType   string
		versionOneData   []byte
		versionTwoType   string
		versionTwoData   []byte
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
				(
					SELECT count(*)
					FROM item_versions
					WHERE vault_item_id = $1::uuid
				),
				version_one.item_type,
				version_one.encrypted_payload,
				version_two.item_type,
				version_two.encrypted_payload,
				audit_outbox.event_type,
				audit_outbox.aggregate_type,
				audit_outbox.aggregate_id::text,
				audit_outbox.actor_id::text,
				audit_outbox.correlation_id,
				audit_outbox.sanitized_payload::text
			FROM item_versions AS version_one
			JOIN item_versions AS version_two
			  ON version_two.vault_item_id = version_one.vault_item_id
			JOIN audit_outbox
			  ON audit_outbox.aggregate_type = 'vault_item'
			 AND audit_outbox.aggregate_id = version_one.vault_item_id
			 AND audit_outbox.event_type = 'vault_item.restored'
			WHERE version_one.vault_item_id = $1::uuid
			  AND version_one.version = 1
			  AND version_two.version = 2
		`,
		createdItem.ID,
	).Scan(
		&versionCount,
		&versionOneType,
		&versionOneData,
		&versionTwoType,
		&versionTwoData,
		&eventType,
		&aggregateType,
		&aggregateID,
		&actorID,
		&correlationID,
		&sanitizedPayload,
	)
	if err != nil {
		t.Fatalf("read restored item transaction: %v", err)
	}

	if versionCount != 2 {
		t.Fatalf("item version count = %d, want 2", versionCount)
	}

	if versionTwoType != versionOneType {
		t.Fatalf("version 2 type = %q, want %q", versionTwoType, versionOneType)
	}

	if !bytes.Equal(versionTwoData, versionOneData) {
		t.Fatal("restored version payload did not match version 1")
	}

	if eventType != "vault_item.restored" {
		t.Fatalf("event type = %q, want vault_item.restored", eventType)
	}

	if aggregateType != "vault_item" {
		t.Fatalf("aggregate type = %q, want vault_item", aggregateType)
	}

	if aggregateID != createdItem.ID {
		t.Fatalf("aggregate ID = %q, want %q", aggregateID, createdItem.ID)
	}

	if actorID != fixture.ownerID {
		t.Fatalf("actor ID = %q, want %q", actorID, fixture.ownerID)
	}

	if correlationID != vaultItemRestoreCorrelationID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			correlationID,
			vaultItemRestoreCorrelationID,
		)
	}

	assertVaultItemAuditPayload(
		t,
		sanitizedPayload,
		restoredItem.VaultID,
		restoredItem.Type,
		restoredItem.Version,
		string(restoredItem.Payload),
	)
}

func TestVaultStoreRestoreItemUsesSafeNotFoundForInaccessibleItems(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	if _, err := fixture.store.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   vaultItemDeleteCorrelationID,
		},
	); err != nil {
		t.Fatalf("SoftDeleteItem() error = %v", err)
	}

	tests := []struct {
		name  string
		input vaultdomain.RestoreItemStoreInput
	}{
		{
			name: "other owner",
			input: vaultdomain.RestoreItemStoreInput{
				OwnerID:         fixture.otherOwnerID,
				VaultID:         fixture.vaultID,
				ItemID:          createdItem.ID,
				ExpectedVersion: 1,
				CorrelationID:   vaultItemRestoreCorrelationID,
			},
		},
		{
			name: "wrong parent vault",
			input: vaultdomain.RestoreItemStoreInput{
				OwnerID:         fixture.ownerID,
				VaultID:         fixture.otherVaultID,
				ItemID:          createdItem.ID,
				ExpectedVersion: 1,
				CorrelationID:   vaultItemRestoreCorrelationID,
			},
		},
		{
			name: "unknown item",
			input: vaultdomain.RestoreItemStoreInput{
				OwnerID:         fixture.ownerID,
				VaultID:         fixture.vaultID,
				ItemID:          vaultItemUnknownID,
				ExpectedVersion: 1,
				CorrelationID:   vaultItemRestoreCorrelationID,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := fixture.store.RestoreItem(context.Background(), test.input)

			if !errors.Is(err, vaultdomain.ErrItemNotFound) {
				t.Fatalf(
					"RestoreItem() error = %v, want %v",
					err,
					vaultdomain.ErrItemNotFound,
				)
			}
		})
	}
}

func TestVaultStoreRestoreItemRejectsActiveItem(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	_, err := fixture.store.RestoreItem(
		context.Background(),
		vaultdomain.RestoreItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   vaultItemRestoreCorrelationID,
		},
	)

	if !errors.Is(err, vaultdomain.ErrItemNotFound) {
		t.Fatalf("RestoreItem() error = %v, want %v", err, vaultdomain.ErrItemNotFound)
	}
}

func TestVaultStoreRestoreItemRollsBackWhenAuditInsertFails(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	if _, err := fixture.store.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   vaultItemDeleteCorrelationID,
		},
	); err != nil {
		t.Fatalf("SoftDeleteItem() error = %v", err)
	}

	_, err := fixture.store.RestoreItem(
		context.Background(),
		vaultdomain.RestoreItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   "",
		},
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("RestoreItem() error = %v, want %v", err, ErrDatabase)
	}

	deletedItem, err := fixture.store.GetItem(
		context.Background(),
		vaultdomain.GetItemStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			ItemID:  createdItem.ID,
			State:   vaultdomain.ItemListStateDeleted,
		},
	)
	if err != nil {
		t.Fatalf("read item after restore rollback: %v", err)
	}

	if !deletedItem.Deleted() {
		t.Fatal("item became active after restore rollback")
	}

	if deletedItem.Version != 1 {
		t.Fatalf("item version after rollback = %d, want 1", deletedItem.Version)
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		versionCount int
		auditCount   int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT count(*)
					FROM item_versions
					WHERE vault_item_id = $1::uuid
				),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE aggregate_type = 'vault_item'
					  AND aggregate_id = $1::uuid
					  AND event_type = 'vault_item.restored'
				)
		`,
		createdItem.ID,
	).Scan(&versionCount, &auditCount)
	if err != nil {
		t.Fatalf("count rolled-back restore rows: %v", err)
	}

	if versionCount != 1 {
		t.Fatalf("item version count after rollback = %d, want 1", versionCount)
	}

	if auditCount != 0 {
		t.Fatalf("restore audit count after rollback = %d, want 0", auditCount)
	}
}

func TestVaultStoreRestoreItemPreservesCanceledContext(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fixture.store.RestoreItem(ctx, vaultdomain.RestoreItemStoreInput{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RestoreItem() error = %v, want %v", err, context.Canceled)
	}
}

func TestVaultStoreRestoreItemMapsMissingDatabaseSafely(t *testing.T) {
	store := NewVaultStore(nil)

	_, err := store.RestoreItem(context.Background(), vaultdomain.RestoreItemStoreInput{})

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("RestoreItem() error = %v, want %v", err, ErrDatabase)
	}
}

func TestVaultStoreRestoreItemReturnsConflictForStaleVersion(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	deletedItem, err := fixture.store.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   "vault-item-first-soft-delete",
		},
	)
	if err != nil {
		t.Fatalf("first SoftDeleteItem() error = %v", err)
	}

	firstRestore, err := fixture.store.RestoreItem(
		context.Background(),
		vaultdomain.RestoreItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: deletedItem.Version,
			CorrelationID:   "vault-item-first-restore",
		},
	)
	if err != nil {
		t.Fatalf("first RestoreItem() error = %v", err)
	}

	if firstRestore.Version != 2 {
		t.Fatalf("first restored version = %d, want 2", firstRestore.Version)
	}

	secondDeletedItem, err := fixture.store.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: firstRestore.Version,
			CorrelationID:   "vault-item-second-soft-delete",
		},
	)
	if err != nil {
		t.Fatalf("second SoftDeleteItem() error = %v", err)
	}

	if secondDeletedItem.Version != 2 {
		t.Fatalf("second deleted version = %d, want 2", secondDeletedItem.Version)
	}

	_, err = fixture.store.RestoreItem(
		context.Background(),
		vaultdomain.RestoreItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   "vault-item-stale-restore",
		},
	)
	if !errors.Is(err, vaultdomain.ErrItemConflict) {
		t.Fatalf(
			"stale RestoreItem() error = %v, want %v",
			err,
			vaultdomain.ErrItemConflict,
		)
	}

	storedDeletedItem, err := fixture.store.GetItem(
		context.Background(),
		vaultdomain.GetItemStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			ItemID:  createdItem.ID,
			State:   vaultdomain.ItemListStateDeleted,
		},
	)
	if err != nil {
		t.Fatalf("read item after stale restore: %v", err)
	}

	if !storedDeletedItem.Deleted() {
		t.Fatal("stale restore changed the deleted item state")
	}

	if storedDeletedItem.Version != 2 {
		t.Fatalf("stored deleted version = %d, want 2", storedDeletedItem.Version)
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		versionCount int
		restoreCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT count(*)
					FROM item_versions
					WHERE vault_item_id = $1::uuid
				),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE aggregate_type = 'vault_item'
					  AND aggregate_id = $1::uuid
					  AND event_type = 'vault_item.restored'
				)
		`,
		createdItem.ID,
	).Scan(&versionCount, &restoreCount)
	if err != nil {
		t.Fatalf("count stale restore rows: %v", err)
	}

	if versionCount != 2 {
		t.Fatalf("item version count = %d, want 2", versionCount)
	}

	if restoreCount != 1 {
		t.Fatalf("restore audit count = %d, want 1", restoreCount)
	}
}
