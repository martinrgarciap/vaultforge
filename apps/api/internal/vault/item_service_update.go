package vault

import (
	"context"
	"fmt"
)

type UpdateItemInput struct {
	OwnerID           string
	VaultID           string
	ItemID            string
	Type              ItemType
	EncryptedEnvelope *EncryptedItemEnvelope
	ExpectedVersion   int
	CorrelationID     string
}

func (service *Service) UpdateItem(
	ctx context.Context,
	input UpdateItemInput,
) (Item, error) {
	if err := ctx.Err(); err != nil {
		return Item{}, err
	}

	if !service.itemsAvailable() {
		return Item{}, fmt.Errorf("update vault item: %w", ErrItemUnavailable)
	}

	if !validIdentifier(input.OwnerID) {
		return Item{}, ErrOwnerInvalid
	}

	if !validIdentifier(input.VaultID) {
		return Item{}, ErrVaultNotFound
	}

	if !validIdentifier(input.ItemID) {
		return Item{}, ErrItemNotFound
	}

	if !input.Type.Valid() {
		return Item{}, ErrItemTypeInvalid
	}

	if !validIdentifier(input.CorrelationID) {
		return Item{}, ErrCorrelationIDInvalid
	}

	envelope, err := newItemEnvelopeFromWriteInput(input.EncryptedEnvelope)
	if err != nil {
		return Item{}, err
	}

	if err := ValidateExpectedItemVersion(input.ExpectedVersion); err != nil {
		return Item{}, err
	}

	if err := service.requireVaultCryptoInitialized(
		ctx,
		input.OwnerID,
		input.VaultID,
	); err != nil {
		return Item{}, err
	}

	updatedItem, err := service.items.UpdateItem(
		ctx,
		UpdateItemStoreInput{
			OwnerID:         input.OwnerID,
			VaultID:         input.VaultID,
			ItemID:          input.ItemID,
			Type:            input.Type,
			Envelope:        envelope,
			ExpectedVersion: input.ExpectedVersion,
			CorrelationID:   input.CorrelationID,
		},
	)
	if err != nil {
		return Item{}, mapItemOperationError("update vault item", err)
	}

	if !validStoredItem(updatedItem, input.VaultID, ItemListStateActive) ||
		updatedItem.ID != input.ItemID ||
		updatedItem.Type != input.Type ||
		updatedItem.Version != input.ExpectedVersion+1 ||
		!itemEnvelopesEqual(updatedItem.Envelope(), envelope) {
		return Item{}, fmt.Errorf("update vault item: %w", ErrItemUnavailable)
	}

	return updatedItem, nil
}
