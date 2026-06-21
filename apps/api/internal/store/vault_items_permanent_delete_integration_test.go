package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const vaultItemPermanentDeleteCorrelationID = "vault-item-permanent-delete-integration-test"

func TestVaultStorePermanentDeleteItemRemovesDeletedItemVersionsAndPreservesAudit(t *testing.T) {
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

	err := fixture.store.PermanentDeleteItem(
		context.Background(),
		vaultdomain.PermanentDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   vaultItemPermanentDeleteCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf("PermanentDeleteItem() error = %v", err)
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
		t.Fatalf("GetItem() error = %v, want %v", err, vaultdomain.ErrItemNotFound)
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		itemCount        int
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
				FROM vault_items
				WHERE id = $1::uuid
			),
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
		  AND event_type = 'vault_item.permanently_deleted'
	`,
		createdItem.ID,
	).Scan(
		&itemCount,
		&versionCount,
		&eventType,
		&aggregateType,
		&aggregateID,
		&actorID,
		&correlationID,
		&sanitizedPayload,
	)
	if err != nil {
		t.Fatalf("read permanent-delete transaction: %v", err)
	}

	if itemCount != 0 {
		t.Fatalf("vault item count = %d, want 0", itemCount)
	}

	if versionCount != 0 {
		t.Fatalf("item version count = %d, want 0", versionCount)
	}

	if eventType != "vault_item.permanently_deleted" {
		t.Fatalf("event type = %q, want vault_item.permanently_deleted", eventType)
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

	if correlationID != vaultItemPermanentDeleteCorrelationID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			correlationID,
			vaultItemPermanentDeleteCorrelationID,
		)
	}

	if sanitizedPayload != "{}" {
		t.Fatalf("sanitized payload = %q, want {}", sanitizedPayload)
	}

}

func TestVaultStorePermanentDeleteItemRejectsActiveItem(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	err := fixture.store.PermanentDeleteItem(
		context.Background(),
		vaultdomain.PermanentDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   vaultItemPermanentDeleteCorrelationID,
		},
	)

	if !errors.Is(err, vaultdomain.ErrItemNotFound) {
		t.Fatalf("PermanentDeleteItem() error = %v, want %v", err, vaultdomain.ErrItemNotFound)
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
		t.Fatalf("read active item after rejected permanent delete: %v", err)
	}

	if activeItem.Deleted() {
		t.Fatal("active item changed after rejected permanent delete")
	}

}

func TestVaultStorePermanentDeleteItemUsesSafeNotFoundForInaccessibleItems(t *testing.T) {
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
		input vaultdomain.PermanentDeleteItemStoreInput
	}{
		{
			name: "other owner",
			input: vaultdomain.PermanentDeleteItemStoreInput{
				OwnerID:         fixture.otherOwnerID,
				VaultID:         fixture.vaultID,
				ItemID:          createdItem.ID,
				ExpectedVersion: 1,
				CorrelationID:   vaultItemPermanentDeleteCorrelationID,
			},
		},
		{
			name: "wrong parent vault",
			input: vaultdomain.PermanentDeleteItemStoreInput{
				OwnerID:         fixture.ownerID,
				VaultID:         fixture.otherVaultID,
				ItemID:          createdItem.ID,
				ExpectedVersion: 1,
				CorrelationID:   vaultItemPermanentDeleteCorrelationID,
			},
		},
		{
			name: "unknown item",
			input: vaultdomain.PermanentDeleteItemStoreInput{
				OwnerID:         fixture.ownerID,
				VaultID:         fixture.vaultID,
				ItemID:          vaultItemUnknownID,
				ExpectedVersion: 1,
				CorrelationID:   vaultItemPermanentDeleteCorrelationID,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := fixture.store.PermanentDeleteItem(context.Background(), test.input)

			if !errors.Is(err, vaultdomain.ErrItemNotFound) {
				t.Fatalf(
					"PermanentDeleteItem() error = %v, want %v",
					err,
					vaultdomain.ErrItemNotFound,
				)
			}
		})
	}

}

func TestVaultStorePermanentDeleteItemRollsBackWhenAuditInsertFails(t *testing.T) {
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

	err := fixture.store.PermanentDeleteItem(
		context.Background(),
		vaultdomain.PermanentDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   "",
		},
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("PermanentDeleteItem() error = %v, want %v", err, ErrDatabase)
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
		t.Fatalf("read item after permanent-delete rollback: %v", err)
	}

	if !deletedItem.Deleted() {
		t.Fatal("item was not restored by transaction rollback")
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		itemCount    int
		versionCount int
		auditCount   int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
		SELECT
			(
				SELECT count(*)
				FROM vault_items
				WHERE id = $1::uuid
			),
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
				  AND event_type = 'vault_item.permanently_deleted'
			)
	`,
		createdItem.ID,
	).Scan(&itemCount, &versionCount, &auditCount)
	if err != nil {
		t.Fatalf("count rolled-back permanent-delete rows: %v", err)
	}

	if itemCount != 1 {
		t.Fatalf("vault item count after rollback = %d, want 1", itemCount)
	}

	if versionCount != 1 {
		t.Fatalf("item version count after rollback = %d, want 1", versionCount)
	}

	if auditCount != 0 {
		t.Fatalf("permanent-delete audit count after rollback = %d, want 0", auditCount)
	}

}

