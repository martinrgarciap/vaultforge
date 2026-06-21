package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const getActiveVaultItemQuery = `
	SELECT
		vault_items.id::text,
		vault_items.vault_id::text,
		vault_items.item_type,
		vault_items.encrypted_payload,
		vault_items.nonce,
		vault_items.version,
		vault_items.created_at,
		vault_items.updated_at,
		vault_items.deleted_at
	FROM vault_items
	JOIN vaults
	  ON vaults.id = vault_items.vault_id
	WHERE vault_items.id = $1::uuid
	  AND vault_items.vault_id = $2::uuid
	  AND vaults.owner_id = $3::uuid
	  AND vault_items.deleted_at IS NULL
`

const getDeletedVaultItemQuery = `
	SELECT
		vault_items.id::text,
		vault_items.vault_id::text,
		vault_items.item_type,
		vault_items.encrypted_payload,
		vault_items.nonce,
		vault_items.version,
		vault_items.created_at,
		vault_items.updated_at,
		vault_items.deleted_at
	FROM vault_items
	JOIN vaults
	  ON vaults.id = vault_items.vault_id
	WHERE vault_items.id = $1::uuid
	  AND vault_items.vault_id = $2::uuid
	  AND vaults.owner_id = $3::uuid
	  AND vault_items.deleted_at IS NOT NULL
`

func (store *VaultStore) GetItem(
	ctx context.Context,
	input vaultdomain.GetItemStoreInput,
) (vaultdomain.Item, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.Item{}, err
	}

	if store == nil || store.database == nil {
		return vaultdomain.Item{}, fmt.Errorf("get vault item: %w", ErrDatabase)
	}

	state := input.State
	if state == "" {
		state = vaultdomain.ItemListStateActive
	}

	if !state.Valid() {
		return vaultdomain.Item{}, vaultdomain.ErrItemListStateInvalid
	}

	query := getActiveVaultItemQuery
	if state == vaultdomain.ItemListStateDeleted {
		query = getDeletedVaultItemQuery
	}

	queryContext, cancelQuery := context.WithTimeout(ctx, queryTimeout)
	defer cancelQuery()

	item, err := scanSyntheticItemRow(
		store.database.QueryRow(
			queryContext,
			query,
			input.ItemID,
			input.VaultID,
			input.OwnerID,
		),
	)
	if err != nil {
		return vaultdomain.Item{}, mapGetItemError(err)
	}

	return item, nil
}

func mapGetItemError(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrItemNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("get vault item: %w", err)

	default:
		return fmt.Errorf("get vault item: %w", ErrDatabase)
	}
}
