package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const vaultItemUpdateCorrelationID = "vault-item-update-integration-test"

func TestVaultStoreUpdateItemPersistsNewVersionAndAudit(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{
			"label": "Updated Item",
			"value": "updated-synthetic-value"
		}`),
	)
	if err != nil {
		t.Fatalf("create updated synthetic envelope: %v", err)
	}

	updatedItem, err := fixture.store.UpdateItem(
		context.Background(),
		vaultdomain.UpdateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			ItemID:        createdItem.ID,
			Type:          vaultdomain.ItemTypeAPIKey,
			Envelope:      envelope,
			CorrelationID: vaultItemUpdateCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf("UpdateItem() error = %v", err)
	}

	if updatedItem.ID != createdItem.ID {
		t.Fatalf("item ID = %q, want %q", updatedItem.ID, createdItem.ID)
	}

	if updatedItem.VaultID != fixture.vaultID {
		t.Fatalf("vault ID = %q, want %q", updatedItem.VaultID, fixture.vaultID)
	}

	if updatedItem.Type != vaultdomain.ItemTypeAPIKey {
		t.Fatalf("item type = %q, want %q", updatedItem.Type, vaultdomain.ItemTypeAPIKey)
	}

	if updatedItem.Version != 2 {
		t.Fatalf("item version = %d, want 2", updatedItem.Version)
	}

	if !updatedItem.CreatedAt.Equal(createdItem.CreatedAt) {
		t.Fatalf("created time = %v, want %v", updatedItem.CreatedAt, createdItem.CreatedAt)
	}

	if updatedItem.UpdatedAt.Before(createdItem.UpdatedAt) {
		t.Fatalf(
			"updated time = %v, previous updated time = %v",
			updatedItem.UpdatedAt,
			createdItem.UpdatedAt,
		)
	}

	if updatedItem.Deleted() {
		t.Fatal("updated active item was marked as deleted")
	}

	if !bytes.Equal(updatedItem.Payload, envelope.Payload) {
		t.Fatal("updated payload did not match the synthetic envelope")
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var versionCount int

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT count(*)
			FROM item_versions
			WHERE vault_item_id = $1::uuid
		`,
		createdItem.ID,
	).Scan(&versionCount)
	if err != nil {
		t.Fatalf("count item versions: %v", err)
	}

	if versionCount != 2 {
		t.Fatalf("item version count = %d, want 2", versionCount)
	}

	var (
		versionOneType    string
		versionOnePayload []byte
		versionTwoType    string
		versionTwoPayload []byte
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				version_one.item_type,
				version_one.encrypted_payload,
				version_two.item_type,
				version_two.encrypted_payload
			FROM item_versions AS version_one
			JOIN item_versions AS version_two
			  ON version_two.vault_item_id = version_one.vault_item_id
			WHERE version_one.vault_item_id = $1::uuid
			  AND version_one.version = 1
			  AND version_two.version = 2
		`,
		createdItem.ID,
	).Scan(
		&versionOneType,
		&versionOnePayload,
		&versionTwoType,
		&versionTwoPayload,
	)
	if err != nil {
		t.Fatalf("read immutable item versions: %v", err)
	}

	if versionOneType != string(vaultdomain.ItemTypeSecureNote) {
		t.Fatalf(
			"version 1 type = %q, want %q",
			versionOneType,
			vaultdomain.ItemTypeSecureNote,
		)
	}

	const originalPayload = `{"label":"Retrieve Test","value":"synthetic-only"}`

	if string(versionOnePayload) != originalPayload {
		t.Fatalf("version 1 payload = %s, want %s", versionOnePayload, originalPayload)
	}

	if versionTwoType != string(vaultdomain.ItemTypeAPIKey) {
		t.Fatalf(
			"version 2 type = %q, want %q",
			versionTwoType,
			vaultdomain.ItemTypeAPIKey,
		)
	}

	if !bytes.Equal(versionTwoPayload, envelope.Payload) {
		t.Fatal("version 2 payload did not match the updated payload")
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
			WHERE aggregate_type = 'vault_item'
			  AND aggregate_id = $1::uuid
			  AND event_type = 'vault_item.updated'
		`,
		createdItem.ID,
	).Scan(
		&eventType,
		&aggregateType,
		&aggregateID,
		&actorID,
		&correlationID,
		&sanitizedPayload,
	)
	if err != nil {
		t.Fatalf("read updated item audit event: %v", err)
	}

	if eventType != "vault_item.updated" {
		t.Fatalf("event type = %q, want vault_item.updated", eventType)
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

	if correlationID != vaultItemUpdateCorrelationID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			correlationID,
			vaultItemUpdateCorrelationID,
		)
	}

	if sanitizedPayload != "{}" {
		t.Fatalf("sanitized payload = %q, want {}", sanitizedPayload)
	}
}

