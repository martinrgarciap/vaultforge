package vault

import (
	"errors"
	"testing"
	"time"
)

const itemPaginationTestID = "00000000-0000-0000-0000-000000000901"

func TestNormalizeItemListOptionsAppliesDefaults(t *testing.T) {
	t.Parallel()

	options, err := NormalizeItemListOptions(ItemListOptions{})
	if err != nil {
		t.Fatalf("NormalizeItemListOptions() error = %v", err)
	}

	if options.State != ItemListStateActive {
		t.Fatalf("state = %q, want %q", options.State, ItemListStateActive)
	}

	if options.Limit != DefaultItemPageLimit {
		t.Fatalf("limit = %d, want %d", options.Limit, DefaultItemPageLimit)
	}

	if options.After != nil {
		t.Fatal("default options unexpectedly contained a cursor")
	}

}

func TestNormalizeItemListOptionsAcceptsDeletedStateAndMaximumLimit(t *testing.T) {
	t.Parallel()

	options, err := NormalizeItemListOptions(ItemListOptions{
		State: ItemListStateDeleted,
		Limit: MaxItemPageLimit,
	})
	if err != nil {
		t.Fatalf("NormalizeItemListOptions() error = %v", err)
	}

	if options.State != ItemListStateDeleted {
		t.Fatalf("state = %q, want %q", options.State, ItemListStateDeleted)
	}

	if options.Limit != MaxItemPageLimit {
		t.Fatalf("limit = %d, want %d", options.Limit, MaxItemPageLimit)
	}

}

func TestNormalizeItemListOptionsRejectsInvalidState(t *testing.T) {
	t.Parallel()

	options, err := NormalizeItemListOptions(ItemListOptions{
		State: "all",
		Limit: DefaultItemPageLimit,
	})

	if !errors.Is(err, ErrItemListStateInvalid) {
		t.Fatalf("error = %v, want %v", err, ErrItemListStateInvalid)
	}

	if options != (ItemListOptions{}) {
		t.Fatalf("options = %+v, want zero value", options)
	}

}

func TestNormalizeItemListOptionsRejectsInvalidLimits(t *testing.T) {
	t.Parallel()

	tests := []int{-1, MaxItemPageLimit + 1}

	for _, limit := range tests {
		limit := limit

		t.Run("", func(t *testing.T) {
			t.Parallel()

			options, err := NormalizeItemListOptions(ItemListOptions{Limit: limit})

			if !errors.Is(err, ErrItemPageLimitInvalid) {
				t.Fatalf("error = %v, want %v", err, ErrItemPageLimitInvalid)
			}

			if options != (ItemListOptions{}) {
				t.Fatalf("options = %+v, want zero value", options)
			}
		})
	}

}

func TestNormalizeItemListOptionsCopiesAndNormalizesCursor(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("pagination-test", -4*60*60)
	updatedAt := time.Date(2026, time.June, 21, 12, 30, 0, 0, location)

	cursor := &ItemCursor{
		UpdatedAt: updatedAt,
		ID:        itemPaginationTestID,
	}

	options, err := NormalizeItemListOptions(ItemListOptions{
		Limit: 10,
		After: cursor,
	})
	if err != nil {
		t.Fatalf("NormalizeItemListOptions() error = %v", err)
	}

	if options.After == cursor {
		t.Fatal("normalized options retained the caller's cursor pointer")
	}

	if !options.After.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("cursor time = %v, want %v", options.After.UpdatedAt, updatedAt)
	}

	if options.After.UpdatedAt.Location() != time.UTC {
		t.Fatalf("cursor location = %v, want UTC", options.After.UpdatedAt.Location())
	}

	if options.After.ID != itemPaginationTestID {
		t.Fatalf("cursor ID = %q, want %q", options.After.ID, itemPaginationTestID)
	}

}

func TestNormalizeItemListOptionsRejectsInvalidCursors(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.June, 21, 16, 30, 0, 0, time.UTC)

	tests := []struct {
		name   string
		cursor ItemCursor
	}{
		{
			name:   "zero timestamp",
			cursor: ItemCursor{ID: itemPaginationTestID},
		},
		{
			name:   "empty ID",
			cursor: ItemCursor{UpdatedAt: updatedAt},
		},
		{
			name: "whitespace ID",
			cursor: ItemCursor{
				UpdatedAt: updatedAt,
				ID:        "   ",
			},
		},
		{
			name: "surrounding whitespace",
			cursor: ItemCursor{
				UpdatedAt: updatedAt,
				ID:        " " + itemPaginationTestID,
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			options, err := NormalizeItemListOptions(ItemListOptions{
				After: &test.cursor,
			})

			if !errors.Is(err, ErrItemCursorInvalid) {
				t.Fatalf("error = %v, want %v", err, ErrItemCursorInvalid)
			}

			if options != (ItemListOptions{}) {
				t.Fatalf("options = %+v, want zero value", options)
			}
		})
	}

}

func TestItemListStateValid(t *testing.T) {
	t.Parallel()

	if !ItemListStateActive.Valid() {
		t.Fatal("active item list state was invalid")
	}

	if !ItemListStateDeleted.Valid() {
		t.Fatal("deleted item list state was invalid")
	}

	if ItemListState("all").Valid() {
		t.Fatal("unsupported item list state was valid")
	}

}
