package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSchemaSupportsSanitizedAuditOutbox(
	t *testing.T,
) {
	fixture := createSchemaFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	var (
		eventID  string
		status   string
		attempts int
		itemType string
	)

	err := testDatabasePool.QueryRow(
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
				'vault_item.created',
				'vault_item',
				$1::uuid,
				$2::uuid,
				'synthetic-request-id',
				jsonb_build_object(
					'itemType',
					'secure_note',
					'version',
					1
				)
			)
			RETURNING
				id::text,
				status,
				attempts,
				sanitized_payload ->> 'itemType'
		`,
		fixture.itemID,
		fixture.userID,
	).Scan(
		&eventID,
		&status,
		&attempts,
		&itemType,
	)
	if err != nil {
		t.Fatal(
			"create sanitized audit outbox event",
		)
	}

	if eventID == "" {
		t.Fatal(
			"audit outbox event did not receive an ID",
		)
	}

	if status != "pending" {
		t.Fatalf(
			"audit outbox status = %q, want pending",
			status,
		)
	}

	if attempts != 0 {
		t.Fatalf(
			"audit outbox attempts = %d, want 0",
			attempts,
		)
	}

	if itemType != "secure_note" {
		t.Fatalf(
			"audit item type = %q, want secure_note",
			itemType,
		)
	}
}

func TestSchemaRejectsNonObjectAuditPayload(
	t *testing.T,
) {
	fixture := createSchemaFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	_, err := testDatabasePool.Exec(
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
				'vault_item.created',
				'vault_item',
				$1::uuid,
				$2::uuid,
				'synthetic-request-id',
				'["not-an-object"]'::jsonb
			)
		`,
		fixture.itemID,
		fixture.userID,
	)

	assertSchemaPostgresError(
		t,
		err,
		"23514",
		"audit_outbox_sanitized_payload_object",
	)
}

func TestSchemaKeepsAuditOutboxAfterAggregateDeletion(
	t *testing.T,
) {
	fixture := createSchemaFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	var eventID string

	err := testDatabasePool.QueryRow(
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
				'vault_item.permanently_deleted',
				'vault_item',
				$1::uuid,
				$2::uuid,
				'synthetic-request-id',
				jsonb_build_object(
					'itemType',
					'secure_note',
					'version',
					1
				)
			)
			RETURNING id::text
		`,
		fixture.itemID,
		fixture.userID,
	).Scan(&eventID)
	if err != nil {
		t.Fatal(
			"create permanent-delete audit event",
		)
	}

	_, err = testDatabasePool.Exec(
		ctx,
		`
			DELETE FROM vault_items
			WHERE id = $1::uuid
		`,
		fixture.itemID,
	)
	if err != nil {
		t.Fatal(
			"delete audit event aggregate",
		)
	}

	var eventCount int

	err = testDatabasePool.QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM audit_outbox
			WHERE id = $1::uuid
		`,
		eventID,
	).Scan(&eventCount)
	if err != nil {
		t.Fatal(
			"count surviving audit event",
		)
	}

	if eventCount != 1 {
		t.Fatalf(
			"surviving audit event count = %d, want 1",
			eventCount,
		)
	}
}

func TestSchemaScopesIdempotencyKeys(
	t *testing.T,
) {
	fixture := createSchemaFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	err := insertSchemaIdempotencyRecord(
		ctx,
		fixture,
		fixture.vaultID,
	)
	if err != nil {
		t.Fatal(
			"create first idempotency record",
		)
	}

	err = insertSchemaIdempotencyRecord(
		ctx,
		fixture,
		fixture.vaultID,
	)

	assertSchemaPostgresError(
		t,
		err,
		"23505",
		"idempotency_records_scope_key_unique",
	)

	err = insertSchemaIdempotencyRecord(
		ctx,
		fixture,
		fixture.userID,
	)
	if err != nil {
		t.Fatal(
			"reuse idempotency key in a different scope",
		)
	}
}

func TestSchemaRejectsInvalidIdempotencyHashLength(
	t *testing.T,
) {
	fixture := createSchemaFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	_, err := testDatabasePool.Exec(
		ctx,
		`
			INSERT INTO idempotency_records (
				actor_id,
				operation,
				scope_id,
				idempotency_key_hash,
				request_hash,
				resource_id,
				expires_at
			)
			VALUES (
				$1::uuid,
				'vault_item.create',
				$2::uuid,
				decode('01', 'hex'),
				decode(
					repeat('22', 32),
					'hex'
				),
				$3::uuid,
				now() + interval '24 hours'
			)
		`,
		fixture.userID,
		fixture.vaultID,
		fixture.itemID,
	)

	assertSchemaPostgresError(
		t,
		err,
		"23514",
		"idempotency_records_key_hash_length",
	)
}

