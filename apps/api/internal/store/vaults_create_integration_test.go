package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const vaultCreateTestCorrelationID = "00000000-0000-0000-0000-000000000401"

func TestVaultStoreCreateCommitsVaultAndAuditEvent(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	user := createVaultTestUser(
		t,
		userStore,
		"vault-create@example.com",
	)

	createdVault, err := vaultStore.Create(
		context.Background(),
		vaultdomain.CreateStoreInput{
			Vault: vaultdomain.Vault{
				OwnerID: user.ID,
				Name:    "Development",
			},
			CorrelationID: vaultCreateTestCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf(
			"create vault: %v",
			err,
		)
	}

	if createdVault.ID == "" {
		t.Fatal(
			"created vault did not receive an ID",
		)
	}

	if createdVault.OwnerID != user.ID {
		t.Fatalf(
			"created owner ID = %q, want %q",
			createdVault.OwnerID,
			user.ID,
		)
	}

	if createdVault.Name != "Development" {
		t.Fatalf(
			"created vault name = %q, want Development",
			createdVault.Name,
		)
	}

	if createdVault.CreatedAt.IsZero() ||
		createdVault.UpdatedAt.IsZero() {
		t.Fatal(
			"created vault did not receive timestamps",
		)
	}

	if createdVault.CryptoVersion != nil ||
		createdVault.KDFVersion != nil {
		t.Fatal(
			"dummy-data vault unexpectedly contained crypto versions",
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var (
		storedOwnerID       string
		storedName          string
		cryptoVersionIsNull bool
		kdfVersionIsNull    bool
		saltIsNull          bool
		wrappedKeyIsNull    bool
		storedCreatedAt     time.Time
		storedUpdatedAt     time.Time
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				owner_id::text,
				name,
				crypto_version IS NULL,
				kdf_version IS NULL,
				salt IS NULL,
				wrapped_key IS NULL,
				created_at,
				updated_at
			FROM vaults
			WHERE id = $1::uuid
		`,
		createdVault.ID,
	).Scan(
		&storedOwnerID,
		&storedName,
		&cryptoVersionIsNull,
		&kdfVersionIsNull,
		&saltIsNull,
		&wrappedKeyIsNull,
		&storedCreatedAt,
		&storedUpdatedAt,
	)
	if err != nil {
		t.Fatal(
			"read created vault",
		)
	}

	if storedOwnerID != user.ID {
		t.Fatalf(
			"stored owner ID = %q, want %q",
			storedOwnerID,
			user.ID,
		)
	}

	if storedName != "Development" {
		t.Fatalf(
			"stored vault name = %q, want Development",
			storedName,
		)
	}

	if !cryptoVersionIsNull ||
		!kdfVersionIsNull ||
		!saltIsNull ||
		!wrappedKeyIsNull {
		t.Fatal(
			"dummy-data vault stored unexpected cryptographic metadata",
		)
	}

	if !storedCreatedAt.Equal(
		createdVault.CreatedAt,
	) {
		t.Fatal(
			"returned creation time did not match PostgreSQL",
		)
	}

	if !storedUpdatedAt.Equal(
		createdVault.UpdatedAt,
	) {
		t.Fatal(
			"returned update time did not match PostgreSQL",
		)
	}

	var (
		eventType        string
		aggregateType    string
		aggregateID      string
		actorID          string
		correlationID    string
		sanitizedPayload string
		status           string
		attempts         int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				event_type,
				aggregate_type,
				aggregate_id::text,
				actor_id::text,
				correlation_id,
				sanitized_payload::text,
				status,
				attempts
			FROM audit_outbox
			WHERE aggregate_id = $1::uuid
		`,
		createdVault.ID,
	).Scan(
		&eventType,
		&aggregateType,
		&aggregateID,
		&actorID,
		&correlationID,
		&sanitizedPayload,
		&status,
		&attempts,
	)
	if err != nil {
		t.Fatal(
			"read vault audit event",
		)
	}

	if eventType != "vault.created" {
		t.Fatalf(
			"event type = %q, want vault.created",
			eventType,
		)
	}

	if aggregateType != "vault" {
		t.Fatalf(
			"aggregate type = %q, want vault",
			aggregateType,
		)
	}

	if aggregateID != createdVault.ID {
		t.Fatalf(
			"aggregate ID = %q, want %q",
			aggregateID,
			createdVault.ID,
		)
	}

	if actorID != user.ID {
		t.Fatalf(
			"actor ID = %q, want %q",
			actorID,
			user.ID,
		)
	}

	if correlationID !=
		vaultCreateTestCorrelationID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			correlationID,
			vaultCreateTestCorrelationID,
		)
	}

	if sanitizedPayload != "{}" {
		t.Fatalf(
			"sanitized payload = %q, want empty object",
			sanitizedPayload,
		)
	}

	if strings.Contains(
		sanitizedPayload,
		createdVault.Name,
	) {
		t.Fatal(
			"audit payload exposed the vault name",
		)
	}

	if status != "pending" {
		t.Fatalf(
			"audit status = %q, want pending",
			status,
		)
	}

	if attempts != 0 {
		t.Fatalf(
			"audit attempts = %d, want 0",
			attempts,
		)
	}
}

func TestVaultStoreCreateRollsBackWhenAuditInsertFails(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	user := createVaultTestUser(
		t,
		userStore,
		"vault-rollback@example.com",
	)

	_, err := vaultStore.Create(
		context.Background(),
		vaultdomain.CreateStoreInput{
			Vault: vaultdomain.Vault{
				OwnerID: user.ID,
				Name:    "Rollback Vault",
			},
			CorrelationID: "",
		},
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			ErrDatabase,
		)
	}

	if strings.Contains(
		err.Error(),
		"audit_outbox_correlation_id_not_blank",
	) {
		t.Fatal(
			"create error exposed a database constraint",
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	var (
		vaultCount int
		eventCount int
	)

	err = testDatabasePool.QueryRow(
		queryContext,
		`
			SELECT
				(
					SELECT count(*)
					FROM vaults
					WHERE owner_id = $1::uuid
				),
				(
					SELECT count(*)
					FROM audit_outbox
					WHERE actor_id = $1::uuid
				)
		`,
		user.ID,
	).Scan(
		&vaultCount,
		&eventCount,
	)
	if err != nil {
		t.Fatal(
			"count rolled-back vault records",
		)
	}

	if vaultCount != 0 {
		t.Fatalf(
			"vault count after rollback = %d, want 0",
			vaultCount,
		)
	}

	if eventCount != 0 {
		t.Fatalf(
			"audit event count after rollback = %d, want 0",
			eventCount,
		)
	}
}

func TestVaultStoreCreateHonorsCanceledContext(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	user := createVaultTestUser(
		t,
		userStore,
		"vault-canceled@example.com",
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := vaultStore.Create(
		ctx,
		vaultdomain.CreateStoreInput{
			Vault: vaultdomain.Vault{
				OwnerID: user.ID,
				Name:    "Canceled Vault",
			},
			CorrelationID: vaultCreateTestCorrelationID,
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}

func TestVaultStoreCreateMapsDatabaseFailureSafely(
	t *testing.T,
) {
	vaultStore, _ :=
		newIntegrationTestVaultStores(t)

	_, err := vaultStore.Create(
		context.Background(),
		vaultdomain.CreateStoreInput{
			Vault: vaultdomain.Vault{
				OwnerID: "not-a-valid-uuid",
				Name:    "Development",
			},
			CorrelationID: vaultCreateTestCorrelationID,
		},
	)

	if !errors.Is(err, ErrDatabase) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			ErrDatabase,
		)
	}

	if strings.Contains(
		err.Error(),
		"invalid input syntax",
	) {
		t.Fatal(
			"create error exposed raw PostgreSQL details",
		)
	}
}
