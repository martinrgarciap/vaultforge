package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type schemaFixture struct {
	userID  string
	vaultID string
	itemID  string
}

func TestSchemaRejectsMissingSessionUser(t *testing.T) {
	resetIntegrationTestTables(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	_, err := testDatabasePool.Exec(
		ctx,
		`
			INSERT INTO sessions (
				user_id,
				refresh_token_hash,
				expires_at
			)
			VALUES (
				gen_random_uuid(),
				decode('00112233', 'hex'),
				now() + interval '1 day'
			)
		`,
	)

	if err == nil {
		t.Fatal("expected missing user foreign key to fail")
	}

	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		t.Fatalf(
			"expected PostgreSQL error, got %v",
			err,
		)
	}

	if postgresError.Code != "23503" {
		t.Errorf(
			"expected foreign-key violation code 23503, got %q",
			postgresError.Code,
		)
	}
}

func TestSchemaCascadesUserDeletion(t *testing.T) {
	fixture := createSchemaFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	_, err := testDatabasePool.Exec(
		ctx,
		`
			INSERT INTO sessions (
				user_id,
				refresh_token_hash,
				expires_at
			)
			VALUES (
				$1::uuid,
				decode('0011223344556677', 'hex'),
				now() + interval '30 days'
			)
		`,
		fixture.userID,
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err = testDatabasePool.Exec(
		ctx,
		`
			INSERT INTO item_versions (
				vault_item_id,
				version,
				item_type,
				encrypted_payload,
				nonce
			)
			VALUES (
				$1::uuid,
				1,
				'secure_note',
				decode('1122334455667788', 'hex'),
				decode('aabbccddeeff0011', 'hex')
			)
		`,
		fixture.itemID,
	)
	if err != nil {
		t.Fatalf("create item version: %v", err)
	}

	_, err = testDatabasePool.Exec(
		ctx,
		`
			DELETE FROM users
			WHERE id = $1::uuid
		`,
		fixture.userID,
	)
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var (
		userCount        int
		sessionCount     int
		vaultCount       int
		itemCount        int
		itemVersionCount int
	)

	err = testDatabasePool.QueryRow(
		ctx,
		`
			SELECT
				(SELECT count(*) FROM users),
				(SELECT count(*) FROM sessions),
				(SELECT count(*) FROM vaults),
				(SELECT count(*) FROM vault_items),
				(SELECT count(*) FROM item_versions)
		`,
	).Scan(
		&userCount,
		&sessionCount,
		&vaultCount,
		&itemCount,
		&itemVersionCount,
	)
	if err != nil {
		t.Fatalf("count remaining records: %v", err)
	}

	if userCount != 0 ||
		sessionCount != 0 ||
		vaultCount != 0 ||
		itemCount != 0 ||
		itemVersionCount != 0 {
		t.Fatalf(
			"expected all records deleted, got users=%d sessions=%d vaults=%d items=%d versions=%d",
			userCount,
			sessionCount,
			vaultCount,
			itemCount,
			itemVersionCount,
		)
	}
}

func TestSchemaTransactionRollback(t *testing.T) {
	resetIntegrationTestTables(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	transaction, err := testDatabasePool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO users (
				email,
				password_hash,
				password_algorithm
			)
			VALUES (
				'rollback-user@example.com',
				'dummy-rollback-hash',
				'test'
			)
		`,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)

		t.Fatalf("insert user in transaction: %v", err)
	}

	if err := transaction.Rollback(ctx); err != nil {
		t.Fatalf("roll back transaction: %v", err)
	}

	var userCount int

	err = testDatabasePool.QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM users
			WHERE email = 'rollback-user@example.com'
		`,
	).Scan(&userCount)
	if err != nil {
		t.Fatalf("count rolled-back user: %v", err)
	}

	if userCount != 0 {
		t.Fatalf(
			"expected transaction rollback to remove user, got %d records",
			userCount,
		)
	}
}

func TestSchemaSupportsVaultItemSoftDeletion(t *testing.T) {
	fixture := createSchemaFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	var (
		version   int
		deletedAt time.Time
	)

	err := testDatabasePool.QueryRow(
		ctx,
		`
			UPDATE vault_items
			SET
				deleted_at = now(),
				updated_at = now(),
				version = version + 1
			WHERE id = $1::uuid
			  AND version = 1
			RETURNING
				version,
				deleted_at
		`,
		fixture.itemID,
	).Scan(
		&version,
		&deletedAt,
	)
	if err != nil {
		t.Fatalf("soft delete item: %v", err)
	}

	if version != 2 {
		t.Errorf(
			"expected item version 2, got %d",
			version,
		)
	}

	if deletedAt.IsZero() {
		t.Error("expected deletion timestamp")
	}

	var (
		activeCount  int
		deletedCount int
	)

	err = testDatabasePool.QueryRow(
		ctx,
		`
			SELECT
				count(*) FILTER (
					WHERE deleted_at IS NULL
				),
				count(*) FILTER (
					WHERE deleted_at IS NOT NULL
				)
			FROM vault_items
			WHERE vault_id = $1::uuid
		`,
		fixture.vaultID,
	).Scan(
		&activeCount,
		&deletedCount,
	)
	if err != nil {
		t.Fatalf("count soft-deleted items: %v", err)
	}

	if activeCount != 0 {
		t.Errorf(
			"expected no active items, got %d",
			activeCount,
		)
	}

	if deletedCount != 1 {
		t.Errorf(
			"expected one deleted item, got %d",
			deletedCount,
		)
	}
}

func TestSchemaRejectsStaleVaultItemVersion(t *testing.T) {
	fixture := createSchemaFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	var version int

	err := testDatabasePool.QueryRow(
		ctx,
		`
			UPDATE vault_items
			SET
				encrypted_payload = decode(
					'ffeeddccbbaa9988',
					'hex'
				),
				nonce = decode(
					'8877665544332211',
					'hex'
				),
				version = version + 1,
				updated_at = now()
			WHERE id = $1::uuid
			  AND version = 1
			RETURNING version
		`,
		fixture.itemID,
	).Scan(&version)
	if err != nil {
		t.Fatalf("perform first item update: %v", err)
	}

	if version != 2 {
		t.Errorf(
			"expected version 2, got %d",
			version,
		)
	}

	err = testDatabasePool.QueryRow(
		ctx,
		`
			UPDATE vault_items
			SET
				encrypted_payload = decode(
					'0102030405060708',
					'hex'
				),
				version = version + 1,
				updated_at = now()
			WHERE id = $1::uuid
			  AND version = 1
			RETURNING version
		`,
		fixture.itemID,
	).Scan(&version)

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected stale update to affect no rows, got %v",
			err,
		)
	}
}

