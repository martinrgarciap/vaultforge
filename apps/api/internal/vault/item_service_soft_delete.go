package vault

import (
	"context"
	"fmt"
)

type SoftDeleteItemInput struct {
	OwnerID         string
	VaultID         string
	ItemID          string
	ExpectedVersion int
	CorrelationID   string
}

func (service *Service) SoftDeleteItem(
	ctx context.Context,
	input SoftDeleteItemInput,
) (Item, error) {
	if err := ctx.Err(); err != nil {
		return Item{}, err
	}

	if !service.itemsAvailable() {
		return Item{}, fmt.Errorf("soft delete vault item: %w", ErrItemUnavailable)
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

	if err := ValidateExpectedItemVersion(input.ExpectedVersion); err != nil {
		return Item{}, err
	}

	deletedItem, err := service.items.SoftDeleteItem(
		ctx,
		SoftDeleteItemStoreInput(input),
	)
	if err != nil {
		return Item{}, mapItemOperationError("soft delete vault item", err)
	}

	if !validStoredItem(deletedItem, input.VaultID, ItemListStateDeleted) ||
		deletedItem.ID != input.ItemID ||
		deletedItem.Version != input.ExpectedVersion {
		return Item{}, fmt.Errorf("soft delete vault item: %w", ErrItemUnavailable)
	}

	return deletedItem, nil
}
