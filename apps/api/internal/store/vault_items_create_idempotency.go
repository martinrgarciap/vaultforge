package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const itemCreateIdempotencyTTL = 24 * time.Hour

var errItemCreateReplayMissing = errors.New("idempotent vault item resource is missing")

type itemCreateIdempotencyClaim struct {
	ResourceID  string
	RequestHash []byte
	Claimed     bool
}

func claimItemCreateIdempotency(
	ctx context.Context,
	transaction pgx.Tx,
	input vaultdomain.CreateItemStoreInput,
) (itemCreateIdempotencyClaim, error) {
	var (
		candidateResourceID string
		claimTime           time.Time
	)

	err := transaction.QueryRow(
		ctx,
		`
			SELECT
				gen_random_uuid()::text,
				clock_timestamp()
		`,
	).Scan(&candidateResourceID, &claimTime)
	if err != nil {
		return itemCreateIdempotencyClaim{}, err
	}

	expiresAt := claimTime.Add(itemCreateIdempotencyTTL)

	var (
		storedResourceID  string
		storedRequestHash []byte
	)

	err = transaction.QueryRow(
		ctx,
		`
			INSERT INTO idempotency_records (
				actor_id,
				operation,
				scope_id,
				idempotency_key_hash,
				request_hash,
				resource_id,
				created_at,
				expires_at
			)
			VALUES (
				$1::uuid,
				$2,
				$3::uuid,
				$4::bytea,
				$5::bytea,
				$6::uuid,
				$7,
				$8
			)
			ON CONFLICT (
				actor_id,
				operation,
				scope_id,
				idempotency_key_hash
			)
			DO UPDATE SET
				request_hash = CASE
					WHEN idempotency_records.expires_at <= EXCLUDED.created_at
						THEN EXCLUDED.request_hash
					ELSE idempotency_records.request_hash
				END,
				resource_id = CASE
					WHEN idempotency_records.expires_at <= EXCLUDED.created_at
						THEN EXCLUDED.resource_id
					ELSE idempotency_records.resource_id
				END,
				created_at = CASE
					WHEN idempotency_records.expires_at <= EXCLUDED.created_at
						THEN EXCLUDED.created_at
					ELSE idempotency_records.created_at
				END,
				expires_at = CASE
					WHEN idempotency_records.expires_at <= EXCLUDED.created_at
						THEN EXCLUDED.expires_at
					ELSE idempotency_records.expires_at
				END
			RETURNING
				resource_id::text,
				request_hash
		`,
		input.OwnerID,
		vaultdomain.ItemCreateOperation,
		input.VaultID,
		input.Idempotency.KeyHash[:],
		input.Idempotency.RequestHash[:],
		candidateResourceID,
		claimTime,
		expiresAt,
	).Scan(&storedResourceID, &storedRequestHash)
	if err != nil {
		return itemCreateIdempotencyClaim{}, err
	}

	if len(storedRequestHash) != sha256.Size {
		return itemCreateIdempotencyClaim{}, errors.New("stored item request hash has invalid length")
	}

	return itemCreateIdempotencyClaim{
		ResourceID:  storedResourceID,
		RequestHash: storedRequestHash,
		Claimed:     storedResourceID == candidateResourceID,
	}, nil
}

func loadItemCreateReplayInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	input vaultdomain.CreateItemStoreInput,
	resourceID string,
) (vaultdomain.Item, error) {
	return scanSyntheticItemRow(
		transaction.QueryRow(
			ctx,
			`
				SELECT
					items.id::text,
					items.vault_id::text,
					versions.item_type,
					versions.encrypted_payload,
					versions.nonce,
					versions.version,
					items.created_at,
					items.created_at,
					NULL::timestamptz
				FROM vault_items AS items
				JOIN vaults
				  ON vaults.id = items.vault_id
				JOIN item_versions AS versions
				  ON versions.vault_item_id = items.id
				 AND versions.version = 1
				WHERE items.id = $1::uuid
				  AND items.vault_id = $2::uuid
				  AND vaults.owner_id = $3::uuid
			`,
			resourceID,
			input.VaultID,
			input.OwnerID,
		),
	)
}
