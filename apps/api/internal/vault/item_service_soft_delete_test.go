package vault

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const itemServiceSoftDeleteRequest = "item-service-soft-delete-request"

func TestServiceSoftDeleteItemDeletesOwnedItem(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 23, 0, 0, 0, 0, time.UTC)
	deletedAt := updatedAt

	store := &itemServiceTestStore{
		softDeleteResult: Item{
			ID:        itemServiceTestItemID,
			VaultID:   itemServiceTestVaultID,
			Type:      ItemTypeSecureNote,
			Payload:   json.RawMessage(`{"value":"synthetic"}`),
			Version:   1,
			CreatedAt: updatedAt.Add(-time.Hour),
			UpdatedAt: updatedAt,
			DeletedAt: &deletedAt,
		},
	}

	service := &Service{items: store}

	deletedItem, err := service.SoftDeleteItem(
		context.Background(),
		validItemServiceSoftDeleteInput(),
	)
	if err != nil {
		t.Fatalf("SoftDeleteItem() error = %v", err)
	}

	if store.softDeleteCalls != 1 {
		t.Fatalf(
			"SoftDeleteItem() store calls = %d, want 1",
			store.softDeleteCalls,
		)
	}

	if store.lastSoftDeleteInput.OwnerID != itemServiceTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			store.lastSoftDeleteInput.OwnerID,
			itemServiceTestOwnerID,
		)
	}

	if store.lastSoftDeleteInput.VaultID != itemServiceTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			store.lastSoftDeleteInput.VaultID,
			itemServiceTestVaultID,
		)
	}

	if store.lastSoftDeleteInput.ItemID != itemServiceTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			store.lastSoftDeleteInput.ItemID,
			itemServiceTestItemID,
		)
	}

	if store.lastSoftDeleteInput.ExpectedVersion != 1 {
		t.Fatalf(
			"expected version = %d, want 1",
			store.lastSoftDeleteInput.ExpectedVersion,
		)
	}

	if store.lastSoftDeleteInput.CorrelationID != itemServiceSoftDeleteRequest {
		t.Fatalf(
			"correlation ID = %q, want %q",
			store.lastSoftDeleteInput.CorrelationID,
			itemServiceSoftDeleteRequest,
		)
	}

	if deletedItem.ID != itemServiceTestItemID {
		t.Fatalf(
			"deleted item ID = %q, want %q",
			deletedItem.ID,
			itemServiceTestItemID,
		)
	}

	if !deletedItem.Deleted() {
		t.Fatal("soft-deleted item was returned as active")
	}

	if deletedItem.Version != 1 {
		t.Fatalf("deleted item version = %d, want 1", deletedItem.Version)
	}
}

func TestServiceSoftDeleteItemRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   SoftDeleteItemInput
		wantErr error
	}{
		{
			name: "invalid owner",
			input: SoftDeleteItemInput{
				VaultID:         itemServiceTestVaultID,
				ItemID:          itemServiceTestItemID,
				ExpectedVersion: 1,
				CorrelationID:   itemServiceSoftDeleteRequest,
			},
			wantErr: ErrOwnerInvalid,
		},
		{
			name: "invalid vault",
			input: SoftDeleteItemInput{
				OwnerID:         itemServiceTestOwnerID,
				VaultID:         " ",
				ItemID:          itemServiceTestItemID,
				ExpectedVersion: 1,
				CorrelationID:   itemServiceSoftDeleteRequest,
			},
			wantErr: ErrVaultNotFound,
		},
		{
			name: "invalid item",
			input: SoftDeleteItemInput{
				OwnerID:         itemServiceTestOwnerID,
				VaultID:         itemServiceTestVaultID,
				ItemID:          "",
				ExpectedVersion: 1,
				CorrelationID:   itemServiceSoftDeleteRequest,
			},
			wantErr: ErrItemNotFound,
		},
		{
			name: "invalid correlation ID",
			input: SoftDeleteItemInput{
				OwnerID:         itemServiceTestOwnerID,
				VaultID:         itemServiceTestVaultID,
				ItemID:          itemServiceTestItemID,
				ExpectedVersion: 1,
			},
			wantErr: ErrCorrelationIDInvalid,
		},
		{
			name: "invalid expected version",
			input: SoftDeleteItemInput{
				OwnerID:         itemServiceTestOwnerID,
				VaultID:         itemServiceTestVaultID,
				ItemID:          itemServiceTestItemID,
				ExpectedVersion: 0,
				CorrelationID:   itemServiceSoftDeleteRequest,
			},
			wantErr: ErrItemVersionInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{}
			service := &Service{items: store}

			_, err := service.SoftDeleteItem(context.Background(), test.input)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"SoftDeleteItem() error = %v, want %v",
					err,
					test.wantErr,
				)
			}

			if store.softDeleteCalls != 0 {
				t.Fatal("item store was called for invalid soft-delete input")
			}
		})
	}
}

