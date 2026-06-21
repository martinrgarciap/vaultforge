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
			Idempotency: mustItemCreateIdempotency(
				t,
				"vault-item-create-persist",
				vaultdomain.ItemTypeAPIKey,
				envelope,
			),
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

	assertVaultItemAuditPayload(
		t,
		sanitizedPayload,
		createdItem.VaultID,
		createdItem.Type,
		createdItem.Version,
		string(envelope.Payload),
		string(envelope.Nonce),
	)
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
			Idempotency: mustItemCreateIdempotency(
				t,
				"vault-item-create-other-owner",
				vaultdomain.ItemTypeSecureNote,
				envelope,
			),
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
			Idempotency: mustItemCreateIdempotency(
				t,
				"vault-item-create-audit-rollback",
				vaultdomain.ItemTypeSecureNote,
				envelope,
			),
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
		itemCount        int
		versionCount     int
		auditCount       int
		idempotencyCount int
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
				),
				(SELECT count(*) FROM idempotency_records)
		`,
	).Scan(
		&itemCount,
		&versionCount,
		&auditCount,
		&idempotencyCount,
	)
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

	if idempotencyCount != 0 {
		t.Fatalf("idempotency record count = %d, want 0", idempotencyCount)
	}
}

func TestVaultStoreCreateItemReplaysExistingResult(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{"label":"Development","value":"synthetic-only"}`),
	)
	if err != nil {
		t.Fatalf("create synthetic item envelope: %v", err)
	}

	idempotency := mustItemCreateIdempotency(
		t,
		"vault-item-create-replay",
		vaultdomain.ItemTypeAPIKey,
		envelope,
	)

	firstItem, err := fixture.store.CreateItem(
		context.Background(),
		vaultdomain.CreateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			Type:          vaultdomain.ItemTypeAPIKey,
			Envelope:      envelope,
			Idempotency:   idempotency,
			CorrelationID: "vault-item-create-replay-first",
		},
	)
	if err != nil {
		t.Fatalf("first CreateItem() error = %v", err)
	}

	replayedItem, err := fixture.store.CreateItem(
		context.Background(),
		vaultdomain.CreateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			Type:          vaultdomain.ItemTypeAPIKey,
			Envelope:      envelope,
			Idempotency:   idempotency,
			CorrelationID: "vault-item-create-replay-second",
		},
	)
	if err != nil {
		t.Fatalf("replayed CreateItem() error = %v", err)
	}

	if replayedItem.ID != firstItem.ID {
		t.Fatalf("replayed item ID = %q, want %q", replayedItem.ID, firstItem.ID)
	}

	if replayedItem.Version != 1 {
		t.Fatalf("replayed item version = %d, want 1", replayedItem.Version)
	}

	if replayedItem.Type != firstItem.Type {
		t.Fatalf("replayed item type = %q, want %q", replayedItem.Type, firstItem.Type)
	}

	if !bytes.Equal(replayedItem.Payload, firstItem.Payload) {
		t.Fatal("replayed item payload did not match the original result")
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		itemCount        int
		versionCount     int
		auditCount       int
		idempotencyCount int
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
					  AND event_type = 'vault_item.created'
				),
				(
					SELECT count(*)
					FROM idempotency_records
					WHERE actor_id = $2::uuid
					  AND operation = $3
					  AND scope_id = $4::uuid
					  AND idempotency_key_hash = $5::bytea
				)
		`,
		firstItem.ID,
		fixture.ownerID,
		vaultdomain.ItemCreateOperation,
		fixture.vaultID,
		idempotency.KeyHash[:],
	).Scan(
		&itemCount,
		&versionCount,
		&auditCount,
		&idempotencyCount,
	)
	if err != nil {
		t.Fatalf("count replay rows: %v", err)
	}

	if itemCount != 1 {
		t.Fatalf("vault item count = %d, want 1", itemCount)
	}

	if versionCount != 1 {
		t.Fatalf("item version count = %d, want 1", versionCount)
	}

	if auditCount != 1 {
		t.Fatalf("created audit count = %d, want 1", auditCount)
	}

	if idempotencyCount != 1 {
		t.Fatalf("idempotency record count = %d, want 1", idempotencyCount)
	}
}

func TestVaultStoreCreateItemRejectsReusedKeyWithDifferentInput(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	firstEnvelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{"value":"first"}`),
	)
	if err != nil {
		t.Fatalf("create first envelope: %v", err)
	}

	secondEnvelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{"value":"second"}`),
	)
	if err != nil {
		t.Fatalf("create second envelope: %v", err)
	}

	const idempotencyKey = "vault-item-create-conflict"

	firstIdempotency := mustItemCreateIdempotency(
		t,
		idempotencyKey,
		vaultdomain.ItemTypeSecureNote,
		firstEnvelope,
	)

	firstItem, err := fixture.store.CreateItem(
		context.Background(),
		vaultdomain.CreateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			Type:          vaultdomain.ItemTypeSecureNote,
			Envelope:      firstEnvelope,
			Idempotency:   firstIdempotency,
			CorrelationID: "vault-item-create-conflict-first",
		},
	)
	if err != nil {
		t.Fatalf("first CreateItem() error = %v", err)
	}

	secondIdempotency := mustItemCreateIdempotency(
		t,
		idempotencyKey,
		vaultdomain.ItemTypeSecureNote,
		secondEnvelope,
	)

	_, err = fixture.store.CreateItem(
		context.Background(),
		vaultdomain.CreateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			Type:          vaultdomain.ItemTypeSecureNote,
			Envelope:      secondEnvelope,
			Idempotency:   secondIdempotency,
			CorrelationID: "vault-item-create-conflict-second",
		},
	)
	if !errors.Is(err, vaultdomain.ErrItemIdempotencyConflict) {
		t.Fatalf(
			"reused-key CreateItem() error = %v, want %v",
			err,
			vaultdomain.ErrItemIdempotencyConflict,
		)
	}

	storedItem, err := fixture.store.GetItem(
		context.Background(),
		vaultdomain.GetItemStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			ItemID:  firstItem.ID,
		},
	)
	if err != nil {
		t.Fatalf("read item after idempotency conflict: %v", err)
	}

	if !bytes.Equal(storedItem.Payload, firstEnvelope.Payload) {
		t.Fatal("idempotency conflict changed the original item")
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		itemCount        int
		versionCount     int
		auditCount       int
		idempotencyCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(SELECT count(*) FROM vault_items),
				(SELECT count(*) FROM item_versions),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE aggregate_type = 'vault_item'
				),
				(SELECT count(*) FROM idempotency_records)
		`,
	).Scan(
		&itemCount,
		&versionCount,
		&auditCount,
		&idempotencyCount,
	)
	if err != nil {
		t.Fatalf("count idempotency conflict rows: %v", err)
	}

	if itemCount != 1 {
		t.Fatalf("vault item count = %d, want 1", itemCount)
	}

	if versionCount != 1 {
		t.Fatalf("item version count = %d, want 1", versionCount)
	}

	if auditCount != 1 {
		t.Fatalf("vault item audit count = %d, want 1", auditCount)
	}

	if idempotencyCount != 1 {
		t.Fatalf("idempotency record count = %d, want 1", idempotencyCount)
	}
}

