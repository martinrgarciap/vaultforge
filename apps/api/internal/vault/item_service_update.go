package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type UpdateItemInput struct {
	OwnerID       string
	VaultID       string
	ItemID        string
	Type          ItemType
	Payload       json.RawMessage
	CorrelationID string
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

	envelope, err := NewSyntheticItemEnvelope(input.Payload)
	if err != nil {
		return Item{}, err
	}

	updatedItem, err := service.items.UpdateItem(
		ctx,
		UpdateItemStoreInput{
			OwnerID:       input.OwnerID,
			VaultID:       input.VaultID,
			ItemID:        input.ItemID,
			Type:          input.Type,
			Envelope:      envelope,
			CorrelationID: input.CorrelationID,
		},
	)
	if err != nil {
		return Item{}, mapItemOperationError("update vault item", err)
	}

	if !validStoredItem(updatedItem, input.VaultID, ItemListStateActive) ||
		updatedItem.ID != input.ItemID ||
		updatedItem.Type != input.Type ||
		updatedItem.Version < 2 ||
		!bytes.Equal(updatedItem.Payload, envelope.Payload) {
		return Item{}, fmt.Errorf("update vault item: %w", ErrItemUnavailable)
	}

	return updatedItem, nil
}
