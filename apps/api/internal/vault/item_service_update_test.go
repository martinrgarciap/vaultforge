package vault

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const itemServiceUpdateRequest = "item-service-update-request"

func TestServiceUpdateItemNormalizesAndUpdatesItem(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 22, 22, 0, 0, 0, time.UTC)

	store := &itemServiceTestStore{
		updateResult: Item{
			ID:        itemServiceTestItemID,
			VaultID:   itemServiceTestVaultID,
			Type:      ItemTypeAPIKey,
			Payload:   json.RawMessage(`{"label":"Updated","token":"synthetic-token"}`),
			Version:   2,
			CreatedAt: updatedAt.Add(-time.Hour),
			UpdatedAt: updatedAt,
		},
	}

	service := &Service{items: store}

	updatedItem, err := service.UpdateItem(
		context.Background(),
		UpdateItemInput{
			OwnerID: itemServiceTestOwnerID,
			VaultID: itemServiceTestVaultID,
			ItemID:  itemServiceTestItemID,
			Type:    ItemTypeAPIKey,
			Payload: json.RawMessage(`{
				"token": "synthetic-token",
				"label": "Updated"
			}`),
			CorrelationID: itemServiceUpdateRequest,
		},
	)
	if err != nil {
		t.Fatalf("UpdateItem() error = %v", err)
	}

	if store.updateCalls != 1 {
		t.Fatalf("UpdateItem() store calls = %d, want 1", store.updateCalls)
	}

	if store.lastUpdateInput.OwnerID != itemServiceTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			store.lastUpdateInput.OwnerID,
			itemServiceTestOwnerID,
		)
	}

	if store.lastUpdateInput.VaultID != itemServiceTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			store.lastUpdateInput.VaultID,
			itemServiceTestVaultID,
		)
	}

	if store.lastUpdateInput.ItemID != itemServiceTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			store.lastUpdateInput.ItemID,
			itemServiceTestItemID,
		)
	}

	if store.lastUpdateInput.Type != ItemTypeAPIKey {
		t.Fatalf(
			"item type = %q, want %q",
			store.lastUpdateInput.Type,
			ItemTypeAPIKey,
		)
	}

	if store.lastUpdateInput.CorrelationID != itemServiceUpdateRequest {
		t.Fatalf(
			"correlation ID = %q, want %q",
			store.lastUpdateInput.CorrelationID,
			itemServiceUpdateRequest,
		)
	}

	const wantPayload = `{"label":"Updated","token":"synthetic-token"}`

	if string(store.lastUpdateInput.Envelope.Payload) != wantPayload {
		t.Fatalf(
			"store payload = %s, want %s",
			store.lastUpdateInput.Envelope.Payload,
			wantPayload,
		)
	}

	if !IsSyntheticItemNonce(store.lastUpdateInput.Envelope.Nonce) {
		t.Fatal("store input did not contain the synthetic nonce")
	}

	if updatedItem.ID != itemServiceTestItemID {
		t.Fatalf(
			"updated item ID = %q, want %q",
			updatedItem.ID,
			itemServiceTestItemID,
		)
	}

	if updatedItem.Version != 2 {
		t.Fatalf("updated version = %d, want 2", updatedItem.Version)
	}

	if string(updatedItem.Payload) != wantPayload {
		t.Fatalf("updated payload = %s, want %s", updatedItem.Payload, wantPayload)
	}
}

func TestServiceUpdateItemRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   UpdateItemInput
		wantErr error
	}{
		{
			name: "invalid owner",
			input: UpdateItemInput{
				VaultID:       itemServiceTestVaultID,
				ItemID:        itemServiceTestItemID,
				Type:          ItemTypeSecureNote,
				Payload:       json.RawMessage(`{}`),
				CorrelationID: itemServiceUpdateRequest,
			},
			wantErr: ErrOwnerInvalid,
		},
		{
			name: "invalid vault",
			input: UpdateItemInput{
				OwnerID:       itemServiceTestOwnerID,
				VaultID:       " ",
				ItemID:        itemServiceTestItemID,
				Type:          ItemTypeSecureNote,
				Payload:       json.RawMessage(`{}`),
				CorrelationID: itemServiceUpdateRequest,
			},
			wantErr: ErrVaultNotFound,
		},
		{
			name: "invalid item",
			input: UpdateItemInput{
				OwnerID:       itemServiceTestOwnerID,
				VaultID:       itemServiceTestVaultID,
				ItemID:        "",
				Type:          ItemTypeSecureNote,
				Payload:       json.RawMessage(`{}`),
				CorrelationID: itemServiceUpdateRequest,
			},
			wantErr: ErrItemNotFound,
		},
		{
			name: "invalid type",
			input: UpdateItemInput{
				OwnerID:       itemServiceTestOwnerID,
				VaultID:       itemServiceTestVaultID,
				ItemID:        itemServiceTestItemID,
				Type:          "unsupported",
				Payload:       json.RawMessage(`{}`),
				CorrelationID: itemServiceUpdateRequest,
			},
			wantErr: ErrItemTypeInvalid,
		},
		{
			name: "invalid correlation ID",
			input: UpdateItemInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: itemServiceTestVaultID,
				ItemID:  itemServiceTestItemID,
				Type:    ItemTypeSecureNote,
				Payload: json.RawMessage(`{}`),
			},
			wantErr: ErrCorrelationIDInvalid,
		},
		{
			name: "empty payload",
			input: UpdateItemInput{
				OwnerID:       itemServiceTestOwnerID,
				VaultID:       itemServiceTestVaultID,
				ItemID:        itemServiceTestItemID,
				Type:          ItemTypeSecureNote,
				Payload:       nil,
				CorrelationID: itemServiceUpdateRequest,
			},
			wantErr: ErrItemPayloadEmpty,
		},
		{
			name: "payload is not object",
			input: UpdateItemInput{
				OwnerID:       itemServiceTestOwnerID,
				VaultID:       itemServiceTestVaultID,
				ItemID:        itemServiceTestItemID,
				Type:          ItemTypeSecureNote,
				Payload:       json.RawMessage(`["value"]`),
				CorrelationID: itemServiceUpdateRequest,
			},
			wantErr: ErrItemPayloadNotObject,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{}
			service := &Service{items: store}

			_, err := service.UpdateItem(context.Background(), test.input)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("UpdateItem() error = %v, want %v", err, test.wantErr)
			}

			if store.updateCalls != 0 {
				t.Fatal("item store was called for invalid update input")
			}
		})
	}
}

