package itemhandler

import (
	"encoding/json"
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
	ID        string          `json:"id"`
	Type      vault.ItemType  `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Version   int             `json:"version"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *time.Time      `json:"deletedAt,omitempty"`
}

func newItemResource(storedItem vault.Item) itemResource {
	return itemResource{
		ID:        storedItem.ID,
		Type:      storedItem.Type,
		Payload:   storedItem.Payload,
		Version:   storedItem.Version,
		CreatedAt: storedItem.CreatedAt,
		UpdatedAt: storedItem.UpdatedAt,
		DeletedAt: storedItem.DeletedAt,
	}
}

func newItemListResponse(page vault.ItemPage) (itemListResponse, error) {
	items := make([]itemResource, 0, len(page.Items))

	for _, storedItem := range page.Items {
		items = append(items, newItemResource(storedItem))
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
