package vault

import (
	"context"
	"fmt"
)

type PermanentDeleteItemInput struct {
	OwnerID       string
	VaultID       string
	ItemID        string
	CorrelationID string
}

func (service *Service) PermanentDeleteItem(
	ctx context.Context,
	input PermanentDeleteItemInput,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if !service.itemsAvailable() {
		return fmt.Errorf("permanently delete vault item: %w", ErrItemUnavailable)
	}

	if !validIdentifier(input.OwnerID) {
		return ErrOwnerInvalid
	}

	if !validIdentifier(input.VaultID) {
		return ErrVaultNotFound
	}

	if !validIdentifier(input.ItemID) {
		return ErrItemNotFound
	}

	if !validIdentifier(input.CorrelationID) {
		return ErrCorrelationIDInvalid
	}

	err := service.items.PermanentDeleteItem(
		ctx,
		PermanentDeleteItemStoreInput(input),
	)
	if err != nil {
		return mapItemOperationError("permanently delete vault item", err)
	}

	return nil
}
