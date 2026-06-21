package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const (
	vaultItemListID1 = "00000000-0000-0000-0000-000000000901"
	vaultItemListID2 = "00000000-0000-0000-0000-000000000902"
	vaultItemListID3 = "00000000-0000-0000-0000-000000000903"
	vaultItemListID4 = "00000000-0000-0000-0000-000000000904"
	vaultItemListID5 = "00000000-0000-0000-0000-000000000905"
	vaultItemListID6 = "00000000-0000-0000-0000-000000000906"
)

func TestVaultStoreListItemsUsesStableKeysetPagination(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	baseTime := time.Date(2026, time.June, 22, 12, 0, 0, 0, time.UTC)

	insertVaultItemForListTest(
		t,
		fixture.vaultID,
		vaultItemListID4,
		baseTime.Add(3*time.Minute),
		nil,
	)
	insertVaultItemForListTest(
		t,
		fixture.vaultID,
		vaultItemListID3,
		baseTime.Add(2*time.Minute),
		nil,
	)
	insertVaultItemForListTest(
		t,
		fixture.vaultID,
		vaultItemListID2,
		baseTime.Add(2*time.Minute),
		nil,
	)
	insertVaultItemForListTest(
		t,
		fixture.vaultID,
		vaultItemListID1,
		baseTime.Add(time.Minute),
		nil,
	)

	insertVaultItemForListTest(
		t,
		fixture.otherVaultID,
		vaultItemListID5,
		baseTime.Add(4*time.Minute),
		nil,
	)

	deletedAt := baseTime.Add(5 * time.Minute)

	insertVaultItemForListTest(
		t,
		fixture.vaultID,
		vaultItemListID6,
		deletedAt,
		&deletedAt,
	)

	firstPage, err := fixture.store.ListItems(
		context.Background(),
		vaultdomain.ListItemsStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			Options: vaultdomain.ItemListOptions{
				Limit: 2,
			},
		},
	)
	if err != nil {
		t.Fatalf("ListItems() first page error = %v", err)
	}

	if len(firstPage.Items) != 2 {
		t.Fatalf("first page item count = %d, want 2", len(firstPage.Items))
	}

	if firstPage.Items[0].ID != vaultItemListID4 {
		t.Fatalf("first item ID = %q, want %q", firstPage.Items[0].ID, vaultItemListID4)
	}

	if firstPage.Items[1].ID != vaultItemListID3 {
		t.Fatalf("second item ID = %q, want %q", firstPage.Items[1].ID, vaultItemListID3)
	}

	if firstPage.NextCursor == nil {
		t.Fatal("first page did not include a next cursor")
	}

	if firstPage.NextCursor.ID != vaultItemListID3 {
		t.Fatalf(
			"next cursor ID = %q, want %q",
			firstPage.NextCursor.ID,
			vaultItemListID3,
		)
	}

	if !firstPage.NextCursor.UpdatedAt.Equal(baseTime.Add(2 * time.Minute)) {
		t.Fatalf(
			"next cursor time = %v, want %v",
			firstPage.NextCursor.UpdatedAt,
			baseTime.Add(2*time.Minute),
		)
	}

	secondPage, err := fixture.store.ListItems(
		context.Background(),
		vaultdomain.ListItemsStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			Options: vaultdomain.ItemListOptions{
				Limit: 2,
				After: firstPage.NextCursor,
			},
		},
	)
	if err != nil {
		t.Fatalf("ListItems() second page error = %v", err)
	}

	if len(secondPage.Items) != 2 {
		t.Fatalf("second page item count = %d, want 2", len(secondPage.Items))
	}

	if secondPage.Items[0].ID != vaultItemListID2 {
		t.Fatalf("third item ID = %q, want %q", secondPage.Items[0].ID, vaultItemListID2)
	}

	if secondPage.Items[1].ID != vaultItemListID1 {
		t.Fatalf("fourth item ID = %q, want %q", secondPage.Items[1].ID, vaultItemListID1)
	}

	if secondPage.NextCursor != nil {
		t.Fatal("final page unexpectedly included a next cursor")
	}

	for _, item := range append(firstPage.Items, secondPage.Items...) {
		if item.Deleted() {
			t.Fatalf("active listing returned deleted item %q", item.ID)
		}

		if item.VaultID != fixture.vaultID {
			t.Fatalf("listed vault ID = %q, want %q", item.VaultID, fixture.vaultID)
		}
	}
}