func TestSchemaRejectsExpiredIdempotencyRecord(
	t *testing.T,
) {
	fixture := createSchemaFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	_, err := testDatabasePool.Exec(
		ctx,
		`
			INSERT INTO idempotency_records (
				actor_id,
				operation,
				scope_id,
				idempotency_key_hash,
				request_hash,
				resource_id,
				expires_at
			)
			VALUES (
				$1::uuid,
				'vault_item.create',
				$2::uuid,
				decode(
					repeat('11', 32),
					'hex'
				),
				decode(
					repeat('22', 32),
					'hex'
				),
				$3::uuid,
				now() - interval '1 second'
			)
		`,
		fixture.userID,
		fixture.vaultID,
		fixture.itemID,
	)

	assertSchemaPostgresError(
		t,
		err,
		"23514",
		"idempotency_records_expires_after_creation",
	)
}

func TestSchemaStoresOnlyHashedIdempotencyKeys(
	t *testing.T,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	var (
		hashedColumnCount    int
		plaintextColumnCount int
		hashedColumnType     string
	)

	err := testDatabasePool.QueryRow(
		ctx,
		`
			SELECT
				count(*) FILTER (
					WHERE column_name =
						'idempotency_key_hash'
				),
				count(*) FILTER (
					WHERE column_name =
						'idempotency_key'
				),
				COALESCE(
					max(data_type) FILTER (
						WHERE column_name =
							'idempotency_key_hash'
					),
					''
				)
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name =
				'idempotency_records'
		`,
	).Scan(
		&hashedColumnCount,
		&plaintextColumnCount,
		&hashedColumnType,
	)
	if err != nil {
		t.Fatal(
			"inspect idempotency schema",
		)
	}

	if hashedColumnCount != 1 {
		t.Fatalf(
			"idempotency hash column count = %d, want 1",
			hashedColumnCount,
		)
	}

	if plaintextColumnCount != 0 {
		t.Fatalf(
			"plaintext idempotency column count = %d, want 0",
			plaintextColumnCount,
		)
	}

	if hashedColumnType != "bytea" {
		t.Fatalf(
			"idempotency hash column type = %q, want bytea",
			hashedColumnType,
		)
	}
}

func TestSchemaUsesStableVaultItemPaginationIndexes(
	t *testing.T,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancel()

	tests := []struct {
		name      string
		indexName string
		predicate string
	}{
		{
			name:      "active items",
			indexName: "vault_items_active_by_vault_updated_at_idx",
			predicate: "WHERE (deleted_at IS NULL)",
		},
		{
			name:      "deleted items",
			indexName: "vault_items_deleted_by_vault_updated_at_idx",
			predicate: "WHERE (deleted_at IS NOT NULL)",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			var definition string

			err := testDatabasePool.QueryRow(
				ctx,
				`
					SELECT indexdef
					FROM pg_indexes
					WHERE schemaname = 'public'
					  AND indexname = $1
				`,
				test.indexName,
			).Scan(&definition)
			if err != nil {
				t.Fatal(
					"read vault item pagination index",
				)
			}

			normalizedDefinition := strings.Join(
				strings.Fields(definition),
				" ",
			)

			if !strings.Contains(
				normalizedDefinition,
				"(vault_id, updated_at DESC, id DESC)",
			) {
				t.Fatalf(
					"index %q does not include the stable pagination order",
					test.indexName,
				)
			}

			if !strings.Contains(
				normalizedDefinition,
				test.predicate,
			) {
				t.Fatalf(
					"index %q does not include the expected deletion predicate",
					test.indexName,
				)
			}
		})
	}
}

func insertSchemaIdempotencyRecord(
	ctx context.Context,
	fixture schemaFixture,
	scopeID string,
) error {
	_, err := testDatabasePool.Exec(
		ctx,
		`
			INSERT INTO idempotency_records (
				actor_id,
				operation,
				scope_id,
				idempotency_key_hash,
				request_hash,
				resource_id,
				expires_at
			)
			VALUES (
				$1::uuid,
				'vault_item.create',
				$2::uuid,
				decode(
					repeat('11', 32),
					'hex'
				),
				decode(
					repeat('22', 32),
					'hex'
				),
				$3::uuid,
				now() + interval '24 hours'
			)
		`,
		fixture.userID,
		scopeID,
		fixture.itemID,
	)

	return err
}

func assertSchemaPostgresError(
	t *testing.T,
	err error,
	wantCode string,
	wantConstraint string,
) {
	t.Helper()

	if err == nil {
		t.Fatal(
			"expected PostgreSQL constraint failure",
		)
	}

	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		t.Fatal(
			"expected PostgreSQL constraint error",
		)
	}

	if postgresError.Code != wantCode {
		t.Fatalf(
			"PostgreSQL code = %q, want %q",
			postgresError.Code,
			wantCode,
		)
	}

	if postgresError.ConstraintName !=
		wantConstraint {
		t.Fatalf(
			"constraint = %q, want %q",
			postgresError.ConstraintName,
			wantConstraint,
		)
	}
}
