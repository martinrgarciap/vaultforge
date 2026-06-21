package vault

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const itemServiceRestoreRequest = "item-service-restore-request"

func TestServiceRestoreItemRestoresOwnedItem(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 23, 2, 0, 0, 0, time.UTC)

	store := &itemServiceTestStore{
		restoreResult: Item{
			ID:        itemServiceTestItemID,
			VaultID:   itemServiceTestVaultID,
			Type:      ItemTypeSecureNote,
			Payload:   json.RawMessage(`{"value":"synthetic"}`),
			Version:   2,
			CreatedAt: updatedAt.Add(-time.Hour),
			UpdatedAt: updatedAt,
		},
	}

	service := &Service{items: store}

	restoredItem, err := service.RestoreItem(
		context.Background(),
		validItemServiceRestoreInput(),
	)
	if err != nil {
		t.Fatalf("RestoreItem() error = %v", err)
	}

	if store.restoreCalls != 1 {
		t.Fatalf("RestoreItem() store calls = %d, want 1", store.restoreCalls)
	}

	if store.lastRestoreInput.OwnerID != itemServiceTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			store.lastRestoreInput.OwnerID,
			itemServiceTestOwnerID,
		)
	}

	if store.lastRestoreInput.VaultID != itemServiceTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			store.lastRestoreInput.VaultID,
			itemServiceTestVaultID,
		)
	}

	if store.lastRestoreInput.ItemID != itemServiceTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			store.lastRestoreInput.ItemID,
			itemServiceTestItemID,
		)
	}

	if store.lastRestoreInput.CorrelationID != itemServiceRestoreRequest {
		t.Fatalf(
			"correlation ID = %q, want %q",
			store.lastRestoreInput.CorrelationID,
			itemServiceRestoreRequest,
		)
	}

	if restoredItem.ID != itemServiceTestItemID {
		t.Fatalf(
			"restored item ID = %q, want %q",
			restoredItem.ID,
			itemServiceTestItemID,
		)
	}

	if restoredItem.Deleted() {
		t.Fatal("restored item was returned as deleted")
	}

	if restoredItem.Version != 2 {
		t.Fatalf("restored version = %d, want 2", restoredItem.Version)
	}
}

func TestServiceRestoreItemRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   RestoreItemInput
		wantErr error
	}{
		{
			name: "invalid owner",
			input: RestoreItemInput{
				VaultID:       itemServiceTestVaultID,
				ItemID:        itemServiceTestItemID,
				CorrelationID: itemServiceRestoreRequest,
			},
			wantErr: ErrOwnerInvalid,
		},
		{
			name: "invalid vault",
			input: RestoreItemInput{
				OwnerID:       itemServiceTestOwnerID,
				VaultID:       " ",
				ItemID:        itemServiceTestItemID,
				CorrelationID: itemServiceRestoreRequest,
			},
			wantErr: ErrVaultNotFound,
		},
		{
			name: "invalid item",
			input: RestoreItemInput{
				OwnerID:       itemServiceTestOwnerID,
				VaultID:       itemServiceTestVaultID,
				ItemID:        "",
				CorrelationID: itemServiceRestoreRequest,
			},
			wantErr: ErrItemNotFound,
		},
		{
			name: "invalid correlation ID",
			input: RestoreItemInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: itemServiceTestVaultID,
				ItemID:  itemServiceTestItemID,
			},
			wantErr: ErrCorrelationIDInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{}
			service := &Service{items: store}

			_, err := service.RestoreItem(context.Background(), test.input)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("RestoreItem() error = %v, want %v", err, test.wantErr)
			}

			if store.restoreCalls != 0 {
				t.Fatal("item store was called for invalid restore input")
			}
		})
	}
}

func TestServiceRestoreItemMapsStoreErrorsSafely(t *testing.T) {
	t.Parallel()

	const internalMarker = "synthetic restore database failure"

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

			store := &itemServiceTestStore{restoreErr: test.storeErr}
			service := &Service{items: store}

			_, err := service.RestoreItem(
				context.Background(),
				validItemServiceRestoreInput(),
			)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("RestoreItem() error = %v, want %v", err, test.wantErr)
			}

			if strings.Contains(err.Error(), internalMarker) {
				t.Fatal("RestoreItem() exposed an internal store failure")
			}
		})
	}
}

