package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type CreateItemInput struct {
	OwnerID       string
	VaultID       string
	Type          ItemType
	Payload       json.RawMessage
	CorrelationID string
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

	envelope, err := NewSyntheticItemEnvelope(input.Payload)
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
			CorrelationID: input.CorrelationID,
		},
	)
	if err != nil {
		return Item{}, mapItemOperationError("create vault item", err)
	}

	if !validStoredItem(createdItem, input.VaultID, ItemListStateActive) ||
		createdItem.Type != input.Type ||
		createdItem.Version != 1 ||
		!bytes.Equal(createdItem.Payload, envelope.Payload) {
		return Item{}, fmt.Errorf("create vault item: %w", ErrItemUnavailable)
	}

	return createdItem, nil
}

func (service *Service) itemsAvailable() bool {
	return service != nil && service.items != nil
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

	normalizedPayload, err := NormalizeSyntheticItemPayload(storedItem.Payload)
	if err != nil || !bytes.Equal(normalizedPayload, storedItem.Payload) {
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

func mapItemOperationError(operation string, err error) error {
	switch {
	case errors.Is(err, ErrVaultNotFound):
		return ErrVaultNotFound

	case errors.Is(err, ErrItemNotFound):
		return ErrItemNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf("%s: %w", operation, ErrItemUnavailable)
	}
}
