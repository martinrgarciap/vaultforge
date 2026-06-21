package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func (store *VaultStore) PermanentDeleteItem(
	ctx context.Context,
	input vaultdomain.PermanentDeleteItemStoreInput,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if store == nil || store.database == nil {
		return fmt.Errorf("permanently delete vault item: %w", ErrDatabase)
	}

	queryContext, cancelQuery := context.WithTimeout(ctx, queryTimeout)
	defer cancelQuery()

	transaction, err := store.database.Begin(queryContext)
	if err != nil {
		return mapPermanentDeleteItemError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(queryContext)
		}
	}()

	var deletedItemID string

	err = transaction.QueryRow(
		queryContext,
		`
		DELETE FROM vault_items AS items
		USING vaults
		WHERE items.id = $1::uuid
		  AND items.vault_id = $2::uuid
		  AND vaults.id = items.vault_id
		  AND vaults.owner_id = $3::uuid
		  AND items.deleted_at IS NOT NULL
		RETURNING items.id::text
	`,
		input.ItemID,
		input.VaultID,
		input.OwnerID,
	).Scan(&deletedItemID)
	if err != nil {
		return mapPermanentDeleteItemError(err)
	}

	if err := insertItemAuditEventInTransaction(
		queryContext,
		transaction,
		"vault_item.permanently_deleted",
		deletedItemID,
		input.OwnerID,
		input.CorrelationID,
	); err != nil {
		return mapPermanentDeleteItemError(err)
	}

	if err := transaction.Commit(queryContext); err != nil {
		return mapPermanentDeleteItemError(err)
	}

	committed = true

	return nil

}

func mapPermanentDeleteItemError(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrItemNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("permanently delete vault item: %w", err)

	default:
		return fmt.Errorf("permanently delete vault item: %w", ErrDatabase)
	}

}
