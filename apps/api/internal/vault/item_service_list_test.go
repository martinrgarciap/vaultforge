package vault

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const itemServiceTestSecondItemID = "00000000-0000-0000-0000-000000001004"

func TestServiceListItemsNormalizesOptionsAndReturnsStablePage(t *testing.T) {
	t.Parallel()

	newerTime := time.Date(2026, time.June, 22, 16, 1, 0, 0, time.UTC)
	olderTime := newerTime.Add(-time.Minute)

	firstItem := validItemServiceStoredItem(
		itemServiceTestItemID,
		newerTime,
	)
	secondItem := validItemServiceStoredItem(
		itemServiceTestSecondItemID,
		olderTime,
	)

	store := &itemServiceTestStore{
		listResult: ItemPage{
			Items: []Item{firstItem, secondItem},
		},
	}

	service := &Service{items: store}

	page, err := service.ListItems(
		context.Background(),
		ListItemsInput{
			OwnerID: itemServiceTestOwnerID,
			VaultID: itemServiceTestVaultID,
		},
	)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}

	if store.listCalls != 1 {
		t.Fatalf("ListItems() store calls = %d, want 1", store.listCalls)
	}

	if store.lastListInput.OwnerID != itemServiceTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			store.lastListInput.OwnerID,
			itemServiceTestOwnerID,
		)
	}

	if store.lastListInput.VaultID != itemServiceTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			store.lastListInput.VaultID,
			itemServiceTestVaultID,
		)
	}

	if store.lastListInput.Options.State != ItemListStateActive {
		t.Fatalf(
			"state = %q, want %q",
			store.lastListInput.Options.State,
			ItemListStateActive,
		)
	}

	if store.lastListInput.Options.Limit != DefaultItemPageLimit {
		t.Fatalf(
			"limit = %d, want %d",
			store.lastListInput.Options.Limit,
			DefaultItemPageLimit,
		)
	}

	if page.Items == nil {
		t.Fatal("item page contained a nil item slice")
	}

	if len(page.Items) != 2 {
		t.Fatalf("item count = %d, want 2", len(page.Items))
	}

	if page.Items[0].ID != itemServiceTestItemID {
		t.Fatalf("first item ID = %q, want %q", page.Items[0].ID, itemServiceTestItemID)
	}

	if page.Items[1].ID != itemServiceTestSecondItemID {
		t.Fatalf(
			"second item ID = %q, want %q",
			page.Items[1].ID,
			itemServiceTestSecondItemID,
		)
	}

	if page.NextCursor != nil {
		t.Fatal("item page unexpectedly contained a next cursor")
	}
}

func TestServiceListItemsReturnsValidatedNextCursor(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 22, 17, 0, 0, 0, time.UTC)
	item := validItemServiceStoredItem(itemServiceTestItemID, updatedAt)

	store := &itemServiceTestStore{
		listResult: ItemPage{
			Items: []Item{item},
			NextCursor: &ItemCursor{
				UpdatedAt: updatedAt,
				ID:        itemServiceTestItemID,
			},
		},
	}

	service := &Service{items: store}

	page, err := service.ListItems(
		context.Background(),
		ListItemsInput{
			OwnerID: itemServiceTestOwnerID,
			VaultID: itemServiceTestVaultID,
			Options: ItemListOptions{
				Limit: 1,
			},
		},
	)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}

	if page.NextCursor == nil {
		t.Fatal("item page did not contain a next cursor")
	}

	if page.NextCursor.ID != itemServiceTestItemID {
		t.Fatalf(
			"next cursor ID = %q, want %q",
			page.NextCursor.ID,
			itemServiceTestItemID,
		)
	}

	if !page.NextCursor.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"next cursor time = %v, want %v",
			page.NextCursor.UpdatedAt,
			updatedAt,
		)
	}
}

func TestServiceListItemsReturnsNonNilEmptyPage(t *testing.T) {
	t.Parallel()

	store := &itemServiceTestStore{
		listResult: ItemPage{Items: nil},
	}

	service := &Service{items: store}

	page, err := service.ListItems(
		context.Background(),
		ListItemsInput{
			OwnerID: itemServiceTestOwnerID,
			VaultID: itemServiceTestVaultID,
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
}

func TestServiceListItemsRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   ListItemsInput
		wantErr error
	}{
		{
			name: "invalid owner",
			input: ListItemsInput{
				VaultID: itemServiceTestVaultID,
			},
			wantErr: ErrOwnerInvalid,
		},
		{
			name: "invalid vault",
			input: ListItemsInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: " ",
			},
			wantErr: ErrVaultNotFound,
		},
		{
			name: "invalid state",
			input: ListItemsInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: itemServiceTestVaultID,
				Options: ItemListOptions{State: "all"},
			},
			wantErr: ErrItemListStateInvalid,
		},
		{
			name: "invalid limit",
			input: ListItemsInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: itemServiceTestVaultID,
				Options: ItemListOptions{Limit: MaxItemPageLimit + 1},
			},
			wantErr: ErrItemPageLimitInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{}
			service := &Service{items: store}

			_, err := service.ListItems(context.Background(), test.input)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ListItems() error = %v, want %v", err, test.wantErr)
			}

			if store.listCalls != 0 {
				t.Fatal("item store was called for invalid list input")
			}
		})
	}
}

