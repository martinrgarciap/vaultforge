package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type encryptedItemServiceRegressionStore struct {
	createCalls int
	updateCalls int

	lastCreateInput CreateItemStoreInput
	lastUpdateInput UpdateItemStoreInput
}

var _ ItemStore = (*encryptedItemServiceRegressionStore)(nil)

func (store *encryptedItemServiceRegressionStore) CreateItem(
	_ context.Context,
	input CreateItemStoreInput,
) (Item, error) {
	store.createCalls++
	store.lastCreateInput = input

	now := time.Date(2026, time.July, 8, 12, 0, 0, 0, time.UTC)

	return Item{
		ID:        itemServiceTestItemID,
		VaultID:   input.VaultID,
		Type:      input.Type,
		Payload:   json.RawMessage(bytes.Clone(input.Envelope.Payload)),
		Nonce:     bytes.Clone(input.Envelope.Nonce),
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (store *encryptedItemServiceRegressionStore) ListItems(
	context.Context,
	ListItemsStoreInput,
) (ItemPage, error) {
	return ItemPage{}, nil
}

func (store *encryptedItemServiceRegressionStore) GetItem(
	context.Context,
	GetItemStoreInput,
) (Item, error) {
	return Item{}, ErrItemNotFound
}

func (store *encryptedItemServiceRegressionStore) UpdateItem(
	_ context.Context,
	input UpdateItemStoreInput,
) (Item, error) {
	store.updateCalls++
	store.lastUpdateInput = input

	now := time.Date(2026, time.July, 8, 12, 5, 0, 0, time.UTC)

	return Item{
		ID:        input.ItemID,
		VaultID:   input.VaultID,
		Type:      input.Type,
		Payload:   json.RawMessage(bytes.Clone(input.Envelope.Payload)),
		Nonce:     bytes.Clone(input.Envelope.Nonce),
		Version:   input.ExpectedVersion + 1,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
	}, nil
}

func (store *encryptedItemServiceRegressionStore) SoftDeleteItem(
	context.Context,
	SoftDeleteItemStoreInput,
) (Item, error) {
	return Item{}, ErrItemNotFound
}

func (store *encryptedItemServiceRegressionStore) RestoreItem(
	context.Context,
	RestoreItemStoreInput,
) (Item, error) {
	return Item{}, ErrItemNotFound
}

func (store *encryptedItemServiceRegressionStore) PermanentDeleteItem(
	context.Context,
	PermanentDeleteItemStoreInput,
) error {
	return ErrItemNotFound
}

func TestServiceCreateItemStoresEncryptedEnvelopeWithoutSyntheticPayload(t *testing.T) {
	t.Parallel()

	blob := validEncryptedRegressionBlob(t)
	encryptedEnvelope, err := NewEncryptedItemEnvelope(blob)
	if err != nil {
		t.Fatalf("NewEncryptedItemEnvelope() error = %v", err)
	}

	store := &encryptedItemServiceRegressionStore{}
	service := &Service{items: store}

	createdItem, err := service.CreateItem(
		context.Background(),
		CreateItemInput{
			OwnerID:           itemServiceTestOwnerID,
			VaultID:           itemServiceTestVaultID,
			Type:              ItemTypeSecureNote,
			EncryptedEnvelope: &encryptedEnvelope,
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

	assertEncryptedRegressionEnvelope(
		t,
		store.lastCreateInput.Envelope,
		blob,
	)

	if IsSyntheticItemNonce(createdItem.Nonce) {
		t.Fatal("created encrypted item used synthetic nonce")
	}
}

func TestServiceUpdateItemStoresEncryptedEnvelopeWithoutSyntheticPayload(t *testing.T) {
	t.Parallel()

	blob := validEncryptedRegressionBlob(t)
	encryptedEnvelope, err := NewEncryptedItemEnvelope(blob)
	if err != nil {
		t.Fatalf("NewEncryptedItemEnvelope() error = %v", err)
	}

	store := &encryptedItemServiceRegressionStore{}
	service := &Service{items: store}

	updatedItem, err := service.UpdateItem(
		context.Background(),
		UpdateItemInput{
			OwnerID:           itemServiceTestOwnerID,
			VaultID:           itemServiceTestVaultID,
			ItemID:            itemServiceTestItemID,
			Type:              ItemTypeLogin,
			EncryptedEnvelope: &encryptedEnvelope,
			ExpectedVersion:   1,
			CorrelationID:     itemServiceTestRequest,
		},
	)
	if err != nil {
		t.Fatalf("UpdateItem() error = %v", err)
	}

	if store.updateCalls != 1 {
		t.Fatalf("UpdateItem() store calls = %d, want 1", store.updateCalls)
	}

	assertEncryptedRegressionEnvelope(
		t,
		store.lastUpdateInput.Envelope,
		blob,
	)

	if IsSyntheticItemNonce(updatedItem.Nonce) {
		t.Fatal("updated encrypted item used synthetic nonce")
	}
}

func TestServiceWriteRejectsMixedPlaintextAndEncryptedPayloads(t *testing.T) {
	t.Parallel()

	blob := validEncryptedRegressionBlob(t)
	encryptedEnvelope, err := NewEncryptedItemEnvelope(blob)
	if err != nil {
		t.Fatalf("NewEncryptedItemEnvelope() error = %v", err)
	}

	store := &encryptedItemServiceRegressionStore{}
	service := &Service{items: store}

	_, err = service.CreateItem(
		context.Background(),
		CreateItemInput{
			OwnerID:           itemServiceTestOwnerID,
			VaultID:           itemServiceTestVaultID,
			Type:              ItemTypeSecureNote,
			Payload:           json.RawMessage(`{"secret":"do-not-mix"}`),
			EncryptedEnvelope: &encryptedEnvelope,
			IdempotencyKey:    itemServiceTestIdempotencyKey,
			CorrelationID:     itemServiceTestRequest,
		},
	)
	if !errors.Is(err, ErrItemPayloadInvalid) {
		t.Fatalf("CreateItem() error = %v, want %v", err, ErrItemPayloadInvalid)
	}

	if store.createCalls != 0 {
		t.Fatal("mixed plaintext and encrypted create reached the store")
	}

	_, err = service.UpdateItem(
		context.Background(),
		UpdateItemInput{
			OwnerID:           itemServiceTestOwnerID,
			VaultID:           itemServiceTestVaultID,
			ItemID:            itemServiceTestItemID,
			Type:              ItemTypeSecureNote,
			Payload:           json.RawMessage(`{"secret":"do-not-mix"}`),
			EncryptedEnvelope: &encryptedEnvelope,
			ExpectedVersion:   1,
			CorrelationID:     itemServiceTestRequest,
		},
	)
	if !errors.Is(err, ErrItemPayloadInvalid) {
		t.Fatalf("UpdateItem() error = %v, want %v", err, ErrItemPayloadInvalid)
	}

	if store.updateCalls != 0 {
		t.Fatal("mixed plaintext and encrypted update reached the store")
	}
}

func validEncryptedRegressionBlob(t *testing.T) []byte {
	t.Helper()

	blob := make(
		[]byte,
		ItemEncryptedPayloadNonceBytes+ItemEncryptedPayloadTagBytes+8,
	)

	for index := range blob {
		blob[index] = byte(index + 1)
	}

	return blob
}

func assertEncryptedRegressionEnvelope(
	t *testing.T,
	envelope ItemEnvelope,
	wantBlob []byte,
) {
	t.Helper()

	if IsSyntheticItemNonce(envelope.Nonce) {
		t.Fatal("encrypted envelope used synthetic nonce")
	}

	gotEnvelope := EncryptedItemEnvelope(envelope)

	gotBlob, err := gotEnvelope.Blob()
	if err != nil {
		t.Fatalf("Blob() error = %v", err)
	}

	if !bytes.Equal(gotBlob, wantBlob) {
		t.Fatal("encrypted envelope bytes did not match input blob")
	}
}
