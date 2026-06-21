package store

import (
	"context"
	"errors"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const vaultItemDeleteCorrelationID = "vault-item-delete-integration-test"

func TestVaultStoreSoftDeleteItemPersistsStateAndAudit(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	deletedItem, err := fixture.store.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			ItemID:        createdItem.ID,
			CorrelationID: vaultItemDeleteCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf("SoftDeleteItem() error = %v", err)
	}

	if deletedItem.ID != createdItem.ID {
		t.Fatalf("item ID = %q, want %q", deletedItem.ID, createdItem.ID)
	}

	if deletedItem.VaultID != fixture.vaultID {
		t.Fatalf("vault ID = %q, want %q", deletedItem.VaultID, fixture.vaultID)
	}

	if !deletedItem.Deleted() {
		t.Fatal("soft-deleted item was returned as active")
	}

	if deletedItem.Version != createdItem.Version {
		t.Fatalf(
			"soft-deleted version = %d, want %d",
			deletedItem.Version,
			createdItem.Version,
		)
	}

	if deletedItem.UpdatedAt.Before(createdItem.UpdatedAt) {
		t.Fatalf(
			"deleted updated time = %v, previous updated time = %v",
			deletedItem.UpdatedAt,
			createdItem.UpdatedAt,
		)
	}

	_, err = fixture.store.GetItem(
		context.Background(),
		vaultdomain.GetItemStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			ItemID:  createdItem.ID,
		},
	)
	if !errors.Is(err, vaultdomain.ErrItemNotFound) {
		t.Fatalf("active GetItem() error = %v, want %v", err, vaultdomain.ErrItemNotFound)
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
		t.Fatalf("deleted GetItem() error = %v", err)
	}

	if !storedDeletedItem.Deleted() {
		t.Fatal("stored deleted item was returned as active")
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		versionCount     int
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
				event_type,
				aggregate_type,
				aggregate_id::text,
				actor_id::text,
				correlation_id,
				sanitized_payload::text
			FROM audit_outbox
			WHERE aggregate_type = 'vault_item'
			  AND aggregate_id = $1::uuid
			  AND event_type = 'vault_item.deleted'
		`,
		createdItem.ID,
	).Scan(
		&versionCount,
		&eventType,
		&aggregateType,
		&aggregateID,
		&actorID,
		&correlationID,
		&sanitizedPayload,
	)
	if err != nil {
		t.Fatalf("read soft-delete transaction: %v", err)
	}

	if versionCount != 1 {
		t.Fatalf("item version count = %d, want 1", versionCount)
	}

	if eventType != "vault_item.deleted" {
		t.Fatalf("event type = %q, want vault_item.deleted", eventType)
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

	if correlationID != vaultItemDeleteCorrelationID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			correlationID,
			vaultItemDeleteCorrelationID,
		)
	}

	if sanitizedPayload != "{}" {
		t.Fatalf("sanitized payload = %q, want {}", sanitizedPayload)
	}
}

func TestVaultStoreSoftDeleteItemUsesSafeNotFoundForInaccessibleItems(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	tests := []struct {
		name  string
		input vaultdomain.SoftDeleteItemStoreInput
	}{
		{
			name: "other owner",
			input: vaultdomain.SoftDeleteItemStoreInput{
				OwnerID:       fixture.otherOwnerID,
				VaultID:       fixture.vaultID,
				ItemID:        createdItem.ID,
				CorrelationID: vaultItemDeleteCorrelationID,
			},
		},
		{
			name: "wrong parent vault",
			input: vaultdomain.SoftDeleteItemStoreInput{
				OwnerID:       fixture.ownerID,
				VaultID:       fixture.otherVaultID,
				ItemID:        createdItem.ID,
				CorrelationID: vaultItemDeleteCorrelationID,
			},
		},
		{
			name: "unknown item",
			input: vaultdomain.SoftDeleteItemStoreInput{
				OwnerID:       fixture.ownerID,
				VaultID:       fixture.vaultID,
				ItemID:        vaultItemUnknownID,
				CorrelationID: vaultItemDeleteCorrelationID,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := fixture.store.SoftDeleteItem(context.Background(), test.input)

			if !errors.Is(err, vaultdomain.ErrItemNotFound) {
				t.Fatalf(
					"SoftDeleteItem() error = %v, want %v",
					err,
					vaultdomain.ErrItemNotFound,
				)
			}
		})
	}
}

func TestVaultStoreSoftDeleteItemRejectsAlreadyDeletedItem(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	input := vaultdomain.SoftDeleteItemStoreInput{
		OwnerID:       fixture.ownerID,
		VaultID:       fixture.vaultID,
		ItemID:        createdItem.ID,
		CorrelationID: vaultItemDeleteCorrelationID,
	}

	if _, err := fixture.store.SoftDeleteItem(context.Background(), input); err != nil {
		t.Fatalf("first SoftDeleteItem() error = %v", err)
	}

	_, err := fixture.store.SoftDeleteItem(context.Background(), input)

	if !errors.Is(err, vaultdomain.ErrItemNotFound) {
		t.Fatalf("second SoftDeleteItem() error = %v, want %v", err, vaultdomain.ErrItemNotFound)
	}
}

func TestVaultStoreSoftDeleteItemRollsBackWhenAuditInsertFails(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	_, err := fixture.store.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			ItemID:        createdItem.ID,
			CorrelationID: "",
		},
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("SoftDeleteItem() error = %v, want %v", err, ErrDatabase)
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
		t.Fatalf("read item after soft-delete rollback: %v", err)
	}

	if activeItem.Deleted() {
		t.Fatal("item remained deleted after transaction rollback")
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
					  AND event_type = 'vault_item.deleted'
				)
		`,
		createdItem.ID,
	).Scan(&versionCount, &auditCount)
	if err != nil {
		t.Fatalf("count rolled-back soft-delete rows: %v", err)
	}

	if versionCount != 1 {
		t.Fatalf("item version count after rollback = %d, want 1", versionCount)
	}

	if auditCount != 0 {
		t.Fatalf("delete audit count after rollback = %d, want 0", auditCount)
	}
}

func TestVaultStoreSoftDeleteItemPreservesCanceledContext(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fixture.store.SoftDeleteItem(ctx, vaultdomain.SoftDeleteItemStoreInput{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SoftDeleteItem() error = %v, want %v", err, context.Canceled)
	}
}

func TestVaultStoreSoftDeleteItemMapsMissingDatabaseSafely(t *testing.T) {
	store := NewVaultStore(nil)

	_, err := store.SoftDeleteItem(context.Background(), vaultdomain.SoftDeleteItemStoreInput{})

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("SoftDeleteItem() error = %v, want %v", err, ErrDatabase)
	}
}
