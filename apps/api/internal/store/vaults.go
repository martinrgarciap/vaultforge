package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

type VaultStore struct {
	database *pgxpool.Pool
}

func NewVaultStore(
	database *pgxpool.Pool,
) *VaultStore {
	return &VaultStore{
		database: database,
	}
}

var _ vaultdomain.Store = (*VaultStore)(nil)

func (store *VaultStore) Create(
	ctx context.Context,
	input vaultdomain.CreateStoreInput,
) (vaultdomain.Vault, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.Vault{}, err
	}

	if store == nil ||
		store.database == nil {
		return vaultdomain.Vault{},
			fmt.Errorf(
				"create vault: %w",
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
			mapCreateVaultError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(
				queryContext,
			)
		}
	}()

	createdVault, err :=
		createVaultInTransaction(
			queryContext,
			transaction,
			input.Vault,
		)
	if err != nil {
		return vaultdomain.Vault{},
			mapCreateVaultError(err)
	}

	err = insertVaultAuditEventInTransaction(
		queryContext,
		transaction,
		"vault.created",
		createdVault.ID,
		createdVault.OwnerID,
		input.CorrelationID,
	)
	if err != nil {
		return vaultdomain.Vault{},
			mapCreateVaultError(err)
	}

	if err := transaction.Commit(
		queryContext,
	); err != nil {
		return vaultdomain.Vault{},
			mapCreateVaultError(err)
	}

	committed = true

	return createdVault, nil
}

func (store *VaultStore) ListOwned(
	ctx context.Context,
	ownerID string,
) ([]vaultdomain.Vault, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if store == nil ||
		store.database == nil {
		return nil,
			fmt.Errorf(
				"list owned vaults: %w",
				ErrDatabase,
			)
	}

	queryContext, cancelQuery := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancelQuery()

	rows, err := store.database.Query(
		queryContext,
		`
			SELECT
				id::text,
				owner_id::text,
				name,
				crypto_version,
				kdf_version,
				created_at,
				updated_at
			FROM vaults
			WHERE owner_id = $1::uuid
			ORDER BY
				updated_at DESC,
				id DESC
		`,
		ownerID,
	)
	if err != nil {
		return nil,
			mapListOwnedVaultsError(err)
	}
	defer rows.Close()

	ownedVaults := make(
		[]vaultdomain.Vault,
		0,
	)

	for rows.Next() {
		storedVault, err := scanVaultRow(rows)
		if err != nil {
			return nil,
				mapListOwnedVaultsError(err)
		}

		ownedVaults = append(
			ownedVaults,
			storedVault,
		)
	}

	if err := rows.Err(); err != nil {
		return nil,
			mapListOwnedVaultsError(err)
	}

	return ownedVaults, nil
}

func (store *VaultStore) GetOwned(
	ctx context.Context,
	ownerID string,
	vaultID string,
) (vaultdomain.Vault, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.Vault{}, err
	}

	if store == nil ||
		store.database == nil {
		return vaultdomain.Vault{},
			fmt.Errorf(
				"get owned vault: %w",
				ErrDatabase,
			)
	}

	queryContext, cancelQuery := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancelQuery()

	storedVault, err := scanVaultRow(
		store.database.QueryRow(
			queryContext,
			`
				SELECT
					id::text,
					owner_id::text,
					name,
					crypto_version,
					kdf_version,
					created_at,
					updated_at
				FROM vaults
				WHERE id = $1::uuid
				  AND owner_id = $2::uuid
			`,
			vaultID,
			ownerID,
		),
	)
	if err != nil {
		return vaultdomain.Vault{},
			mapGetOwnedVaultError(err)
	}

	return storedVault, nil
}

func (store *VaultStore) RenameOwned(
	ctx context.Context,
	input vaultdomain.RenameStoreInput,
) (vaultdomain.Vault, error) {
	if err := ctx.Err(); err != nil {
		return vaultdomain.Vault{}, err
	}

	if store == nil ||
		store.database == nil {
		return vaultdomain.Vault{},
			fmt.Errorf(
				"rename owned vault: %w",
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
			mapRenameOwnedVaultError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(
				queryContext,
			)
		}
	}()

	renamedVault, err := scanVaultRow(
		transaction.QueryRow(
			queryContext,
			`
				UPDATE vaults
				SET
					name = $3,
					updated_at = clock_timestamp()
				WHERE id = $1::uuid
				  AND owner_id = $2::uuid
				RETURNING
					id::text,
					owner_id::text,
					name,
					crypto_version,
					kdf_version,
					created_at,
					updated_at
			`,
			input.VaultID,
			input.OwnerID,
			input.Name,
		),
	)
	if err != nil {
		return vaultdomain.Vault{},
			mapRenameOwnedVaultError(err)
	}

	err = insertVaultAuditEventInTransaction(
		queryContext,
		transaction,
		"vault.renamed",
		renamedVault.ID,
		renamedVault.OwnerID,
		input.CorrelationID,
	)
	if err != nil {
		return vaultdomain.Vault{},
			mapRenameOwnedVaultError(err)
	}

	if err := transaction.Commit(
		queryContext,
	); err != nil {
		return vaultdomain.Vault{},
			mapRenameOwnedVaultError(err)
	}

	committed = true

	return renamedVault, nil
}

