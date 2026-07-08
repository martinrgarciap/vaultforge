package store

import (
	"bytes"
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

	claim, err := claimItemCreateIdempotency(queryContext, transaction, input)
	if err != nil {
		return vaultdomain.Item{}, mapCreateItemError(err)
	}

	if !bytes.Equal(claim.RequestHash, input.Idempotency.RequestHash[:]) {
		return vaultdomain.Item{}, vaultdomain.ErrItemIdempotencyConflict
	}

	if !claim.Claimed {
		replayedItem, err := loadItemCreateReplayInTransaction(
			queryContext,
			transaction,
			input,
			claim.ResourceID,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			err = errItemCreateReplayMissing
		}
		if err != nil {
			return vaultdomain.Item{}, mapCreateItemError(err)
		}

		if err := transaction.Commit(queryContext); err != nil {
			return vaultdomain.Item{}, mapCreateItemError(err)
		}

		committed = true

		return replayedItem, nil
	}

	createdItem, err := createItemInTransaction(
		queryContext,
		transaction,
		claim.ResourceID,
		input,
	)
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
		createdItem.VaultID,
		createdItem.Type,
		createdItem.Version,
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
	itemID string,
	input vaultdomain.CreateItemStoreInput,
) (vaultdomain.Item, error) {
	return scanSyntheticItemRow(
		transaction.QueryRow(
			ctx,
			`
				INSERT INTO vault_items (
					id,
					vault_id,
					item_type,
					encrypted_payload,
					nonce
				)
				SELECT
					$1::uuid,
					vaults.id,
					$4,
					$5::bytea,
					$6::bytea
				FROM vaults
				WHERE vaults.id = $2::uuid
				  AND vaults.owner_id = $3::uuid
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
			itemID,
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
	vaultID string,
	itemType vaultdomain.ItemType,
	version int,
	actorID string,
	correlationID string,
) error {
	sanitizedPayload, err := newVaultItemAuditPayload(
		vaultID,
		itemType,
		version,
	)
	if err != nil {
		return err
	}

	_, err = transaction.Exec(
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
				$5::jsonb
			)
		`,
		eventType,
		itemID,
		actorID,
		correlationID,
		sanitizedPayload,
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

	envelope, err := vaultdomain.NewItemEnvelopeFromStorage(
		encryptedPayload,
		nonce,
	)
	if err != nil {
		return vaultdomain.Item{}, fmt.Errorf("scan vault item payload: %w", err)
	}

	storedItem.Type = itemType
	storedItem.Payload = envelope.Payload
	storedItem.Nonce = envelope.Nonce

	if deletedAt.Valid {
		value := deletedAt.Time
		storedItem.DeletedAt = &value
	}

	return storedItem, nil
}

func mapCreateItemError(err error) error {
	switch {
	case errors.Is(err, vaultdomain.ErrItemIdempotencyConflict):
		return vaultdomain.ErrItemIdempotencyConflict

	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrVaultNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("create vault item: %w", err)

	default:
		return fmt.Errorf("create vault item: %w", ErrDatabase)
	}
}
