package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func (store *VaultStore) RestoreItem(
	ctx context.Context,
	input vaultdomain.RestoreItemStoreInput,
) (vaultdomain.Item, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.Item{}, err
	}

	if store == nil || store.database == nil {
		return vaultdomain.Item{}, fmt.Errorf("restore vault item: %w", ErrDatabase)
	}

	queryContext, cancelQuery := context.WithTimeout(ctx, queryTimeout)
	defer cancelQuery()

	transaction, err := store.database.Begin(queryContext)
	if err != nil {
		return vaultdomain.Item{}, mapRestoreItemError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(queryContext)
		}
	}()

	restoredItem, err := restoreItemInTransaction(queryContext, transaction, input)
	if errors.Is(err, pgx.ErrNoRows) {
		err = classifyRestoreItemMiss(queryContext, transaction, input)
	}
	if err != nil {
		return vaultdomain.Item{}, mapRestoreItemError(err)
	}

	if err := insertCurrentItemVersionInTransaction(
		queryContext,
		transaction,
		restoredItem.ID,
	); err != nil {
		return vaultdomain.Item{}, mapRestoreItemError(err)
	}

	if err := insertItemAuditEventInTransaction(
		queryContext,
		transaction,
		"vault_item.restored",
		restoredItem.ID,
		restoredItem.VaultID,
		restoredItem.Type,
		restoredItem.Version,
		input.OwnerID,
		input.CorrelationID,
	); err != nil {
		return vaultdomain.Item{}, mapRestoreItemError(err)
	}

	if err := transaction.Commit(queryContext); err != nil {
		return vaultdomain.Item{}, mapRestoreItemError(err)
	}

	committed = true

	return restoredItem, nil
}

func restoreItemInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	input vaultdomain.RestoreItemStoreInput,
) (vaultdomain.Item, error) {
	return scanSyntheticItemRow(
		transaction.QueryRow(
			ctx,
			`
				UPDATE vault_items AS items
				SET
					version = items.version + 1,
					updated_at = clock_timestamp(),
					deleted_at = NULL
				FROM vaults
				WHERE items.id = $1::uuid
				  AND items.vault_id = $2::uuid
				  AND vaults.id = items.vault_id
				  AND vaults.owner_id = $3::uuid
				  AND items.deleted_at IS NOT NULL
				  AND items.version = $4
				RETURNING
					items.id::text,
					items.vault_id::text,
					items.item_type,
					items.encrypted_payload,
					items.nonce,
					items.version,
					items.created_at,
					items.updated_at,
					items.deleted_at
			`,
			input.ItemID,
			input.VaultID,
			input.OwnerID,
			input.ExpectedVersion,
		),
	)
}

func classifyRestoreItemMiss(
	ctx context.Context,
	transaction pgx.Tx,
	input vaultdomain.RestoreItemStoreInput,
) error {
	var currentVersion int

	err := transaction.QueryRow(
		ctx,
		`
			SELECT items.version
			FROM vault_items AS items
			JOIN vaults
			  ON vaults.id = items.vault_id
			WHERE items.id = $1::uuid
			  AND items.vault_id = $2::uuid
			  AND vaults.owner_id = $3::uuid
			  AND items.deleted_at IS NOT NULL
		`,
		input.ItemID,
		input.VaultID,
		input.OwnerID,
	).Scan(&currentVersion)
	if err != nil {
		return err
	}

	return vaultdomain.ErrItemConflict
}

func mapRestoreItemError(err error) error {
	switch {
	case errors.Is(err, vaultdomain.ErrItemConflict):
		return vaultdomain.ErrItemConflict

	case errors.Is(err, vaultdomain.ErrItemNotFound),
		errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrItemNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("restore vault item: %w", err)

	default:
		return fmt.Errorf("restore vault item: %w", ErrDatabase)
	}
}