func TestVaultStoreCreateItemReplaysConcurrentDuplicateRequests(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{"label":"Concurrent","value":"synthetic-only"}`),
	)
	if err != nil {
		t.Fatalf("create synthetic item envelope: %v", err)
	}

	idempotency := mustItemCreateIdempotency(
		t,
		"vault-item-create-concurrent-replay",
		vaultdomain.ItemTypeSecureNote,
		envelope,
	)

	type createAttempt struct {
		item vaultdomain.Item
		err  error
	}

	correlationIDs := []string{
		"vault-item-create-concurrent-first",
		"vault-item-create-concurrent-second",
	}

	ready := make(chan struct{}, len(correlationIDs))
	start := make(chan struct{})
	results := make(chan createAttempt, len(correlationIDs))

	for _, correlationID := range correlationIDs {
		correlationID := correlationID

		go func() {
			ready <- struct{}{}
			<-start

			item, createErr := fixture.store.CreateItem(
				context.Background(),
				vaultdomain.CreateItemStoreInput{
					OwnerID:       fixture.ownerID,
					VaultID:       fixture.vaultID,
					Type:          vaultdomain.ItemTypeSecureNote,
					Envelope:      envelope,
					Idempotency:   idempotency,
					CorrelationID: correlationID,
				},
			)

			results <- createAttempt{
				item: item,
				err:  createErr,
			}
		}()
	}

	for range correlationIDs {
		<-ready
	}

	close(start)

	createdItems := make([]vaultdomain.Item, 0, len(correlationIDs))

	for range correlationIDs {
		result := <-results

		if result.err != nil {
			t.Fatalf("concurrent CreateItem() error = %v", result.err)
		}

		createdItems = append(createdItems, result.item)
	}

	if createdItems[0].ID == "" {
		t.Fatal("concurrent create result did not contain an item ID")
	}

	if createdItems[1].ID != createdItems[0].ID {
		t.Fatalf(
			"concurrent item IDs = %q and %q, want identical IDs",
			createdItems[0].ID,
			createdItems[1].ID,
		)
	}

	for index, item := range createdItems {
		if item.VaultID != fixture.vaultID {
			t.Fatalf(
				"result %d vault ID = %q, want %q",
				index,
				item.VaultID,
				fixture.vaultID,
			)
		}

		if item.Type != vaultdomain.ItemTypeSecureNote {
			t.Fatalf(
				"result %d item type = %q, want %q",
				index,
				item.Type,
				vaultdomain.ItemTypeSecureNote,
			)
		}

		if item.Version != 1 {
			t.Fatalf("result %d version = %d, want 1", index, item.Version)
		}

		if !bytes.Equal(item.Payload, envelope.Payload) {
			t.Fatalf("result %d payload did not match the original request", index)
		}
	}

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	var (
		itemCount        int
		versionCount     int
		auditCount       int
		idempotencyCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT count(*)
					FROM vault_items
					WHERE vault_id = $1::uuid
				),
				(
					SELECT count(*)
					FROM item_versions AS versions
					JOIN vault_items AS items
					  ON items.id = versions.vault_item_id
					WHERE items.vault_id = $1::uuid
				),
				(
					SELECT count(*)
					FROM audit_outbox AS outbox
					JOIN vault_items AS items
					  ON items.id = outbox.aggregate_id
					WHERE outbox.aggregate_type = 'vault_item'
					  AND outbox.event_type = 'vault_item.created'
					  AND items.vault_id = $1::uuid
				),
				(
					SELECT count(*)
					FROM idempotency_records
					WHERE actor_id = $2::uuid
					  AND operation = $3
					  AND scope_id = $1::uuid
					  AND idempotency_key_hash = $4::bytea
				)
		`,
		fixture.vaultID,
		fixture.ownerID,
		vaultdomain.ItemCreateOperation,
		idempotency.KeyHash[:],
	).Scan(
		&itemCount,
		&versionCount,
		&auditCount,
		&idempotencyCount,
	)
	if err != nil {
		t.Fatalf("count concurrent create rows: %v", err)
	}

	if itemCount != 1 {
		t.Fatalf("vault item count = %d, want 1", itemCount)
	}

	if versionCount != 1 {
		t.Fatalf("item version count = %d, want 1", versionCount)
	}

	if auditCount != 1 {
		t.Fatalf("created audit count = %d, want 1", auditCount)
	}

	if idempotencyCount != 1 {
		t.Fatalf("idempotency record count = %d, want 1", idempotencyCount)
	}
}

func mustItemCreateIdempotency(
	t *testing.T,
	key string,
	itemType vaultdomain.ItemType,
	envelope vaultdomain.SyntheticItemEnvelope,
) vaultdomain.ItemCreateIdempotency {
	t.Helper()

	idempotency, err := vaultdomain.NewItemCreateIdempotency(
		key,
		itemType,
		envelope,
	)
	if err != nil {
		t.Fatalf("create item idempotency value: %v", err)
	}

	return idempotency
}
