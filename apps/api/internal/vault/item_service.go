package vault

import (
	"bytes"
	"context"
	"errors"
	"fmt"
)

type CreateItemInput struct {
	OwnerID           string
	VaultID           string
	Type              ItemType
	EncryptedEnvelope *EncryptedItemEnvelope
	IdempotencyKey    string
	CorrelationID     string
}

func (service *Service) CreateItem(
	ctx context.Context,
	input CreateItemInput,
) (Item, error) {
	if err := ctx.Err(); err != nil {
		return Item{}, err
	}

	if !service.itemsAvailable() {
		return Item{}, fmt.Errorf("create vault item: %w", ErrItemUnavailable)
	}

	if !validIdentifier(input.OwnerID) {
		return Item{}, ErrOwnerInvalid
	}

	if !validIdentifier(input.VaultID) {
		return Item{}, ErrVaultNotFound
	}

	if !input.Type.Valid() {
		return Item{}, ErrItemTypeInvalid
	}

	if !validIdentifier(input.CorrelationID) {
		return Item{}, ErrCorrelationIDInvalid
	}

	if err := service.requireVaultCryptoInitialized(
		ctx,
		input.OwnerID,
		input.VaultID,
	); err != nil {
		return Item{}, err
	}

	envelope, err := newItemEnvelopeFromWriteInput(input.EncryptedEnvelope)
	if err != nil {
		return Item{}, err
	}

	idempotency, err := NewItemCreateIdempotency(
		input.IdempotencyKey,
		input.Type,
		envelope,
	)
	if err != nil {
		return Item{}, err
	}

	createdItem, err := service.items.CreateItem(
		ctx,
		CreateItemStoreInput{
			OwnerID:       input.OwnerID,
			VaultID:       input.VaultID,
			Type:          input.Type,
			Envelope:      envelope,
			Idempotency:   idempotency,
			CorrelationID: input.CorrelationID,
		},
	)
	if err != nil {
		return Item{}, mapItemOperationError("create vault item", err)
	}

	if !validStoredItem(createdItem, input.VaultID, ItemListStateActive) ||
		createdItem.Type != input.Type ||
		createdItem.Version != 1 ||
		!itemEnvelopesEqual(createdItem.Envelope(), envelope) {
		return Item{}, fmt.Errorf("create vault item: %w", ErrItemUnavailable)
	}

	return createdItem, nil
}

func (service *Service) itemsAvailable() bool {
	return service != nil && service.items != nil
}

func (service *Service) requireVaultCryptoInitialized(
	ctx context.Context,
	ownerID string,
	vaultID string,
) error {
	if !service.available() {
		return fmt.Errorf("require vault crypto metadata: %w", ErrVaultUnavailable)
	}

	storedVault, err := service.vaults.GetOwned(ctx, ownerID, vaultID)
	if err != nil {
		return mapVaultOperationError("require vault crypto metadata", err)
	}

	if !validStoredVault(storedVault, ownerID) || storedVault.ID != vaultID {
		return fmt.Errorf("require vault crypto metadata: %w", ErrVaultUnavailable)
	}

	if storedVault.CryptoVersion == nil ||
		storedVault.KDFVersion == nil ||
		len(storedVault.Salt) == 0 ||
		len(storedVault.WrappedKey) == 0 {
		return ErrVaultCryptoMetadataInvalid
	}

	return validateVaultCryptoMetadata(
		VaultCryptoMetadata{
			CryptoVersion: *storedVault.CryptoVersion,
			KDFVersion:    *storedVault.KDFVersion,
			Salt:          storedVault.Salt,
			WrappedKey:    storedVault.WrappedKey,
		},
	)
}

func newItemEnvelopeFromWriteInput(
	encryptedEnvelope *EncryptedItemEnvelope,
) (ItemEnvelope, error) {
	if encryptedEnvelope == nil {
		return ItemEnvelope{}, ErrItemEncryptedPayloadEmpty
	}

	return NewItemEnvelopeFromEncrypted(*encryptedEnvelope)
}

func validStoredItem(
	storedItem Item,
	expectedVaultID string,
	expectedState ItemListState,
) bool {
	if !validIdentifier(storedItem.ID) ||
		storedItem.VaultID != expectedVaultID ||
		!storedItem.Type.Valid() ||
		storedItem.Version < 1 ||
		storedItem.CreatedAt.IsZero() ||
		storedItem.UpdatedAt.IsZero() ||
		storedItem.UpdatedAt.Before(storedItem.CreatedAt) {
		return false
	}

	if !validStoredItemEnvelope(storedItem) {
		return false
	}

	if storedItem.DeletedAt != nil && storedItem.DeletedAt.Before(storedItem.CreatedAt) {
		return false
	}

	switch expectedState {
	case ItemListStateActive:
		return storedItem.DeletedAt == nil

	case ItemListStateDeleted:
		return storedItem.DeletedAt != nil

	default:
		return false
	}
}

func validStoredItemEnvelope(storedItem Item) bool {
	if len(storedItem.Nonce) == 0 || IsSyntheticItemNonce(storedItem.Nonce) {
		normalizedPayload, err := NormalizeSyntheticItemPayload(storedItem.Payload)
		return err == nil && bytes.Equal(normalizedPayload, storedItem.Payload)
	}

	_, err := NewItemEnvelopeFromStorage(storedItem.Payload, storedItem.Nonce)
	return err == nil
}

func itemEnvelopesEqual(left ItemEnvelope, right ItemEnvelope) bool {
	return bytes.Equal(left.Payload, right.Payload) &&
		bytes.Equal(left.Nonce, right.Nonce)
}

func mapItemOperationError(operation string, err error) error {
	switch {
	case errors.Is(err, ErrVaultNotFound):
		return ErrVaultNotFound
	case errors.Is(err, ErrItemNotFound):
		return ErrItemNotFound
	case errors.Is(err, ErrItemConflict):
		return ErrItemConflict
	case errors.Is(err, ErrItemIdempotencyConflict):
		return ErrItemIdempotencyConflict
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf("%s: %w", operation, ErrItemUnavailable)
	}
}
