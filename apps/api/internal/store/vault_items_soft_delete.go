package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func (store *VaultStore) SoftDeleteItem(
	ctx context.Context,
	input vaultdomain.SoftDeleteItemStoreInput,
) (vaultdomain.Item, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.Item{}, err
	}

	if store == nil || store.database == nil {
		return vaultdomain.Item{}, fmt.Errorf("soft delete vault item: %w", ErrDatabase)
	}

	queryContext, cancelQuery := context.WithTimeout(ctx, queryTimeout)
	defer cancelQuery()

	transaction, err := store.database.Begin(queryContext)
	if err != nil {
		return vaultdomain.Item{}, mapSoftDeleteItemError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(queryContext)
		}
	}()

	deletedItem, err := softDeleteItemInTransaction(queryContext, transaction, input)
	if err != nil {
		return vaultdomain.Item{}, mapSoftDeleteItemError(err)
	}

	if err := insertItemAuditEventInTransaction(
		queryContext,
		transaction,
		"vault_item.deleted",
		deletedItem.ID,
		input.OwnerID,
		input.CorrelationID,
	); err != nil {
		return vaultdomain.Item{}, mapSoftDeleteItemError(err)
	}

	if err := transaction.Commit(queryContext); err != nil {
		return vaultdomain.Item{}, mapSoftDeleteItemError(err)
	}

	committed = true

	return deletedItem, nil
}

func softDeleteItemInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	input vaultdomain.SoftDeleteItemStoreInput,
) (vaultdomain.Item, error) {
	return scanSyntheticItemRow(
		transaction.QueryRow(
			ctx,
			`
				UPDATE vault_items AS items
				SET
					updated_at = clock_timestamp(),
					deleted_at = clock_timestamp()
				FROM vaults
				WHERE items.id = $1::uuid
				  AND items.vault_id = $2::uuid
				  AND vaults.id = items.vault_id
				  AND vaults.owner_id = $3::uuid
				  AND items.deleted_at IS NULL
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
		),
	)
}

func mapSoftDeleteItemError(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrItemNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("soft delete vault item: %w", err)

	default:
		return fmt.Errorf("soft delete vault item: %w", ErrDatabase)
	}
}
