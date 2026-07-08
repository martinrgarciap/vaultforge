package itemhandler

import (
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

type itemResponse struct {
	Item itemResource `json:"item"`
}

type itemListResponse struct {
	Items      []itemResource `json:"items"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

type itemResource struct {
	ID               string                       `json:"id"`
	Type             vault.ItemType               `json:"type"`
	EncryptedPayload itemEncryptedPayloadResource `json:"encryptedPayload"`
	Version          int                          `json:"version"`
	CreatedAt        time.Time                    `json:"createdAt"`
	UpdatedAt        time.Time                    `json:"updatedAt"`
	DeletedAt        *time.Time                   `json:"deletedAt,omitempty"`
}

func newItemResource(storedItem vault.Item) (itemResource, error) {
	if len(storedItem.Nonce) == 0 || vault.IsSyntheticItemNonce(storedItem.Nonce) {
		return itemResource{}, vault.ErrItemEncryptedPayloadInvalid
	}

	encryptedPayload, err := newItemEncryptedPayloadResource(
		vault.EncryptedItemEnvelope{
			Payload: storedItem.Payload,
			Nonce:   storedItem.Nonce,
		},
	)
	if err != nil {
		return itemResource{}, err
	}

	return itemResource{
		ID:               storedItem.ID,
		Type:             storedItem.Type,
		EncryptedPayload: encryptedPayload,
		Version:          storedItem.Version,
		CreatedAt:        storedItem.CreatedAt,
		UpdatedAt:        storedItem.UpdatedAt,
		DeletedAt:        storedItem.DeletedAt,
	}, nil
}

func newItemListResponse(page vault.ItemPage) (itemListResponse, error) {
	items := make([]itemResource, 0, len(page.Items))

	for _, storedItem := range page.Items {
		resource, err := newItemResource(storedItem)
		if err != nil {
			return itemListResponse{}, err
		}

		items = append(items, resource)
	}

	result := itemListResponse{
		Items: items,
	}

	if page.NextCursor == nil {
		return result, nil
	}

	nextCursor, err := encodeItemCursor(*page.NextCursor)
	if err != nil {
		return itemListResponse{}, err
	}

	result.NextCursor = nextCursor

	return result, nil
}