func TestServiceRestoreItemRejectsMalformedStoredItems(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 23, 3, 0, 0, 0, time.UTC)
	deletedAt := updatedAt

	validResult := Item{
		ID:        itemServiceTestItemID,
		VaultID:   itemServiceTestVaultID,
		Type:      ItemTypeSecureNote,
		Payload:   json.RawMessage(`{"value":"synthetic"}`),
		Version:   2,
		CreatedAt: updatedAt.Add(-time.Hour),
		UpdatedAt: updatedAt,
	}

	tests := []struct {
		name string
		item Item
	}{
		{
			name: "wrong item ID",
			item: Item{
				ID:        itemServiceTestSecondItemID,
				VaultID:   validResult.VaultID,
				Type:      validResult.Type,
				Payload:   validResult.Payload,
				Version:   validResult.Version,
				CreatedAt: validResult.CreatedAt,
				UpdatedAt: validResult.UpdatedAt,
			},
		},
		{
			name: "wrong vault ID",
			item: Item{
				ID:        validResult.ID,
				VaultID:   "00000000-0000-0000-0000-000000001999",
				Type:      validResult.Type,
				Payload:   validResult.Payload,
				Version:   validResult.Version,
				CreatedAt: validResult.CreatedAt,
				UpdatedAt: validResult.UpdatedAt,
			},
		},
		{
			name: "item remained deleted",
			item: Item{
				ID:        validResult.ID,
				VaultID:   validResult.VaultID,
				Type:      validResult.Type,
				Payload:   validResult.Payload,
				Version:   validResult.Version,
				CreatedAt: validResult.CreatedAt,
				UpdatedAt: validResult.UpdatedAt,
				DeletedAt: &deletedAt,
			},
		},
		{
			name: "version was not incremented",
			item: Item{
				ID:        validResult.ID,
				VaultID:   validResult.VaultID,
				Type:      validResult.Type,
				Payload:   validResult.Payload,
				Version:   1,
				CreatedAt: validResult.CreatedAt,
				UpdatedAt: validResult.UpdatedAt,
			},
		},
		{
			name: "invalid payload",
			item: Item{
				ID:        validResult.ID,
				VaultID:   validResult.VaultID,
				Type:      validResult.Type,
				Payload:   json.RawMessage(`["not-an-object"]`),
				Version:   validResult.Version,
				CreatedAt: validResult.CreatedAt,
				UpdatedAt: validResult.UpdatedAt,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{restoreResult: test.item}
			service := &Service{items: store}

			_, err := service.RestoreItem(
				context.Background(),
				validItemServiceRestoreInput(),
			)

			if !errors.Is(err, ErrItemUnavailable) {
				t.Fatalf(
					"RestoreItem() error = %v, want %v",
					err,
					ErrItemUnavailable,
				)
			}
		})
	}
}

func TestServiceRestoreItemRejectsUnavailableDependency(t *testing.T) {
	t.Parallel()

	service := &Service{}

	_, err := service.RestoreItem(
		context.Background(),
		validItemServiceRestoreInput(),
	)

	if !errors.Is(err, ErrItemUnavailable) {
		t.Fatalf("RestoreItem() error = %v, want %v", err, ErrItemUnavailable)
	}
}

func TestServiceRestoreItemPreservesCanceledContext(t *testing.T) {
	t.Parallel()

	store := &itemServiceTestStore{}
	service := &Service{items: store}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.RestoreItem(ctx, validItemServiceRestoreInput())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RestoreItem() error = %v, want %v", err, context.Canceled)
	}

	if store.restoreCalls != 0 {
		t.Fatal("item store was called after context cancellation")
	}
}

func validItemServiceRestoreInput() RestoreItemInput {
	return RestoreItemInput{
		OwnerID:       itemServiceTestOwnerID,
		VaultID:       itemServiceTestVaultID,
		ItemID:        itemServiceTestItemID,
		CorrelationID: itemServiceRestoreRequest,
	}
}