func TestServiceListItemsMapsStoreErrorsSafely(t *testing.T) {
	t.Parallel()

	const internalMarker = "synthetic list database failure"

	tests := []struct {
		name     string
		storeErr error
		wantErr  error
	}{
		{
			name:     "vault not found",
			storeErr: ErrVaultNotFound,
			wantErr:  ErrVaultNotFound,
		},
		{
			name:     "context canceled",
			storeErr: context.Canceled,
			wantErr:  context.Canceled,
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

			store := &itemServiceTestStore{listErr: test.storeErr}
			service := &Service{items: store}

			_, err := service.ListItems(
				context.Background(),
				ListItemsInput{
					OwnerID: itemServiceTestOwnerID,
					VaultID: itemServiceTestVaultID,
				},
			)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ListItems() error = %v, want %v", err, test.wantErr)
			}

			if strings.Contains(err.Error(), internalMarker) {
				t.Fatal("ListItems() exposed an internal store failure")
			}
		})
	}
}

func TestServiceListItemsRejectsMalformedStoredPages(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 22, 18, 0, 0, 0, time.UTC)
	validItem := validItemServiceStoredItem(itemServiceTestItemID, updatedAt)

	tests := []struct {
		name string
		page ItemPage
	}{
		{
			name: "wrong vault",
			page: ItemPage{
				Items: []Item{{
					ID:        itemServiceTestItemID,
					VaultID:   "00000000-0000-0000-0000-000000001999",
					Type:      ItemTypeSecureNote,
					Payload:   json.RawMessage(`{"value":"synthetic"}`),
					Version:   1,
					CreatedAt: updatedAt,
					UpdatedAt: updatedAt,
				}},
			},
		},
		{
			name: "too many items",
			page: ItemPage{
				Items: []Item{
					validItem,
					validItemServiceStoredItem(itemServiceTestSecondItemID, updatedAt.Add(-time.Minute)),
				},
			},
		},
		{
			name: "invalid ordering",
			page: ItemPage{
				Items: []Item{
					validItemServiceStoredItem(itemServiceTestItemID, updatedAt.Add(-time.Minute)),
					validItemServiceStoredItem(itemServiceTestSecondItemID, updatedAt),
				},
			},
		},
		{
			name: "cursor does not match final item",
			page: ItemPage{
				Items: []Item{validItem},
				NextCursor: &ItemCursor{
					UpdatedAt: updatedAt,
					ID:        itemServiceTestSecondItemID,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{listResult: test.page}
			service := &Service{items: store}

			_, err := service.ListItems(
				context.Background(),
				ListItemsInput{
					OwnerID: itemServiceTestOwnerID,
					VaultID: itemServiceTestVaultID,
					Options: ItemListOptions{Limit: 1},
				},
			)

			if !errors.Is(err, ErrItemUnavailable) {
				t.Fatalf("ListItems() error = %v, want %v", err, ErrItemUnavailable)
			}
		})
	}
}

func TestServiceListItemsRejectsUnavailableDependency(t *testing.T) {
	t.Parallel()

	service := &Service{}

	_, err := service.ListItems(
		context.Background(),
		ListItemsInput{
			OwnerID: itemServiceTestOwnerID,
			VaultID: itemServiceTestVaultID,
		},
	)

	if !errors.Is(err, ErrItemUnavailable) {
		t.Fatalf("ListItems() error = %v, want %v", err, ErrItemUnavailable)
	}
}

func TestServiceListItemsPreservesCanceledContext(t *testing.T) {
	t.Parallel()

	store := &itemServiceTestStore{}
	service := &Service{items: store}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.ListItems(
		ctx,
		ListItemsInput{
			OwnerID: itemServiceTestOwnerID,
			VaultID: itemServiceTestVaultID,
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListItems() error = %v, want %v", err, context.Canceled)
	}

	if store.listCalls != 0 {
		t.Fatal("item store was called after context cancellation")
	}
}

func validItemServiceStoredItem(itemID string, updatedAt time.Time) Item {
	return Item{
		ID:        itemID,
		VaultID:   itemServiceTestVaultID,
		Type:      ItemTypeSecureNote,
		Payload:   json.RawMessage(`{"value":"synthetic"}`),
		Version:   1,
		CreatedAt: updatedAt.Add(-time.Minute),
		UpdatedAt: updatedAt,
	}
}
