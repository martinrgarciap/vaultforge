package vault

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	itemServiceTestOwnerID        = "00000000-0000-0000-0000-000000001001"
	itemServiceTestVaultID        = "00000000-0000-0000-0000-000000001002"
	itemServiceTestItemID         = "00000000-0000-0000-0000-000000001003"
	itemServiceTestRequest        = "item-service-create-request"
	itemServiceTestIdempotencyKey = "item-service-create-idempotency-key"
)

func TestServiceCreateItemNormalizesAndCreatesItem(t *testing.T) {
	t.Parallel()

	encryptedEnvelope := validItemServiceEncryptedEnvelopePointer(t)
	createdAt := time.Date(2026, time.June, 22, 14, 0, 0, 0, time.UTC)

	store := &itemServiceTestStore{
		createResult: Item{
			ID:        itemServiceTestItemID,
			VaultID:   itemServiceTestVaultID,
			Type:      ItemTypeAPIKey,
			Payload:   append([]byte(nil), encryptedEnvelope.Payload...),
			Nonce:     append([]byte(nil), encryptedEnvelope.Nonce...),
			Version:   1,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	service := NewService(store)

	createdItem, err := service.CreateItem(
		context.Background(),
		CreateItemInput{
			OwnerID:           itemServiceTestOwnerID,
			VaultID:           itemServiceTestVaultID,
			Type:              ItemTypeAPIKey,
			EncryptedEnvelope: encryptedEnvelope,
			IdempotencyKey:    itemServiceTestIdempotencyKey,
			CorrelationID:     itemServiceTestRequest,
		},
	)
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}

	if store.createCalls != 1 {
		t.Fatalf("CreateItem() store calls = %d, want 1", store.createCalls)
	}

	if store.lastCreateInput.OwnerID != itemServiceTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			store.lastCreateInput.OwnerID,
			itemServiceTestOwnerID,
		)
	}

	if store.lastCreateInput.VaultID != itemServiceTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			store.lastCreateInput.VaultID,
			itemServiceTestVaultID,
		)
	}

	if store.lastCreateInput.Type != ItemTypeAPIKey {
		t.Fatalf("item type = %q, want %q", store.lastCreateInput.Type, ItemTypeAPIKey)
	}

	if store.lastCreateInput.CorrelationID != itemServiceTestRequest {
		t.Fatalf(
			"correlation ID = %q, want %q",
			store.lastCreateInput.CorrelationID,
			itemServiceTestRequest,
		)
	}

	wantEnvelope, err := NewItemEnvelopeFromEncrypted(*encryptedEnvelope)
	if err != nil {
		t.Fatalf("NewItemEnvelopeFromEncrypted() error = %v", err)
	}

	if !itemEnvelopesEqual(store.lastCreateInput.Envelope, wantEnvelope) {
		t.Fatal("store input did not contain the encrypted envelope")
	}

	if IsSyntheticItemNonce(store.lastCreateInput.Envelope.Nonce) {
		t.Fatal("store input used the synthetic nonce")
	}

	wantIdempotency, err := NewItemCreateIdempotency(
		itemServiceTestIdempotencyKey,
		ItemTypeAPIKey,
		store.lastCreateInput.Envelope,
	)
	if err != nil {
		t.Fatalf("create expected idempotency value: %v", err)
	}

	if store.lastCreateInput.Idempotency != wantIdempotency {
		t.Fatal("store input did not contain the expected idempotency hashes")
	}

	if createdItem.ID != itemServiceTestItemID {
		t.Fatalf("item ID = %q, want %q", createdItem.ID, itemServiceTestItemID)
	}

	if !itemEnvelopesEqual(createdItem.Envelope(), wantEnvelope) {
		t.Fatal("created item did not preserve the encrypted envelope")
	}
}

func TestServiceCreateItemRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	validEncryptedEnvelope := validItemServiceEncryptedEnvelopePointer(t)

	tests := []struct {
		name    string
		input   CreateItemInput
		wantErr error
	}{
		{
			name: "invalid owner",
			input: CreateItemInput{
				OwnerID:           "",
				VaultID:           itemServiceTestVaultID,
				Type:              ItemTypeSecureNote,
				EncryptedEnvelope: validEncryptedEnvelope,
				IdempotencyKey:    itemServiceTestIdempotencyKey,
				CorrelationID:     itemServiceTestRequest,
			},
			wantErr: ErrOwnerInvalid,
		},
		{
			name: "invalid vault",
			input: CreateItemInput{
				OwnerID:           itemServiceTestOwnerID,
				VaultID:           " ",
				Type:              ItemTypeSecureNote,
				EncryptedEnvelope: validEncryptedEnvelope,
				IdempotencyKey:    itemServiceTestIdempotencyKey,
				CorrelationID:     itemServiceTestRequest,
			},
			wantErr: ErrVaultNotFound,
		},
		{
			name: "invalid item type",
			input: CreateItemInput{
				OwnerID:           itemServiceTestOwnerID,
				VaultID:           itemServiceTestVaultID,
				Type:              "unsupported",
				EncryptedEnvelope: validEncryptedEnvelope,
				IdempotencyKey:    itemServiceTestIdempotencyKey,
				CorrelationID:     itemServiceTestRequest,
			},
			wantErr: ErrItemTypeInvalid,
		},
		{
			name: "invalid correlation ID",
			input: CreateItemInput{
				OwnerID:           itemServiceTestOwnerID,
				VaultID:           itemServiceTestVaultID,
				Type:              ItemTypeSecureNote,
				EncryptedEnvelope: validEncryptedEnvelope,
				IdempotencyKey:    itemServiceTestIdempotencyKey,
				CorrelationID:     "",
			},
			wantErr: ErrCorrelationIDInvalid,
		},
		{
			name: "missing encrypted payload",
			input: CreateItemInput{
				OwnerID:        itemServiceTestOwnerID,
				VaultID:        itemServiceTestVaultID,
				Type:           ItemTypeSecureNote,
				IdempotencyKey: itemServiceTestIdempotencyKey,
				CorrelationID:  itemServiceTestRequest,
			},
			wantErr: ErrItemEncryptedPayloadEmpty,
		},
		{
			name: "invalid idempotency key",
			input: CreateItemInput{
				OwnerID:           itemServiceTestOwnerID,
				VaultID:           itemServiceTestVaultID,
				Type:              ItemTypeSecureNote,
				EncryptedEnvelope: validEncryptedEnvelope,
				IdempotencyKey:    " ",
				CorrelationID:     itemServiceTestRequest,
			},
			wantErr: ErrItemIdempotencyKeyInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{}
			service := NewService(store)

			_, err := service.CreateItem(context.Background(), test.input)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("CreateItem() error = %v, want %v", err, test.wantErr)
			}

			if store.createCalls != 0 {
				t.Fatal("item store was called for invalid input")
			}
		})
	}
}