func TestServiceUpdateItemMapsStoreErrorsSafely(t *testing.T) {
	t.Parallel()

	const internalMarker = "synthetic update database failure"

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

			store := &itemServiceTestStore{updateErr: test.storeErr}
			service := &Service{items: store}

			_, err := service.UpdateItem(
				context.Background(),
				validItemServiceUpdateInput(),
			)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("UpdateItem() error = %v, want %v", err, test.wantErr)
			}

			if strings.Contains(err.Error(), internalMarker) {
				t.Fatal("UpdateItem() exposed an internal store failure")
			}
		})
	}
}

func TestServiceUpdateItemRejectsMalformedStoredItems(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 22, 23, 0, 0, 0, time.UTC)
	deletedAt := updatedAt

	validResult := Item{
		ID:        itemServiceTestItemID,
		VaultID:   itemServiceTestVaultID,
		Type:      ItemTypeAPIKey,
		Payload:   json.RawMessage(`{"label":"Updated","token":"synthetic-token"}`),
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
			name: "wrong item type",
			item: Item{
				ID:        validResult.ID,
				VaultID:   validResult.VaultID,
				Type:      ItemTypeSecureNote,
				Payload:   validResult.Payload,
				Version:   validResult.Version,
				CreatedAt: validResult.CreatedAt,
				UpdatedAt: validResult.UpdatedAt,
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
			name: "payload does not match request",
			item: Item{
				ID:        validResult.ID,
				VaultID:   validResult.VaultID,
				Type:      validResult.Type,
				Payload:   json.RawMessage(`{"value":"different"}`),
				Version:   validResult.Version,
				CreatedAt: validResult.CreatedAt,
				UpdatedAt: validResult.UpdatedAt,
			},
		},
		{
			name: "deleted item returned",
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{updateResult: test.item}
			service := &Service{items: store}

			_, err := service.UpdateItem(
				context.Background(),
				validItemServiceUpdateInput(),
			)

			if !errors.Is(err, ErrItemUnavailable) {
				t.Fatalf("UpdateItem() error = %v, want %v", err, ErrItemUnavailable)
			}
		})
	}
}

func TestServiceUpdateItemRejectsUnavailableDependency(t *testing.T) {
	t.Parallel()

	service := &Service{}

	_, err := service.UpdateItem(
		context.Background(),
		validItemServiceUpdateInput(),
	)

	if !errors.Is(err, ErrItemUnavailable) {
		t.Fatalf("UpdateItem() error = %v, want %v", err, ErrItemUnavailable)
	}
}

func TestServiceUpdateItemPreservesCanceledContext(t *testing.T) {
	t.Parallel()

	store := &itemServiceTestStore{}
	service := &Service{items: store}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.UpdateItem(ctx, validItemServiceUpdateInput())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateItem() error = %v, want %v", err, context.Canceled)
	}

	if store.updateCalls != 0 {
		t.Fatal("item store was called after context cancellation")
	}
}

func validItemServiceUpdateInput() UpdateItemInput {
	return UpdateItemInput{
		OwnerID:       itemServiceTestOwnerID,
		VaultID:       itemServiceTestVaultID,
		ItemID:        itemServiceTestItemID,
		Type:          ItemTypeAPIKey,
		Payload:       json.RawMessage(`{"label":"Updated","token":"synthetic-token"}`),
		CorrelationID: itemServiceUpdateRequest,
	}
}