func TestSchemaContainsNoPlaintextSecretColumns(
	t *testing.T,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	rows, err := testDatabasePool.Query(
		ctx,
		`
			SELECT
				table_name,
				column_name
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name IN (
				  'vaults',
				  'vault_items',
				  'item_versions'
			  )
		`,
	)
	if err != nil {
		t.Fatalf("read schema columns: %v", err)
	}
	defer rows.Close()

	forbiddenColumns := map[string]struct{}{
		"password":          {},
		"username":          {},
		"title":             {},
		"notes":             {},
		"api_key":           {},
		"secret":            {},
		"secret_value":      {},
		"connection_string": {},
		"plaintext_payload": {},
	}

	for rows.Next() {
		var (
			tableName  string
			columnName string
		)

		if err := rows.Scan(
			&tableName,
			&columnName,
		); err != nil {
			t.Fatalf("scan schema column: %v", err)
		}

		if _, forbidden := forbiddenColumns[columnName]; forbidden {
			t.Errorf(
				"found forbidden plaintext column %s.%s",
				tableName,
				columnName,
			)
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema columns: %v", err)
	}
}

func createSchemaFixture(
	t *testing.T,
) schemaFixture {
	t.Helper()

	resetIntegrationTestTables(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	var fixture schemaFixture

	err := testDatabasePool.QueryRow(
		ctx,
		`
			INSERT INTO users (
				email,
				password_hash,
				password_algorithm
			)
			VALUES (
				'schema-fixture@example.com',
				'dummy-schema-hash',
				'test'
			)
			RETURNING id::text
		`,
	).Scan(&fixture.userID)
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	err = testDatabasePool.QueryRow(
		ctx,
		`
			INSERT INTO vaults (
				owner_id,
				name
			)
			VALUES (
				$1::uuid,
				'Schema Test Vault'
			)
			RETURNING id::text
		`,
		fixture.userID,
	).Scan(&fixture.vaultID)
	if err != nil {
		t.Fatalf("create fixture vault: %v", err)
	}

	err = testDatabasePool.QueryRow(
		ctx,
		`
			INSERT INTO vault_items (
				vault_id,
				item_type,
				encrypted_payload,
				nonce
			)
			VALUES (
				$1::uuid,
				'secure_note',
				decode(
					'0011223344556677',
					'hex'
				),
				decode(
					'aabbccddeeff0011',
					'hex'
				)
			)
			RETURNING id::text
		`,
		fixture.vaultID,
	).Scan(&fixture.itemID)
	if err != nil {
		t.Fatalf("create fixture item: %v", err)
	}

	return fixture
}