func TestVaultStorePermanentDeleteItemPreservesCanceledContext(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := fixture.store.PermanentDeleteItem(ctx, vaultdomain.PermanentDeleteItemStoreInput{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PermanentDeleteItem() error = %v, want %v", err, context.Canceled)
	}

}

func TestVaultStorePermanentDeleteItemMapsMissingDatabaseSafely(t *testing.T) {
	store := NewVaultStore(nil)

	err := store.PermanentDeleteItem(
		context.Background(),
		vaultdomain.PermanentDeleteItemStoreInput{},
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("PermanentDeleteItem() error = %v, want %v", err, ErrDatabase)
	}

}

func TestVaultStorePermanentDeleteItemReturnsConflictForStaleVersion(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{"value":"version-two"}`),
	)
	if err != nil {
		t.Fatalf("create version-two envelope: %v", err)
	}

	updatedItem, err := fixture.store.UpdateItem(
		context.Background(),
		vaultdomain.UpdateItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			Type:            vaultdomain.ItemTypeAPIKey,
			Envelope:        envelope,
			ExpectedVersion: 1,
			CorrelationID:   "vault-item-update-before-permanent-delete",
		},
	)
	if err != nil {
		t.Fatalf("UpdateItem() error = %v", err)
	}

	if updatedItem.Version != 2 {
		t.Fatalf("updated version = %d, want 2", updatedItem.Version)
	}

	deletedItem, err := fixture.store.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: updatedItem.Version,
			CorrelationID:   "vault-item-delete-before-permanent-delete",
		},
	)
	if err != nil {
		t.Fatalf("SoftDeleteItem() error = %v", err)
	}

	if deletedItem.Version != 2 {
		t.Fatalf("deleted version = %d, want 2", deletedItem.Version)
	}

	err = fixture.store.PermanentDeleteItem(
		context.Background(),
		vaultdomain.PermanentDeleteItemStoreInput{
			OwnerID:         fixture.ownerID,
			VaultID:         fixture.vaultID,
			ItemID:          createdItem.ID,
			ExpectedVersion: 1,
			CorrelationID:   "vault-item-stale-permanent-delete",
		},
	)
	if !errors.Is(err, vaultdomain.ErrItemConflict) {
		t.Fatalf(
			"stale PermanentDeleteItem() error = %v, want %v",
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
		t.Fatalf("read item after stale permanent delete: %v", err)
	}

	if !storedDeletedItem.Deleted() {
		t.Fatal("stale permanent delete changed the deleted state")
	}

	if storedDeletedItem.Version != 2 {
		t.Fatalf("stored deleted version = %d, want 2", storedDeletedItem.Version)
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		itemCount            int
		versionCount         int
		permanentDeleteCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT count(*)
					FROM vault_items
					WHERE id = $1::uuid
				),
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
					  AND event_type = 'vault_item.permanently_deleted'
				)
		`,
		createdItem.ID,
	).Scan(
		&itemCount,
		&versionCount,
		&permanentDeleteCount,
	)
	if err != nil {
		t.Fatalf("count stale permanent-delete rows: %v", err)
	}

	if itemCount != 1 {
		t.Fatalf("vault item count = %d, want 1", itemCount)
	}

	if versionCount != 2 {
		t.Fatalf("item version count = %d, want 2", versionCount)
	}

	if permanentDeleteCount != 0 {
		t.Fatalf(
			"permanent-delete audit count = %d, want 0",
			permanentDeleteCount,
		)
	}
}
