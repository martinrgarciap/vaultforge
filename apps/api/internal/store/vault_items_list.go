package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const listActiveVaultItemsQuery = `
	SELECT
		id::text,
		vault_id::text,
		item_type,
		encrypted_payload,
		nonce,
		version,
		created_at,
		updated_at,
		deleted_at
	FROM vault_items
	WHERE vault_id = $1::uuid
	  AND deleted_at IS NULL
	  AND (
			$2::timestamptz IS NULL
			OR (updated_at, id) < ($2::timestamptz, $3::uuid)
	  )
	ORDER BY
		updated_at DESC,
		id DESC
	LIMIT $4
`

const listDeletedVaultItemsQuery = `
	SELECT
		id::text,
		vault_id::text,
		item_type,
		encrypted_payload,
		nonce,
		version,
		created_at,
		updated_at,
		deleted_at
	FROM vault_items
	WHERE vault_id = $1::uuid
	  AND deleted_at IS NOT NULL
	  AND (
			$2::timestamptz IS NULL
			OR (updated_at, id) < ($2::timestamptz, $3::uuid)
	  )
	ORDER BY
		updated_at DESC,
		id DESC
	LIMIT $4
`

func (store *VaultStore) ListItems(
	ctx context.Context,
	input vaultdomain.ListItemsStoreInput,
) (vaultdomain.ItemPage, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.ItemPage{}, err
	}

	if store == nil || store.database == nil {
		return vaultdomain.ItemPage{}, fmt.Errorf("list vault items: %w", ErrDatabase)
	}

	options, err := vaultdomain.NormalizeItemListOptions(input.Options)
	if err != nil {
		return vaultdomain.ItemPage{}, err
	}

	queryContext, cancelQuery := context.WithTimeout(ctx, queryTimeout)
	defer cancelQuery()

	err = store.requireOwnedVault(
		queryContext,
		input.OwnerID,
		input.VaultID,
	)
	if err != nil {
		return vaultdomain.ItemPage{}, mapListItemsError(err)
	}

	query := listActiveVaultItemsQuery
	if options.State == vaultdomain.ItemListStateDeleted {
		query = listDeletedVaultItemsQuery
	}

	var (
		cursorUpdatedAt any
		cursorID        any
	)

	if options.After != nil {
		cursorUpdatedAt = options.After.UpdatedAt
		cursorID = options.After.ID
	}

	rows, err := store.database.Query(
		queryContext,
		query,
		input.VaultID,
		cursorUpdatedAt,
		cursorID,
		options.Limit+1,
	)
	if err != nil {
		return vaultdomain.ItemPage{}, mapListItemsError(err)
	}
	defer rows.Close()

	items := make([]vaultdomain.Item, 0, options.Limit+1)

	for rows.Next() {
		item, err := scanSyntheticItemRow(rows)
		if err != nil {
			return vaultdomain.ItemPage{}, mapListItemsError(err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return vaultdomain.ItemPage{}, mapListItemsError(err)
	}

	page := vaultdomain.ItemPage{
		Items: items,
	}

	if len(items) > options.Limit {
		lastReturnedItem := items[options.Limit-1]

		page.Items = items[:options.Limit]
		page.NextCursor = &vaultdomain.ItemCursor{
			UpdatedAt: lastReturnedItem.UpdatedAt.UTC(),
			ID:        lastReturnedItem.ID,
		}
	}

	return page, nil
}

func (store *VaultStore) requireOwnedVault(
	ctx context.Context,
	ownerID string,
	vaultID string,
) error {
	var storedVaultID string

	return store.database.QueryRow(
		ctx,
		`
			SELECT id::text
			FROM vaults
			WHERE id = $1::uuid
			  AND owner_id = $2::uuid
		`,
		vaultID,
		ownerID,
	).Scan(&storedVaultID)
}

func mapListItemsError(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrVaultNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("list vault items: %w", err)

	default:
		return fmt.Errorf("list vault items: %w", ErrDatabase)
	}
}