func TestServiceSoftDeleteItemMapsStoreErrorsSafely(t *testing.T) {
	t.Parallel()

	const internalMarker = "synthetic soft-delete database failure"

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
		{
			name:     "version conflict",
			storeErr: ErrItemConflict,
			wantErr:  ErrItemConflict,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{softDeleteErr: test.storeErr}
			service := &Service{items: store}

			_, err := service.SoftDeleteItem(
				context.Background(),
				validItemServiceSoftDeleteInput(),
			)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"SoftDeleteItem() error = %v, want %v",
					err,
					test.wantErr,
				)
			}

			if strings.Contains(err.Error(), internalMarker) {
				t.Fatal("SoftDeleteItem() exposed an internal store failure")
			}
		})
	}
}

func TestServiceSoftDeleteItemRejectsMalformedStoredItems(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 23, 1, 0, 0, 0, time.UTC)
	deletedAt := updatedAt

	validResult := Item{
		ID:        itemServiceTestItemID,
		VaultID:   itemServiceTestVaultID,
		Type:      ItemTypeSecureNote,
		Payload:   json.RawMessage(`{"value":"synthetic"}`),
		Version:   1,
		CreatedAt: updatedAt.Add(-time.Hour),
		UpdatedAt: updatedAt,
		DeletedAt: &deletedAt,
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
				DeletedAt: validResult.DeletedAt,
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
				DeletedAt: validResult.DeletedAt,
			},
		},
		{
			name: "active item returned",
			item: Item{
				ID:        validResult.ID,
				VaultID:   validResult.VaultID,
				Type:      validResult.Type,
				Payload:   validResult.Payload,
				Version:   validResult.Version,
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
				DeletedAt: validResult.DeletedAt,
			},
		},
		{
			name: "invalid version",
			item: Item{
				ID:        validResult.ID,
				VaultID:   validResult.VaultID,
				Type:      validResult.Type,
				Payload:   validResult.Payload,
				Version:   0,
				CreatedAt: validResult.CreatedAt,
				UpdatedAt: validResult.UpdatedAt,
				DeletedAt: validResult.DeletedAt,
			},
		},
		{
			name: "wrong returned version",
			item: Item{
				ID:        validResult.ID,
				VaultID:   validResult.VaultID,
				Type:      validResult.Type,
				Payload:   validResult.Payload,
				Version:   2,
				CreatedAt: validResult.CreatedAt,
				UpdatedAt: validResult.UpdatedAt,
				DeletedAt: validResult.DeletedAt,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{softDeleteResult: test.item}
			service := &Service{items: store}

			_, err := service.SoftDeleteItem(
				context.Background(),
				validItemServiceSoftDeleteInput(),
			)

			if !errors.Is(err, ErrItemUnavailable) {
				t.Fatalf(
					"SoftDeleteItem() error = %v, want %v",
					err,
					ErrItemUnavailable,
				)
			}
		})
	}
}

func TestServiceSoftDeleteItemRejectsUnavailableDependency(t *testing.T) {
	t.Parallel()

	service := &Service{}

	_, err := service.SoftDeleteItem(
		context.Background(),
		validItemServiceSoftDeleteInput(),
	)

	if !errors.Is(err, ErrItemUnavailable) {
		t.Fatalf(
			"SoftDeleteItem() error = %v, want %v",
			err,
			ErrItemUnavailable,
		)
	}
}

func TestServiceSoftDeleteItemPreservesCanceledContext(t *testing.T) {
	t.Parallel()

	store := &itemServiceTestStore{}
	service := &Service{items: store}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.SoftDeleteItem(
		ctx,
		validItemServiceSoftDeleteInput(),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"SoftDeleteItem() error = %v, want %v",
			err,
			context.Canceled,
		)
	}

	if store.softDeleteCalls != 0 {
		t.Fatal("item store was called after context cancellation")
	}
}

func validItemServiceSoftDeleteInput() SoftDeleteItemInput {
	return SoftDeleteItemInput{
		OwnerID:         itemServiceTestOwnerID,
		VaultID:         itemServiceTestVaultID,
		ItemID:          itemServiceTestItemID,
		ExpectedVersion: 1,
		CorrelationID:   itemServiceSoftDeleteRequest,
	}
}
