package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const vaultItemCreateCorrelationID = "vault-item-create-integration-test"

func TestVaultStoreCreateItemPersistsVersionAndAudit(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{"label":"Development","value":"synthetic-only"}`),
	)
	if err != nil {
		t.Fatalf("create synthetic item envelope: %v", err)
	}

	createdItem, err := fixture.store.CreateItem(
		context.Background(),
		vaultdomain.CreateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			Type:          vaultdomain.ItemTypeAPIKey,
			Envelope:      envelope,
			CorrelationID: vaultItemCreateCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}

	if createdItem.ID == "" {
		t.Fatal("created item did not contain an ID")
	}

	if createdItem.VaultID != fixture.vaultID {
		t.Fatalf("vault ID = %q, want %q", createdItem.VaultID, fixture.vaultID)
	}

	if createdItem.Type != vaultdomain.ItemTypeAPIKey {
		t.Fatalf("item type = %q, want %q", createdItem.Type, vaultdomain.ItemTypeAPIKey)
	}

	if createdItem.Version != 1 {
		t.Fatalf("version = %d, want 1", createdItem.Version)
	}

	if createdItem.CreatedAt.IsZero() || createdItem.UpdatedAt.IsZero() {
		t.Fatal("created item did not contain timestamps")
	}

	if createdItem.Deleted() {
		t.Fatal("newly created item was marked as deleted")
	}

	if !bytes.Equal(createdItem.Payload, envelope.Payload) {
		t.Fatal("created item payload did not match the normalized payload")
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		storedType       string
		storedPayload    []byte
		storedNonce      []byte
		snapshotVersion  int
		snapshotType     string
		snapshotPayload  []byte
		snapshotNonce    []byte
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
				vault_items.item_type,
				vault_items.encrypted_payload,
				vault_items.nonce,
				item_versions.version,
				item_versions.item_type,
				item_versions.encrypted_payload,
				item_versions.nonce,
				audit_outbox.event_type,
				audit_outbox.aggregate_type,
				audit_outbox.aggregate_id::text,
				audit_outbox.actor_id::text,
				audit_outbox.correlation_id,
				audit_outbox.sanitized_payload::text
			FROM vault_items
			JOIN item_versions
			  ON item_versions.vault_item_id = vault_items.id
			 AND item_versions.version = vault_items.version
			JOIN audit_outbox
			  ON audit_outbox.aggregate_type = 'vault_item'
			 AND audit_outbox.aggregate_id = vault_items.id
			WHERE vault_items.id = $1::uuid
		`,
		createdItem.ID,
	).Scan(
		&storedType,
		&storedPayload,
		&storedNonce,
		&snapshotVersion,
		&snapshotType,
		&snapshotPayload,
		&snapshotNonce,
		&eventType,
		&aggregateType,
		&aggregateID,
		&actorID,
		&correlationID,
		&sanitizedPayload,
	)
	if err != nil {
		t.Fatalf("read created vault item transaction: %v", err)
	}

	if storedType != string(vaultdomain.ItemTypeAPIKey) {
		t.Fatalf("stored item type = %q, want %q", storedType, vaultdomain.ItemTypeAPIKey)
	}

	if !bytes.Equal(storedPayload, envelope.Payload) {
		t.Fatal("stored item payload did not match the synthetic envelope")
	}

	if !bytes.Equal(storedNonce, envelope.Nonce) {
		t.Fatal("stored item nonce did not match the synthetic envelope")
	}

	if snapshotVersion != 1 {
		t.Fatalf("snapshot version = %d, want 1", snapshotVersion)
	}

	if snapshotType != storedType {
		t.Fatal("snapshot item type did not match the current item")
	}

	if !bytes.Equal(snapshotPayload, storedPayload) {
		t.Fatal("snapshot payload did not match the current item")
	}

	if !bytes.Equal(snapshotNonce, storedNonce) {
		t.Fatal("snapshot nonce did not match the current item")
	}

	if eventType != "vault_item.created" {
		t.Fatalf("event type = %q, want vault_item.created", eventType)
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

	if correlationID != vaultItemCreateCorrelationID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			correlationID,
			vaultItemCreateCorrelationID,
		)
	}

	if sanitizedPayload != "{}" {
		t.Fatalf("sanitized payload = %q, want {}", sanitizedPayload)
	}
}

func TestVaultStoreCreateItemUsesSafeNotFoundForOtherOwner(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(json.RawMessage(`{"value":"synthetic"}`))
	if err != nil {
		t.Fatalf("create synthetic item envelope: %v", err)
	}

	_, err = fixture.store.CreateItem(
		context.Background(),
		vaultdomain.CreateItemStoreInput{
			OwnerID:       fixture.otherOwnerID,
			VaultID:       fixture.vaultID,
			Type:          vaultdomain.ItemTypeSecureNote,
			Envelope:      envelope,
			CorrelationID: vaultItemCreateCorrelationID,
		},
	)

	if !errors.Is(err, vaultdomain.ErrVaultNotFound) {
		t.Fatalf("CreateItem() error = %v, want %v", err, vaultdomain.ErrVaultNotFound)
	}

	assertNoVaultItemMutationRows(t)
}

func TestVaultStoreCreateItemRollsBackWhenAuditInsertFails(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(json.RawMessage(`{"value":"synthetic"}`))
	if err != nil {
		t.Fatalf("create synthetic item envelope: %v", err)
	}

	_, err = fixture.store.CreateItem(
		context.Background(),
		vaultdomain.CreateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			Type:          vaultdomain.ItemTypeSecureNote,
			Envelope:      envelope,
			CorrelationID: "",
		},
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("CreateItem() error = %v, want %v", err, ErrDatabase)
	}

	assertNoVaultItemMutationRows(t)
}

func TestVaultStoreCreateItemPreservesCanceledContext(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fixture.store.CreateItem(ctx, vaultdomain.CreateItemStoreInput{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateItem() error = %v, want %v", err, context.Canceled)
	}
}

func TestVaultStoreCreateItemMapsMissingDatabaseSafely(t *testing.T) {
	store := NewVaultStore(nil)

	_, err := store.CreateItem(context.Background(), vaultdomain.CreateItemStoreInput{})

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("CreateItem() error = %v, want %v", err, ErrDatabase)
	}
}

func assertNoVaultItemMutationRows(t *testing.T) {
	t.Helper()

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		itemCount    int
		versionCount int
		auditCount   int
	)

	err := testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(SELECT count(*) FROM vault_items),
				(SELECT count(*) FROM item_versions),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE aggregate_type = 'vault_item'
				)
		`,
	).Scan(&itemCount, &versionCount, &auditCount)
	if err != nil {
		t.Fatalf("count vault item mutation rows: %v", err)
	}

	if itemCount != 0 {
		t.Fatalf("vault item count = %d, want 0", itemCount)
	}

	if versionCount != 0 {
		t.Fatalf("item version count = %d, want 0", versionCount)
	}

	if auditCount != 0 {
		t.Fatalf("vault item audit count = %d, want 0", auditCount)
	}
}