func TestServiceCreateItemMapsStoreErrorsSafely(t *testing.T) {
	t.Parallel()

	const internalMarker = "synthetic item database failure"

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
		{
			name:     "idempotency conflict",
			storeErr: ErrItemIdempotencyConflict,
			wantErr:  ErrItemIdempotencyConflict,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{createErr: test.storeErr}
			service := NewService(store)

			_, err := service.CreateItem(
				context.Background(),
				validItemServiceCreateInput(t),
			)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("CreateItem() error = %v, want %v", err, test.wantErr)
			}

			if strings.Contains(err.Error(), internalMarker) {
				t.Fatal("CreateItem() exposed an internal store failure")
			}
		})
	}
}

func TestServiceCreateItemRejectsMalformedStoredItem(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.June, 22, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		item Item
	}{
		{
			name: "missing item ID",
			item: Item{
				VaultID:   itemServiceTestVaultID,
				Type:      ItemTypeSecureNote,
				Payload:   json.RawMessage(`{"value":"synthetic"}`),
				Version:   1,
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
		},
		{
			name: "wrong vault ID",
			item: Item{
				ID:        itemServiceTestItemID,
				VaultID:   "00000000-0000-0000-0000-000000001999",
				Type:      ItemTypeSecureNote,
				Payload:   json.RawMessage(`{"value":"synthetic"}`),
				Version:   1,
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
		},
		{
			name: "wrong version",
			item: Item{
				ID:        itemServiceTestItemID,
				VaultID:   itemServiceTestVaultID,
				Type:      ItemTypeSecureNote,
				Payload:   json.RawMessage(`{"value":"synthetic"}`),
				Version:   2,
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
		},
		{
			name: "deleted item",
			item: Item{
				ID:        itemServiceTestItemID,
				VaultID:   itemServiceTestVaultID,
				Type:      ItemTypeSecureNote,
				Payload:   json.RawMessage(`{"value":"synthetic"}`),
				Version:   1,
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
				DeletedAt: &createdAt,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{createResult: test.item}
			service := NewService(store)

			_, err := service.CreateItem(
				context.Background(),
				validItemServiceCreateInput(t),
			)

			if !errors.Is(err, ErrItemUnavailable) {
				t.Fatalf("CreateItem() error = %v, want %v", err, ErrItemUnavailable)
			}
		})
	}
}

func TestServiceCreateItemRejectsUnavailableDependency(t *testing.T) {
	t.Parallel()

	service := &Service{}

	_, err := service.CreateItem(context.Background(), validItemServiceCreateInput(t))

	if !errors.Is(err, ErrItemUnavailable) {
		t.Fatalf("CreateItem() error = %v, want %v", err, ErrItemUnavailable)
	}
}

func TestServiceCreateItemPreservesCanceledContext(t *testing.T) {
	t.Parallel()

	store := &itemServiceTestStore{}
	service := NewService(store)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.CreateItem(ctx, validItemServiceCreateInput(t))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateItem() error = %v, want %v", err, context.Canceled)
	}

	if store.createCalls != 0 {
		t.Fatal("item store was called after context cancellation")
	}
}

func validItemServiceEncryptedEnvelopePointer(t *testing.T) *EncryptedItemEnvelope {
	t.Helper()

	encryptedEnvelope, err := NewEncryptedItemEnvelope(validEncryptedRegressionBlob(t))
	if err != nil {
		t.Fatalf("NewEncryptedItemEnvelope() error = %v", err)
	}

	return &encryptedEnvelope
}

func validItemServiceCreateInput(t *testing.T) CreateItemInput {
	t.Helper()

	return CreateItemInput{
		OwnerID:           itemServiceTestOwnerID,
		VaultID:           itemServiceTestVaultID,
		Type:              ItemTypeSecureNote,
		EncryptedEnvelope: validItemServiceEncryptedEnvelopePointer(t),
		IdempotencyKey:    itemServiceTestIdempotencyKey,
		CorrelationID:     itemServiceTestRequest,
	}
}