func TestVaultStoreUpdateItemUsesSafeNotFoundForInaccessibleItems(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{"value":"updated"}`),
	)
	if err != nil {
		t.Fatalf("create update test envelope: %v", err)
	}

	tests := []struct {
		name  string
		input vaultdomain.UpdateItemStoreInput
	}{
		{
			name: "other owner",
			input: vaultdomain.UpdateItemStoreInput{
				OwnerID:       fixture.otherOwnerID,
				VaultID:       fixture.vaultID,
				ItemID:        createdItem.ID,
				Type:          vaultdomain.ItemTypeAPIKey,
				Envelope:      envelope,
				CorrelationID: vaultItemUpdateCorrelationID,
			},
		},
		{
			name: "wrong parent vault",
			input: vaultdomain.UpdateItemStoreInput{
				OwnerID:       fixture.ownerID,
				VaultID:       fixture.otherVaultID,
				ItemID:        createdItem.ID,
				Type:          vaultdomain.ItemTypeAPIKey,
				Envelope:      envelope,
				CorrelationID: vaultItemUpdateCorrelationID,
			},
		},
		{
			name: "unknown item",
			input: vaultdomain.UpdateItemStoreInput{
				OwnerID:       fixture.ownerID,
				VaultID:       fixture.vaultID,
				ItemID:        vaultItemUnknownID,
				Type:          vaultdomain.ItemTypeAPIKey,
				Envelope:      envelope,
				CorrelationID: vaultItemUpdateCorrelationID,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := fixture.store.UpdateItem(context.Background(), test.input)

			if !errors.Is(err, vaultdomain.ErrItemNotFound) {
				t.Fatalf("UpdateItem() error = %v, want %v", err, vaultdomain.ErrItemNotFound)
			}
		})
	}
}

func TestVaultStoreUpdateItemRejectsDeletedItem(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	softDeleteVaultItemForGetTest(t, createdItem.ID)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{"value":"updated"}`),
	)
	if err != nil {
		t.Fatalf("create deleted update envelope: %v", err)
	}

	_, err = fixture.store.UpdateItem(
		context.Background(),
		vaultdomain.UpdateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			ItemID:        createdItem.ID,
			Type:          vaultdomain.ItemTypeAPIKey,
			Envelope:      envelope,
			CorrelationID: vaultItemUpdateCorrelationID,
		},
	)

	if !errors.Is(err, vaultdomain.ErrItemNotFound) {
		t.Fatalf("UpdateItem() error = %v, want %v", err, vaultdomain.ErrItemNotFound)
	}
}

func TestVaultStoreUpdateItemRollsBackWhenAuditInsertFails(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{"value":"updated"}`),
	)
	if err != nil {
		t.Fatalf("create rollback update envelope: %v", err)
	}

	_, err = fixture.store.UpdateItem(
		context.Background(),
		vaultdomain.UpdateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			ItemID:        createdItem.ID,
			Type:          vaultdomain.ItemTypeAPIKey,
			Envelope:      envelope,
			CorrelationID: "",
		},
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("UpdateItem() error = %v, want %v", err, ErrDatabase)
	}

	storedItem, err := fixture.store.GetItem(
		context.Background(),
		vaultdomain.GetItemStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			ItemID:  createdItem.ID,
		},
	)
	if err != nil {
		t.Fatalf("read item after update rollback: %v", err)
	}

	if storedItem.Version != 1 {
		t.Fatalf("item version after rollback = %d, want 1", storedItem.Version)
	}

	if storedItem.Type != vaultdomain.ItemTypeSecureNote {
		t.Fatalf(
			"item type after rollback = %q, want %q",
			storedItem.Type,
			vaultdomain.ItemTypeSecureNote,
		)
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
					  AND event_type = 'vault_item.updated'
				)
		`,
		createdItem.ID,
	).Scan(&versionCount, &auditCount)
	if err != nil {
		t.Fatalf("count rolled-back update rows: %v", err)
	}

	if versionCount != 1 {
		t.Fatalf("item version count after rollback = %d, want 1", versionCount)
	}

	if auditCount != 0 {
		t.Fatalf("updated audit count after rollback = %d, want 0", auditCount)
	}
}

func TestVaultStoreUpdateItemPreservesCanceledContext(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fixture.store.UpdateItem(ctx, vaultdomain.UpdateItemStoreInput{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateItem() error = %v, want %v", err, context.Canceled)
	}
}

func TestVaultStoreUpdateItemMapsMissingDatabaseSafely(t *testing.T) {
	store := NewVaultStore(nil)

	_, err := store.UpdateItem(context.Background(), vaultdomain.UpdateItemStoreInput{})

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("UpdateItem() error = %v, want %v", err, ErrDatabase)
	}
}
