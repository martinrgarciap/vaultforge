package vault

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceGetItemReturnsOwnedActiveItem(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 22, 19, 0, 0, 0, time.UTC)

	store := &itemServiceTestStore{
		getResult: itemServiceGetStoredItem(
			itemServiceTestItemID,
			itemServiceTestVaultID,
			updatedAt,
			nil,
		),
	}

	service := &Service{items: store}

	item, err := service.GetItem(
		context.Background(),
		GetItemInput{
			OwnerID: itemServiceTestOwnerID,
			VaultID: itemServiceTestVaultID,
			ItemID:  itemServiceTestItemID,
		},
	)
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}

	if store.getCalls != 1 {
		t.Fatalf("GetItem() store calls = %d, want 1", store.getCalls)
	}

	if store.lastGetInput.OwnerID != itemServiceTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			store.lastGetInput.OwnerID,
			itemServiceTestOwnerID,
		)
	}

	if store.lastGetInput.VaultID != itemServiceTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			store.lastGetInput.VaultID,
			itemServiceTestVaultID,
		)
	}

	if store.lastGetInput.ItemID != itemServiceTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			store.lastGetInput.ItemID,
			itemServiceTestItemID,
		)
	}

	if store.lastGetInput.State != ItemListStateActive {
		t.Fatalf(
			"state = %q, want %q",
			store.lastGetInput.State,
			ItemListStateActive,
		)
	}

	if item.ID != itemServiceTestItemID {
		t.Fatalf("returned item ID = %q, want %q", item.ID, itemServiceTestItemID)
	}

	if item.Deleted() {
		t.Fatal("active item was returned as deleted")
	}
}

func TestServiceGetItemReturnsDeletedItem(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 22, 20, 0, 0, 0, time.UTC)
	deletedAt := updatedAt

	store := &itemServiceTestStore{
		getResult: itemServiceGetStoredItem(
			itemServiceTestItemID,
			itemServiceTestVaultID,
			updatedAt,
			&deletedAt,
		),
	}

	service := &Service{items: store}

	item, err := service.GetItem(
		context.Background(),
		GetItemInput{
			OwnerID: itemServiceTestOwnerID,
			VaultID: itemServiceTestVaultID,
			ItemID:  itemServiceTestItemID,
			State:   ItemListStateDeleted,
		},
	)
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}

	if store.lastGetInput.State != ItemListStateDeleted {
		t.Fatalf(
			"state = %q, want %q",
			store.lastGetInput.State,
			ItemListStateDeleted,
		)
	}

	if !item.Deleted() {
		t.Fatal("deleted item was returned as active")
	}
}

func TestServiceGetItemRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   GetItemInput
		wantErr error
	}{
		{
			name: "invalid owner",
			input: GetItemInput{
				VaultID: itemServiceTestVaultID,
				ItemID:  itemServiceTestItemID,
			},
			wantErr: ErrOwnerInvalid,
		},
		{
			name: "invalid vault",
			input: GetItemInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: " ",
				ItemID:  itemServiceTestItemID,
			},
			wantErr: ErrVaultNotFound,
		},
		{
			name: "invalid item",
			input: GetItemInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: itemServiceTestVaultID,
				ItemID:  "",
			},
			wantErr: ErrItemNotFound,
		},
		{
			name: "invalid state",
			input: GetItemInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: itemServiceTestVaultID,
				ItemID:  itemServiceTestItemID,
				State:   "all",
			},
			wantErr: ErrItemListStateInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{}
			service := &Service{items: store}

			_, err := service.GetItem(context.Background(), test.input)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("GetItem() error = %v, want %v", err, test.wantErr)
			}

			if store.getCalls != 0 {
				t.Fatal("item store was called for invalid get input")
			}
		})
	}
}