func TestVaultStoreListItemsReturnsDeletedItemsOnly(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	updatedAt := time.Date(2026, time.June, 22, 13, 0, 0, 0, time.UTC)
	deletedAt := updatedAt.Add(time.Minute)

	insertVaultItemForListTest(
		t,
		fixture.vaultID,
		vaultItemListID1,
		updatedAt,
		nil,
	)
	insertVaultItemForListTest(
		t,
		fixture.vaultID,
		vaultItemListID2,
		deletedAt,
		&deletedAt,
	)

	page, err := fixture.store.ListItems(
		context.Background(),
		vaultdomain.ListItemsStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
			Options: vaultdomain.ItemListOptions{
				State: vaultdomain.ItemListStateDeleted,
			},
		},
	)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}

	if len(page.Items) != 1 {
		t.Fatalf("deleted item count = %d, want 1", len(page.Items))
	}

	if page.Items[0].ID != vaultItemListID2 {
		t.Fatalf("deleted item ID = %q, want %q", page.Items[0].ID, vaultItemListID2)
	}

	if !page.Items[0].Deleted() {
		t.Fatal("deleted listing returned an active item")
	}
}

func TestVaultStoreListItemsReturnsNonNilEmptyPage(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	page, err := fixture.store.ListItems(
		context.Background(),
		vaultdomain.ListItemsStoreInput{
			OwnerID: fixture.ownerID,
			VaultID: fixture.vaultID,
		},
	)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}

	if page.Items == nil {
		t.Fatal("empty item page contained a nil item slice")
	}

	if len(page.Items) != 0 {
		t.Fatalf("item count = %d, want 0", len(page.Items))
	}

	if page.NextCursor != nil {
		t.Fatal("empty item page unexpectedly contained a next cursor")
	}
}

func TestVaultStoreListItemsUsesSafeNotFoundForOtherOwner(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	_, err := fixture.store.ListItems(
		context.Background(),
		vaultdomain.ListItemsStoreInput{
			OwnerID: fixture.otherOwnerID,
			VaultID: fixture.vaultID,
		},
	)

	if !errors.Is(err, vaultdomain.ErrVaultNotFound) {
		t.Fatalf("ListItems() error = %v, want %v", err, vaultdomain.ErrVaultNotFound)
	}
}

func TestVaultStoreListItemsPreservesCanceledContext(t *testing.T) {
	fixture := newVaultItemIntegrationFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fixture.store.ListItems(ctx, vaultdomain.ListItemsStoreInput{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListItems() error = %v, want %v", err, context.Canceled)
	}
}

func TestVaultStoreListItemsMapsMissingDatabaseSafely(t *testing.T) {
	store := NewVaultStore(nil)

	_, err := store.ListItems(context.Background(), vaultdomain.ListItemsStoreInput{})

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf("ListItems() error = %v, want %v", err, ErrDatabase)
	}
}

func insertVaultItemForListTest(
	t *testing.T,
	vaultID string,
	itemID string,
	updatedAt time.Time,
	deletedAt *time.Time,
) {
	t.Helper()

	payload, err := json.Marshal(map[string]string{
		"itemID": itemID,
	})
	if err != nil {
		t.Fatalf("marshal list test payload: %v", err)
	}

	envelope, err := vaultdomain.NewSyntheticItemEnvelope(payload)
	if err != nil {
		t.Fatalf("create list test envelope: %v", err)
	}

	createdAt := updatedAt.Add(-time.Hour)

	queryContext, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	transaction, err := testDatabasePool.Begin(queryContext)
	if err != nil {
		t.Fatalf("begin list test item transaction: %v", err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(queryContext)
		}
	}()

	_, err = transaction.Exec(
		queryContext,
		`
			INSERT INTO vault_items (
				id,
				vault_id,
				item_type,
				encrypted_payload,
				nonce,
				version,
				created_at,
				updated_at,
				deleted_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				'secure_note',
				$3::bytea,
				$4::bytea,
				1,
				$5,
				$6,
				$7
			)
		`,
		itemID,
		vaultID,
		envelope.Payload,
		envelope.Nonce,
		createdAt,
		updatedAt,
		deletedAt,
	)
	if err != nil {
		t.Fatalf("insert list test vault item: %v", err)
	}

	_, err = transaction.Exec(
		queryContext,
		`
			INSERT INTO item_versions (
				vault_item_id,
				version,
				item_type,
				encrypted_payload,
				nonce,
				created_at
			)
			VALUES (
				$1::uuid,
				1,
				'secure_note',
				$2::bytea,
				$3::bytea,
				$4
			)
		`,
		itemID,
		envelope.Payload,
		envelope.Nonce,
		createdAt,
	)
	if err != nil {
		t.Fatalf("insert list test item version: %v", err)
	}

	if err := transaction.Commit(queryContext); err != nil {
		t.Fatalf("commit list test item transaction: %v", err)
	}

	committed = true
}
