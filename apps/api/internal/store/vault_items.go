package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

var _ vaultdomain.ItemStore = (*VaultStore)(nil)

func (store *VaultStore) CreateItem(
	ctx context.Context,
	input vaultdomain.CreateItemStoreInput,
) (vaultdomain.Item, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.Item{}, err
	}

	if store == nil || store.database == nil {
		return vaultdomain.Item{}, fmt.Errorf("create vault item: %w", ErrDatabase)
	}

	queryContext, cancelQuery := context.WithTimeout(ctx, queryTimeout)
	defer cancelQuery()

	transaction, err := store.database.Begin(queryContext)
	if err != nil {
		return vaultdomain.Item{}, mapCreateItemError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(queryContext)
		}
	}()

	createdItem, err := createItemInTransaction(queryContext, transaction, input)
	if err != nil {
		return vaultdomain.Item{}, mapCreateItemError(err)
	}

	if err := insertInitialItemVersionInTransaction(
		queryContext,
		transaction,
		createdItem.ID,
	); err != nil {
		return vaultdomain.Item{}, mapCreateItemError(err)
	}

	if err := insertItemAuditEventInTransaction(
		queryContext,
		transaction,
		"vault_item.created",
		createdItem.ID,
		input.OwnerID,
		input.CorrelationID,
	); err != nil {
		return vaultdomain.Item{}, mapCreateItemError(err)
	}

	if err := transaction.Commit(queryContext); err != nil {
		return vaultdomain.Item{}, mapCreateItemError(err)
	}

	committed = true

	return createdItem, nil
}

func createItemInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	input vaultdomain.CreateItemStoreInput,
) (vaultdomain.Item, error) {
	return scanSyntheticItemRow(
		transaction.QueryRow(
			ctx,
			`
				INSERT INTO vault_items (
					vault_id,
					item_type,
					encrypted_payload,
					nonce
				)
				SELECT
					vaults.id,
					$3,
					$4::bytea,
					$5::bytea
				FROM vaults
				WHERE vaults.id = $1::uuid
				  AND vaults.owner_id = $2::uuid
				RETURNING
					id::text,
					vault_id::text,
					item_type,
					encrypted_payload,
					nonce,
					version,
					created_at,
					updated_at,
					deleted_at
			`,
			input.VaultID,
			input.OwnerID,
			string(input.Type),
			input.Envelope.Payload,
			input.Envelope.Nonce,
		),
	)
}

func insertInitialItemVersionInTransaction(
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

func insertItemAuditEventInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	eventType string,
	itemID string,
	actorID string,
	correlationID string,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO audit_outbox (
				event_type,
				aggregate_type,
				aggregate_id,
				actor_id,
				correlation_id,
				sanitized_payload
			)
			VALUES (
				$1,
				'vault_item',
				$2::uuid,
				$3::uuid,
				$4,
				'{}'::jsonb
			)
		`,
		eventType,
		itemID,
		actorID,
		correlationID,
	)

	return err
}

type itemRowScanner interface {
	Scan(destinations ...any) error
}

func scanSyntheticItemRow(row itemRowScanner) (vaultdomain.Item, error) {
	var (
		storedItem       vaultdomain.Item
		storedItemType   string
		encryptedPayload []byte
		nonce            []byte
		deletedAt        pgtype.Timestamptz
	)

	err := row.Scan(
		&storedItem.ID,
		&storedItem.VaultID,
		&storedItemType,
		&encryptedPayload,
		&nonce,
		&storedItem.Version,
		&storedItem.CreatedAt,
		&storedItem.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return vaultdomain.Item{}, err
	}

	itemType, err := vaultdomain.ParseItemType(storedItemType)
	if err != nil {
		return vaultdomain.Item{}, fmt.Errorf("scan vault item type: %w", err)
	}

	payload, err := vaultdomain.NormalizeSyntheticItemPayload(encryptedPayload)
	if err != nil {
		return vaultdomain.Item{}, fmt.Errorf("scan vault item payload: %w", err)
	}

	if !vaultdomain.IsSyntheticItemNonce(nonce) {
		return vaultdomain.Item{}, errors.New("vault item nonce is not synthetic")
	}

	storedItem.Type = itemType
	storedItem.Payload = payload

	if deletedAt.Valid {
		value := deletedAt.Time
		storedItem.DeletedAt = &value
	}

	return storedItem, nil
}

func mapCreateItemError(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrVaultNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("create vault item: %w", err)

	default:
		return fmt.Errorf("create vault item: %w", ErrDatabase)
	}
}
