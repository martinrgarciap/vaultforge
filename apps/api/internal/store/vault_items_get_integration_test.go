package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const vaultItemUnknownID = "00000000-0000-0000-0000-000000009999"

func TestVaultStoreGetItemReturnsOwnedActiveItem(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	storedItem, err := fixture.store.GetItem(
		context.Background(),
		vaultdomain.GetItemStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			ItemID:  createdItem.ID,
		},
	)
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}

	if storedItem.ID != createdItem.ID {
		t.Fatalf("item ID = %q, want %q", storedItem.ID, createdItem.ID)
	}

	if storedItem.VaultID != fixture.vaultID {
		t.Fatalf("vault ID = %q, want %q", storedItem.VaultID, fixture.vaultID)
	}

	if storedItem.Type != vaultdomain.ItemTypeSecureNote {
		t.Fatalf("item type = %q, want %q", storedItem.Type, vaultdomain.ItemTypeSecureNote)
	}

	if storedItem.Version != 1 {
		t.Fatalf("item version = %d, want 1", storedItem.Version)
	}

	if storedItem.Deleted() {
		t.Fatal("active item was marked as deleted")
	}

	const wantPayload = `{"label":"Retrieve Test","value":"synthetic-only"}`

	if string(storedItem.Payload) != wantPayload {
		t.Fatalf("item payload = %s, want %s", storedItem.Payload, wantPayload)
	}
}

func TestVaultStoreGetItemSeparatesActiveAndDeletedStates(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	softDeleteVaultItemForGetTest(t, createdItem.ID)

	_, err := fixture.store.GetItem(
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
		t.Fatalf("deleted GetItem() error = %v", err)
	}

	if deletedItem.ID != createdItem.ID {
		t.Fatalf("deleted item ID = %q, want %q", deletedItem.ID, createdItem.ID)
	}

	if !deletedItem.Deleted() {
		t.Fatal("deleted item was returned as active")
	}
}

func TestVaultStoreGetItemUsesSameSafeNotFoundForInaccessibleItems(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)
	createdItem := createVaultItemForGetTest(t, fixture)

	tests := []struct {
		name  string
		input vaultdomain.GetItemStoreInput
	}{
		{
			name: "other owner",
			input: vaultdomain.GetItemStoreInput{
				OwnerID: fixture.otherOwnerID,
				VaultID: fixture.vaultID,
				ItemID:  createdItem.ID,
			},
		},
		{
			name: "wrong parent vault",
			input: vaultdomain.GetItemStoreInput{
				OwnerID: fixture.ownerID,
				VaultID: fixture.otherVaultID,
				ItemID:  createdItem.ID,
			},
		},
		{
			name: "unknown item",
			input: vaultdomain.GetItemStoreInput{
				OwnerID: fixture.ownerID,
				VaultID: fixture.vaultID,
				ItemID:  vaultItemUnknownID,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := fixture.store.GetItem(context.Background(), test.input)

			if !errors.Is(err, vaultdomain.ErrItemNotFound) {
				t.Fatalf("GetItem() error = %v, want %v", err, vaultdomain.ErrItemNotFound)
			}
		})
	}
}

func TestVaultStoreGetItemRejectsInvalidState(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	_, err := fixture.store.GetItem(
		context.Background(),
		vaultdomain.GetItemStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			ItemID:  vaultItemUnknownID,
			State:   "all",
		},
	)

	if !errors.Is(err, vaultdomain.ErrItemListStateInvalid) {
		t.Fatalf("GetItem() error = %v, want %v", err, vaultdomain.ErrItemListStateInvalid)
	}
}

func TestVaultStoreGetItemPreservesCanceledContext(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fixture.store.GetItem(ctx, vaultdomain.GetItemStoreInput{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetItem() error = %v, want %v", err, context.Canceled)
	}
}

func TestVaultStoreGetItemMapsMissingDatabaseSafely(t *testing.T) {
	store := NewVaultStore(nil)

	_, err := store.GetItem(context.Background(), vaultdomain.GetItemStoreInput{})

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("GetItem() error = %v, want %v", err, ErrDatabase)
	}
}

func createVaultItemForGetTest(
	t *testing.T,
	fixture vaultItemIntegrationFixture,
) vaultdomain.Item {
	t.Helper()

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{
			"label": "Retrieve Test",
			"value": "synthetic-only"
		}`),
	)
	if err != nil {
		t.Fatalf("create retrieve test envelope: %v", err)
	}

	item, err := fixture.store.CreateItem(
		context.Background(),
		vaultdomain.CreateItemStoreInput{
			OwnerID:       fixture.ownerID,
			VaultID:       fixture.vaultID,
			Type:          vaultdomain.ItemTypeSecureNote,
			Envelope:      envelope,
			CorrelationID: "vault-item-get-integration-test",
		},
	)
	if err != nil {
		t.Fatalf("create retrieve test item: %v", err)
	}

	return item
}

func softDeleteVaultItemForGetTest(t *testing.T, itemID string) {
	t.Helper()

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	result, err := testDatabasePool.Exec(
		queryContext,
		`
			UPDATE vault_items
			SET
				updated_at = clock_timestamp(),
				deleted_at = clock_timestamp()
			WHERE id = $1::uuid
		`,
		itemID,
	)
	if err != nil {
		t.Fatalf("soft delete retrieve test item: %v", err)
	}

	if result.RowsAffected() != 1 {
		t.Fatalf("soft-deleted rows = %d, want 1", result.RowsAffected())
	}
}