func (store *VaultStore) DeleteOwned(
	ctx context.Context,
	input vaultdomain.DeleteStoreInput,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if store == nil ||
		store.database == nil {
		return fmt.Errorf(
			"delete owned vault: %w",
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
		return mapDeleteOwnedVaultError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(
				queryContext,
			)
		}
	}()

	var (
		deletedVaultID string
		deletedOwnerID string
	)

	err = transaction.QueryRow(
		queryContext,
		`
			DELETE FROM vaults
			WHERE id = $1::uuid
			  AND owner_id = $2::uuid
			RETURNING
				id::text,
				owner_id::text
		`,
		input.VaultID,
		input.OwnerID,
	).Scan(
		&deletedVaultID,
		&deletedOwnerID,
	)
	if err != nil {
		return mapDeleteOwnedVaultError(err)
	}

	err = insertVaultAuditEventInTransaction(
		queryContext,
		transaction,
		"vault.deleted",
		deletedVaultID,
		deletedOwnerID,
		input.CorrelationID,
	)
	if err != nil {
		return mapDeleteOwnedVaultError(err)
	}

	if err := transaction.Commit(
		queryContext,
	); err != nil {
		return mapDeleteOwnedVaultError(err)
	}

	committed = true

	return nil
}

type vaultRowScanner interface {
	Scan(destinations ...any) error
}

func scanVaultRow(
	row vaultRowScanner,
) (vaultdomain.Vault, error) {
	var (
		storedVault   vaultdomain.Vault
		cryptoVersion pgtype.Int2
		kdfVersion    pgtype.Int2
	)

	err := row.Scan(
		&storedVault.ID,
		&storedVault.OwnerID,
		&storedVault.Name,
		&cryptoVersion,
		&kdfVersion,
		&storedVault.CreatedAt,
		&storedVault.UpdatedAt,
	)
	if err != nil {
		return vaultdomain.Vault{}, err
	}

	if cryptoVersion.Valid {
		value := cryptoVersion.Int16
		storedVault.CryptoVersion = &value
	}

	if kdfVersion.Valid {
		value := kdfVersion.Int16
		storedVault.KDFVersion = &value
	}

	return storedVault, nil
}

func createVaultInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	input vaultdomain.Vault,
) (vaultdomain.Vault, error) {
	query := `
		INSERT INTO vaults (
			owner_id,
			name
		)
		VALUES (
			$1::uuid,
			$2
		)
		RETURNING
			id::text,
			owner_id::text,
			name,
			crypto_version,
			kdf_version,
			created_at,
			updated_at
	`

	return scanVaultRow(
		transaction.QueryRow(
			ctx,
			query,
			input.OwnerID,
			input.Name,
		),
	)
}

func insertVaultAuditEventInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	eventType string,
	vaultID string,
	actorID string,
	correlationID string,
) error {
	sanitizedPayload, err := newVaultAuditPayload()
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
				'vault',
				$2::uuid,
				$3::uuid,
				$4,
				$5::jsonb
			)
		`,
		eventType,
		vaultID,
		actorID,
		correlationID,
		sanitizedPayload,
	)

	return err
}

func mapCreateVaultError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"create vault: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"create vault: %w",
			ErrDatabase,
		)
	}
}

func mapListOwnedVaultsError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"list owned vaults: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"list owned vaults: %w",
			ErrDatabase,
		)
	}
}

func mapGetOwnedVaultError(
	err error,
) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrVaultNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"get owned vault: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"get owned vault: %w",
			ErrDatabase,
		)
	}
}

func mapRenameOwnedVaultError(
	err error,
) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrVaultNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"rename owned vault: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"rename owned vault: %w",
			ErrDatabase,
		)
	}
}

func mapDeleteOwnedVaultError(
	err error,
) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return vaultdomain.ErrVaultNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"delete owned vault: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"delete owned vault: %w",
			ErrDatabase,
		)
	}
}
