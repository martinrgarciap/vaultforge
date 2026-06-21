package vault

import (
	"context"
	"fmt"
)

type GetItemInput struct {
	OwnerID string
	VaultID string
	ItemID  string
	State   ItemListState
}

func (service *Service) GetItem(
	ctx context.Context,
	input GetItemInput,
) (Item, error) {
	if err := ctx.Err(); err != nil {
		return Item{}, err
	}

	if !service.itemsAvailable() {
		return Item{}, fmt.Errorf("get vault item: %w", ErrItemUnavailable)
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

	state := input.State
	if state == "" {
		state = ItemListStateActive
	}

	if !state.Valid() {
		return Item{}, ErrItemListStateInvalid
	}

	storedItem, err := service.items.GetItem(
		ctx,
		GetItemStoreInput{
			OwnerID: input.OwnerID,
			VaultID: input.VaultID,
			ItemID:  input.ItemID,
			State:   state,
		},
	)
	if err != nil {
		return Item{}, mapItemOperationError("get vault item", err)
	}

	if !validStoredItem(storedItem, input.VaultID, state) ||
		storedItem.ID != input.ItemID {
		return Item{}, fmt.Errorf("get vault item: %w", ErrItemUnavailable)
	}

	return storedItem, nil
}
