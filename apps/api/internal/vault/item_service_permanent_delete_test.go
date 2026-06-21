package vault

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const itemServicePermanentDeleteRequest = "item-service-permanent-delete-request"

func TestServicePermanentDeleteItemDeletesOwnedItem(t *testing.T) {
	t.Parallel()

	store := &itemServiceTestStore{}
	service := &Service{items: store}

	err := service.PermanentDeleteItem(
		context.Background(),
		validItemServicePermanentDeleteInput(),
	)
	if err != nil {
		t.Fatalf("PermanentDeleteItem() error = %v", err)
	}

	if store.permanentDeleteCalls != 1 {
		t.Fatalf(
			"PermanentDeleteItem() store calls = %d, want 1",
			store.permanentDeleteCalls,
		)
	}

	if store.lastPermanentDeleteInput.OwnerID != itemServiceTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			store.lastPermanentDeleteInput.OwnerID,
			itemServiceTestOwnerID,
		)
	}

	if store.lastPermanentDeleteInput.VaultID != itemServiceTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			store.lastPermanentDeleteInput.VaultID,
			itemServiceTestVaultID,
		)
	}

	if store.lastPermanentDeleteInput.ItemID != itemServiceTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			store.lastPermanentDeleteInput.ItemID,
			itemServiceTestItemID,
		)
	}

	if store.lastPermanentDeleteInput.CorrelationID != itemServicePermanentDeleteRequest {
		t.Fatalf(
			"correlation ID = %q, want %q",
			store.lastPermanentDeleteInput.CorrelationID,
			itemServicePermanentDeleteRequest,
		)
	}
}

func TestServicePermanentDeleteItemRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   PermanentDeleteItemInput
		wantErr error
	}{
		{
			name: "invalid owner",
			input: PermanentDeleteItemInput{
				VaultID:       itemServiceTestVaultID,
				ItemID:        itemServiceTestItemID,
				CorrelationID: itemServicePermanentDeleteRequest,
			},
			wantErr: ErrOwnerInvalid,
		},
		{
			name: "invalid vault",
			input: PermanentDeleteItemInput{
				OwnerID:       itemServiceTestOwnerID,
				VaultID:       " ",
				ItemID:        itemServiceTestItemID,
				CorrelationID: itemServicePermanentDeleteRequest,
			},
			wantErr: ErrVaultNotFound,
		},
		{
			name: "invalid item",
			input: PermanentDeleteItemInput{
				OwnerID:       itemServiceTestOwnerID,
				VaultID:       itemServiceTestVaultID,
				ItemID:        "",
				CorrelationID: itemServicePermanentDeleteRequest,
			},
			wantErr: ErrItemNotFound,
		},
		{
			name: "invalid correlation ID",
			input: PermanentDeleteItemInput{
				OwnerID: itemServiceTestOwnerID,
				VaultID: itemServiceTestVaultID,
				ItemID:  itemServiceTestItemID,
			},
			wantErr: ErrCorrelationIDInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{}
			service := &Service{items: store}

			err := service.PermanentDeleteItem(context.Background(), test.input)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"PermanentDeleteItem() error = %v, want %v",
					err,
					test.wantErr,
				)
			}

			if store.permanentDeleteCalls != 0 {
				t.Fatal("item store was called for invalid permanent-delete input")
			}
		})
	}
}

func TestServicePermanentDeleteItemMapsStoreErrorsSafely(t *testing.T) {
	t.Parallel()

	const internalMarker = "synthetic permanent-delete database failure"

	tests := []struct {
		name     string
		storeErr error
		wantErr  error
	}{
		{
			name:     "item not found",
			storeErr: ErrItemNotFound,
			wantErr:  ErrItemNotFound,
		},
		{
			name:     "context deadline",
			storeErr: context.DeadlineExceeded,
			wantErr:  context.DeadlineExceeded,
		},
		{
			name:     "internal failure",
			storeErr: errors.New(internalMarker),
			wantErr:  ErrItemUnavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &itemServiceTestStore{
				permanentDeleteErr: test.storeErr,
			}
			service := &Service{items: store}

			err := service.PermanentDeleteItem(
				context.Background(),
				validItemServicePermanentDeleteInput(),
			)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"PermanentDeleteItem() error = %v, want %v",
					err,
					test.wantErr,
				)
			}

			if strings.Contains(err.Error(), internalMarker) {
				t.Fatal("PermanentDeleteItem() exposed an internal store failure")
			}
		})
	}
}

func TestServicePermanentDeleteItemRejectsUnavailableDependency(t *testing.T) {
	t.Parallel()

	service := &Service{}

	err := service.PermanentDeleteItem(
		context.Background(),
		validItemServicePermanentDeleteInput(),
	)

	if !errors.Is(err, ErrItemUnavailable) {
		t.Fatalf(
			"PermanentDeleteItem() error = %v, want %v",
			err,
			ErrItemUnavailable,
		)
	}
}

func TestServicePermanentDeleteItemPreservesCanceledContext(t *testing.T) {
	t.Parallel()

	store := &itemServiceTestStore{}
	service := &Service{items: store}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := service.PermanentDeleteItem(
		ctx,
		validItemServicePermanentDeleteInput(),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"PermanentDeleteItem() error = %v, want %v",
			err,
			context.Canceled,
		)
	}

	if store.permanentDeleteCalls != 0 {
		t.Fatal("item store was called after context cancellation")
	}
}

func validItemServicePermanentDeleteInput() PermanentDeleteItemInput {
	return PermanentDeleteItemInput{
		OwnerID:       itemServiceTestOwnerID,
		VaultID:       itemServiceTestVaultID,
		ItemID:        itemServiceTestItemID,
		CorrelationID: itemServicePermanentDeleteRequest,
	}
}
