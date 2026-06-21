package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func (store *VaultStore) UpdateItem(
	ctx context.Context,
	input vaultdomain.UpdateItemStoreInput,
) (vaultdomain.Item, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.Item{}, err
	}

	if store == nil || store.database == nil {
		return vaultdomain.Item{}, fmt.Errorf("update vault item: %w", ErrDatabase)
	}

	queryContext, cancelQuery := context.WithTimeout(ctx, queryTimeout)
	defer cancelQuery()

	transaction, err := store.database.Begin(queryContext)
	if err != nil {
		return vaultdomain.Item{}, mapUpdateItemError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(queryContext)
		}
	}()

	updatedItem, err := updateItemInTransaction(queryContext, transaction, input)
	if err != nil {
		return vaultdomain.Item{}, mapUpdateItemError(err)
	}

	if err := insertCurrentItemVersionInTransaction(
		queryContext,
		transaction,
		updatedItem.ID,
	); err != nil {
		return vaultdomain.Item{}, mapUpdateItemError(err)
	}

	if err := insertItemAuditEventInTransaction(
		queryContext,
		transaction,
		"vault_item.updated",
		updatedItem.ID,
		input.OwnerID,
		input.CorrelationID,
	); err != nil {
		return vaultdomain.Item{}, mapUpdateItemError(err)
	}

	if err := transaction.Commit(queryContext); err != nil {
		return vaultdomain.Item{}, mapUpdateItemError(err)
	}

	committed = true

	return updatedItem, nil
}

func updateItemInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	input vaultdomain.UpdateItemStoreInput,
) (vaultdomain.Item, error) {
	return scanSyntheticItemRow(
		transaction.QueryRow(
			ctx,
			`
				UPDATE vault_items AS items
				SET
					item_type = $4,
					encrypted_payload = $5::bytea,
					nonce = $6::bytea,
					version = items.version + 1,
					updated_at = clock_timestamp()
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
			string(input.Type),
			input.Envelope.Payload,
			input.Envelope.Nonce,
		),
	)
}

func insertCurrentItemVersionInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	itemID string,
) error {
	result, err := transaction.Exec(
		ctx,
		`
			INSERT INTO item_versions (
				vault_item_id,
				version,
				item_type,
				encrypted_payload,
				nonce
			)
			SELECT
				id,
				version,
				item_type,
				encrypted_payload,
				nonce
			FROM vault_items
			WHERE id = $1::uuid
		`,
		itemID,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}

	return nil
}

func mapUpdateItemError(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrItemNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("update vault item: %w", err)

	default:
		return fmt.Errorf("update vault item: %w", ErrDatabase)
	}
}
