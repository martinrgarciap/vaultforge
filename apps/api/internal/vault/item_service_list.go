package vault

import (
	"context"
	"fmt"
)

type ListItemsInput struct {
	OwnerID string
	VaultID string
	Options ItemListOptions
}

func (service *Service) ListItems(
	ctx context.Context,
	input ListItemsInput,
) (ItemPage, error) {
	if err := ctx.Err(); err != nil {
		return ItemPage{}, err
	}

	if !service.itemsAvailable() {
		return ItemPage{}, fmt.Errorf("list vault items: %w", ErrItemUnavailable)
	}

	if !validIdentifier(input.OwnerID) {
		return ItemPage{}, ErrOwnerInvalid
	}

	if !validIdentifier(input.VaultID) {
		return ItemPage{}, ErrVaultNotFound
	}

	options, err := NormalizeItemListOptions(input.Options)
	if err != nil {
		return ItemPage{}, err
	}

	storedPage, err := service.items.ListItems(
		ctx,
		ListItemsStoreInput{
			OwnerID: input.OwnerID,
			VaultID: input.VaultID,
			Options: options,
		},
	)
	if err != nil {
		return ItemPage{}, mapItemOperationError("list vault items", err)
	}

	return validateStoredItemPage(storedPage, input.VaultID, options)
}

func validateStoredItemPage(
	storedPage ItemPage,
	expectedVaultID string,
	options ItemListOptions,
) (ItemPage, error) {
	if len(storedPage.Items) > options.Limit {
		return ItemPage{}, fmt.Errorf("list vault items: %w", ErrItemUnavailable)
	}

	items := make([]Item, 0, len(storedPage.Items))

	for index, storedItem := range storedPage.Items {
		if !validStoredItem(storedItem, expectedVaultID, options.State) {
			return ItemPage{}, fmt.Errorf("list vault items: %w", ErrItemUnavailable)
		}

		if index > 0 && !itemsFollowDescendingKeyset(items[index-1], storedItem) {
			return ItemPage{}, fmt.Errorf("list vault items: %w", ErrItemUnavailable)
		}

		items = append(items, storedItem)
	}

	page := ItemPage{Items: items}

	if storedPage.NextCursor == nil {
		return page, nil
	}

	if len(items) == 0 || len(items) != options.Limit || !storedPage.NextCursor.Valid() {
		return ItemPage{}, fmt.Errorf("list vault items: %w", ErrItemUnavailable)
	}

	lastItem := items[len(items)-1]

	if storedPage.NextCursor.ID != lastItem.ID ||
		!storedPage.NextCursor.UpdatedAt.Equal(lastItem.UpdatedAt) {
		return ItemPage{}, fmt.Errorf("list vault items: %w", ErrItemUnavailable)
	}

	cursor := *storedPage.NextCursor
	cursor.UpdatedAt = cursor.UpdatedAt.UTC()
	page.NextCursor = &cursor

	return page, nil
}

func itemsFollowDescendingKeyset(previous Item, current Item) bool {
	if previous.UpdatedAt.After(current.UpdatedAt) {
		return true
	}

	return previous.UpdatedAt.Equal(current.UpdatedAt) && previous.ID > current.ID
}
