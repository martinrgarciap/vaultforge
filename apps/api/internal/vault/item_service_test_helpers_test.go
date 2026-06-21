package vault

import "context"

type itemServiceTestStore struct {
	createResult             Item
	createErr                error
	listResult               ItemPage
	listErr                  error
	getResult                Item
	getErr                   error
	updateResult             Item
	updateErr                error
	softDeleteResult         Item
	softDeleteErr            error
	restoreResult            Item
	restoreErr               error
	permanentDeleteErr       error
	createCalls              int
	listCalls                int
	getCalls                 int
	updateCalls              int
	softDeleteCalls          int
	restoreCalls             int
	permanentDeleteCalls     int
	lastCreateInput          CreateItemStoreInput
	lastListInput            ListItemsStoreInput
	lastGetInput             GetItemStoreInput
	lastUpdateInput          UpdateItemStoreInput
	lastSoftDeleteInput      SoftDeleteItemStoreInput
	lastRestoreInput         RestoreItemStoreInput
	lastPermanentDeleteInput PermanentDeleteItemStoreInput
}

var _ ItemStore = (*itemServiceTestStore)(nil)

func (store *itemServiceTestStore) CreateItem(
	_ context.Context,
	input CreateItemStoreInput,
) (Item, error) {
	store.createCalls++
	store.lastCreateInput = input

	if store.createErr != nil {
		return Item{}, store.createErr
	}

	return store.createResult, nil
}

func (store *itemServiceTestStore) ListItems(
	_ context.Context,
	input ListItemsStoreInput,
) (ItemPage, error) {
	store.listCalls++
	store.lastListInput = input

	if store.listErr != nil {
		return ItemPage{}, store.listErr
	}

	return store.listResult, nil
}

func (store *itemServiceTestStore) GetItem(
	_ context.Context,
	input GetItemStoreInput,
) (Item, error) {
	store.getCalls++
	store.lastGetInput = input

	if store.getErr != nil {
		return Item{}, store.getErr
	}

	return store.getResult, nil
}

func (store *itemServiceTestStore) UpdateItem(
	_ context.Context,
	input UpdateItemStoreInput,
) (Item, error) {
	store.updateCalls++
	store.lastUpdateInput = input

	if store.updateErr != nil {
		return Item{}, store.updateErr
	}

	return store.updateResult, nil
}

func (store *itemServiceTestStore) SoftDeleteItem(
	_ context.Context,
	input SoftDeleteItemStoreInput,
) (Item, error) {
	store.softDeleteCalls++
	store.lastSoftDeleteInput = input

	if store.softDeleteErr != nil {
		return Item{}, store.softDeleteErr
	}

	return store.softDeleteResult, nil
}

func (store *itemServiceTestStore) RestoreItem(
	_ context.Context,
	input RestoreItemStoreInput,
) (Item, error) {
	store.restoreCalls++
	store.lastRestoreInput = input

	if store.restoreErr != nil {
		return Item{}, store.restoreErr
	}

	return store.restoreResult, nil
}

func (store *itemServiceTestStore) PermanentDeleteItem(
	_ context.Context,
	input PermanentDeleteItemStoreInput,
) error {
	store.permanentDeleteCalls++
	store.lastPermanentDeleteInput = input

	return store.permanentDeleteErr
}
