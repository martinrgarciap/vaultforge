package vault

import (
	"context"
	"fmt"
)

type RestoreItemInput struct {
	OwnerID       string
	VaultID       string
	ItemID        string
	CorrelationID string
}

func (service *Service) RestoreItem(
	ctx context.Context,
	input RestoreItemInput,
) (Item, error) {
	if err := ctx.Err(); err != nil {
		return Item{}, err
	}

	if !service.itemsAvailable() {
		return Item{}, fmt.Errorf("restore vault item: %w", ErrItemUnavailable)
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

	if !validIdentifier(input.CorrelationID) {
		return Item{}, ErrCorrelationIDInvalid
	}

	restoredItem, err := service.items.RestoreItem(
		ctx,
		RestoreItemStoreInput(input),
	)
	if err != nil {
		return Item{}, mapItemOperationError("restore vault item", err)
	}

	if !validStoredItem(restoredItem, input.VaultID, ItemListStateActive) ||
		restoredItem.ID != input.ItemID ||
		restoredItem.Version < 2 {
		return Item{}, fmt.Errorf("restore vault item: %w", ErrItemUnavailable)
	}

	return restoredItem, nil
}