func TestServiceGetItemMapsStoreErrorsSafely(t *testing.T) {
	t.Parallel()

	const internalMarker = "synthetic get database failure"

	tests := []struct {
		name     string
		storeErr error
		wantErr  error
	}{
		{
			name:     "item not found",
			storeErr: ErrItemNotFound,
			wantErr:  ErrItemNotFound,
		},
		{
			name:     "context deadline",
			storeErr: context.DeadlineExceeded,
			wantErr:  context.DeadlineExceeded,
		},
		{
			name:     "internal failure",
			storeErr: errors.New(internalMarker),
			wantErr:  ErrItemUnavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{getErr: test.storeErr}
			service := &Service{items: store}

			_, err := service.GetItem(
				context.Background(),
				validItemServiceGetInput(),
			)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("GetItem() error = %v, want %v", err, test.wantErr)
			}

			if strings.Contains(err.Error(), internalMarker) {
				t.Fatal("GetItem() exposed an internal store failure")
			}
		})
	}
}

func TestServiceGetItemRejectsMalformedStoredItems(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 22, 21, 0, 0, 0, time.UTC)
	deletedAt := updatedAt

	tests := []struct {
		name  string
		item  Item
		input GetItemInput
	}{
		{
			name: "wrong item ID",
			item: itemServiceGetStoredItem(
				itemServiceTestSecondItemID,
				itemServiceTestVaultID,
				updatedAt,
				nil,
			),
			input: validItemServiceGetInput(),
		},
		{
			name: "wrong vault ID",
			item: itemServiceGetStoredItem(
				itemServiceTestItemID,
				"00000000-0000-0000-0000-000000001999",
				updatedAt,
				nil,
			),
			input: validItemServiceGetInput(),
		},
		{
			name: "deleted item returned for active lookup",
			item: itemServiceGetStoredItem(
				itemServiceTestItemID,
				itemServiceTestVaultID,
				updatedAt,
				&deletedAt,
			),
			input: validItemServiceGetInput(),
		},
		{
			name: "active item returned for deleted lookup",
			item: itemServiceGetStoredItem(
				itemServiceTestItemID,
				itemServiceTestVaultID,
				updatedAt,
				nil,
			),
			input: GetItemInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: itemServiceTestVaultID,
				ItemID:  itemServiceTestItemID,
				State:   ItemListStateDeleted,
			},
		},
		{
			name: "invalid payload",
			item: Item{
				ID:        itemServiceTestItemID,
				VaultID:   itemServiceTestVaultID,
				Type:      ItemTypeSecureNote,
				Payload:   json.RawMessage(`["not-an-object"]`),
				Version:   1,
				CreatedAt: updatedAt.Add(-time.Minute),
				UpdatedAt: updatedAt,
			},
			input: validItemServiceGetInput(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{getResult: test.item}
			service := &Service{items: store}

			_, err := service.GetItem(context.Background(), test.input)

			if !errors.Is(err, ErrItemUnavailable) {
				t.Fatalf("GetItem() error = %v, want %v", err, ErrItemUnavailable)
			}
		})
	}
}

func TestServiceGetItemRejectsUnavailableDependency(t *testing.T) {
	t.Parallel()

	service := &Service{}

	_, err := service.GetItem(context.Background(), validItemServiceGetInput())

	if !errors.Is(err, ErrItemUnavailable) {
		t.Fatalf("GetItem() error = %v, want %v", err, ErrItemUnavailable)
	}
}

func TestServiceGetItemPreservesCanceledContext(t *testing.T) {
	t.Parallel()

	store := &itemServiceTestStore{}
	service := &Service{items: store}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.GetItem(ctx, validItemServiceGetInput())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetItem() error = %v, want %v", err, context.Canceled)
	}

	if store.getCalls != 0 {
		t.Fatal("item store was called after context cancellation")
	}
}

func validItemServiceGetInput() GetItemInput {
	return GetItemInput{
		OwnerID: itemServiceTestOwnerID,
		VaultID: itemServiceTestVaultID,
		ItemID:  itemServiceTestItemID,
	}
}

func itemServiceGetStoredItem(
	itemID string,
	vaultID string,
	updatedAt time.Time,
	deletedAt *time.Time,
) Item {
	return Item{
		ID:        itemID,
		VaultID:   vaultID,
		Type:      ItemTypeSecureNote,
		Payload:   json.RawMessage(`{"value":"synthetic"}`),
		Version:   1,
		CreatedAt: updatedAt.Add(-time.Minute),
		UpdatedAt: updatedAt,
		DeletedAt: deletedAt,
	}
}
