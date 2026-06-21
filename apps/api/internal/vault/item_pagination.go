package vault

import (
	"strings"
	"time"
)

const (
	DefaultItemPageLimit = 20
	MaxItemPageLimit     = 100
)

type ItemListState string

const (
	ItemListStateActive  ItemListState = "active"
	ItemListStateDeleted ItemListState = "deleted"
)

type ItemCursor struct {
	UpdatedAt time.Time
	ID        string
}

type ItemListOptions struct {
	State ItemListState
	Limit int
	After *ItemCursor
}

type ItemPage struct {
	Items      []Item
	NextCursor *ItemCursor
}

func NormalizeItemListOptions(options ItemListOptions) (ItemListOptions, error) {
	normalized := options

	if normalized.State == "" {
		normalized.State = ItemListStateActive
	}

	if !normalized.State.Valid() {
		return ItemListOptions{}, ErrItemListStateInvalid
	}

	switch {
	case normalized.Limit == 0:
		normalized.Limit = DefaultItemPageLimit
	case normalized.Limit < 1 || normalized.Limit > MaxItemPageLimit:
		return ItemListOptions{}, ErrItemPageLimitInvalid
	}

	if normalized.After != nil {
		if !normalized.After.Valid() {
			return ItemListOptions{}, ErrItemCursorInvalid
		}

		cursor := *normalized.After
		cursor.UpdatedAt = cursor.UpdatedAt.UTC()
		normalized.After = &cursor
	}

	return normalized, nil

}

func (state ItemListState) Valid() bool {
	switch state {
	case ItemListStateActive, ItemListStateDeleted:
		return true
	default:
		return false
	}
}

func (cursor ItemCursor) Valid() bool {
	return !cursor.UpdatedAt.IsZero() &&
		cursor.ID != "" &&
		cursor.ID == strings.TrimSpace(cursor.ID)
}
