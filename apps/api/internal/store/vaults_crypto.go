package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func (store *VaultStore) InitializeCryptoMetadataOwned(
	ctx context.Context,
	input vaultdomain.InitializeCryptoMetadataStoreInput,
) (vaultdomain.Vault, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.Vault{}, err
	}

	if store == nil ||
		store.database == nil {
		return vaultdomain.Vault{},
			fmt.Errorf(
				"initialize vault crypto metadata: %w",
				ErrDatabase,
			)
	}

	queryContext, cancelQuery := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancelQuery()

	transaction, err := store.database.Begin(
		queryContext,
	)
	if err != nil {
		return vaultdomain.Vault{},
			mapInitializeVaultCryptoMetadataError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(
				queryContext,
			)
		}
	}()

	initializedVault, err := scanVaultRow(
		transaction.QueryRow(
			queryContext,
			`
				UPDATE vaults
				SET
					crypto_version = $3,
					kdf_version = $4,
					salt = $5,
					wrapped_key = $6,
					updated_at = clock_timestamp()
				WHERE id = $1::uuid
				  AND owner_id = $2::uuid
				  AND crypto_version IS NULL
				  AND kdf_version IS NULL
				  AND salt IS NULL
				  AND wrapped_key IS NULL
				RETURNING
					id::text,
					owner_id::text,
					name,
					crypto_version,
					kdf_version,
					salt,
					wrapped_key,
					created_at,
					updated_at
			`,
			input.VaultID,
			input.OwnerID,
			input.Metadata.CryptoVersion,
			input.Metadata.KDFVersion,
			input.Metadata.Salt,
			input.Metadata.WrappedKey,
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return vaultdomain.Vault{},
				mapInitializeVaultCryptoMetadataNoRows(
					queryContext,
					transaction,
					input,
				)
		}

		return vaultdomain.Vault{},
			mapInitializeVaultCryptoMetadataError(err)
	}

	err = insertVaultAuditEventInTransaction(
		queryContext,
		transaction,
		"vault.crypto_initialized",
		initializedVault.ID,
		initializedVault.OwnerID,
		input.CorrelationID,
	)
	if err != nil {
		return vaultdomain.Vault{},
			mapInitializeVaultCryptoMetadataError(err)
	}

	if err := transaction.Commit(
		queryContext,
	); err != nil {
		return vaultdomain.Vault{},
			mapInitializeVaultCryptoMetadataError(err)
	}

	committed = true

	return initializedVault, nil
}

func mapInitializeVaultCryptoMetadataNoRows(
	ctx context.Context,
	transaction pgx.Tx,
	input vaultdomain.InitializeCryptoMetadataStoreInput,
) error {
	var vaultExists bool

	err := transaction.QueryRow(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM vaults
				WHERE id = $1::uuid
				  AND owner_id = $2::uuid
			)
		`,
		input.VaultID,
		input.OwnerID,
	).Scan(&vaultExists)
	if err != nil {
		return mapInitializeVaultCryptoMetadataError(err)
	}

	if vaultExists {
		return vaultdomain.ErrVaultCryptoMetadataAlreadyInitialized
	}

	return vaultdomain.ErrVaultNotFound
}

func mapInitializeVaultCryptoMetadataError(
	err error,
) error {
	switch {
	case errors.Is(err, vaultdomain.ErrVaultCryptoMetadataAlreadyInitialized):
		return vaultdomain.ErrVaultCryptoMetadataAlreadyInitialized

	case errors.Is(err, vaultdomain.ErrVaultNotFound):
		return vaultdomain.ErrVaultNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"initialize vault crypto metadata: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"initialize vault crypto metadata: %w",
			ErrDatabase,
		)
	}
}
