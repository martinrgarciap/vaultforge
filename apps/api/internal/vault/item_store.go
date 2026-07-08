package vault

import "context"

type ItemStore interface {
	CreateItem(ctx context.Context, input CreateItemStoreInput) (Item, error)
	ListItems(ctx context.Context, input ListItemsStoreInput) (ItemPage, error)
	GetItem(ctx context.Context, input GetItemStoreInput) (Item, error)
	UpdateItem(ctx context.Context, input UpdateItemStoreInput) (Item, error)
	SoftDeleteItem(ctx context.Context, input SoftDeleteItemStoreInput) (Item, error)
	RestoreItem(ctx context.Context, input RestoreItemStoreInput) (Item, error)
	PermanentDeleteItem(ctx context.Context, input PermanentDeleteItemStoreInput) error
}

type CreateItemStoreInput struct {
	OwnerID       string
	VaultID       string
	Type          ItemType
	Envelope      ItemEnvelope
	Idempotency   ItemCreateIdempotency
	CorrelationID string
}

type ListItemsStoreInput struct {
	OwnerID string
	VaultID string
	Options ItemListOptions
}

type GetItemStoreInput struct {
	OwnerID string
	VaultID string
	ItemID  string
	State   ItemListState
}

type UpdateItemStoreInput struct {
	OwnerID         string
	VaultID         string
	ItemID          string
	Type            ItemType
	Envelope        ItemEnvelope
	ExpectedVersion int
	CorrelationID   string
}

type SoftDeleteItemStoreInput struct {
	OwnerID         string
	VaultID         string
	ItemID          string
	ExpectedVersion int
	CorrelationID   string
}

type RestoreItemStoreInput struct {
	OwnerID         string
	VaultID         string
	ItemID          string
	ExpectedVersion int
	CorrelationID   string
}

type PermanentDeleteItemStoreInput struct {
	OwnerID         string
	VaultID         string
	ItemID          string
	ExpectedVersion int
	CorrelationID   string
}
