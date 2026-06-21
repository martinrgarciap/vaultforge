package store

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestAuditOutboxPayloadsNeverExposeSecretMaterial(t *testing.T) {
	vaultStore, userStore := newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"audit-payload-owner@example.com",
	)

	const (
		originalVaultName = "DO-NOT-LEAK-ORIGINAL-VAULT-NAME"
		renamedVaultName  = "DO-NOT-LEAK-RENAMED-VAULT-NAME"
		idempotencyKey    = "do-not-leak-idempotency-key"
		createMarker      = "do-not-leak-created-item-value"
		updateMarker      = "do-not-leak-updated-item-value"
	)

	createdVault, err := vaultStore.Create(
		context.Background(),
		vaultdomain.CreateStoreInput{
			Vault: vaultdomain.Vault{
				OwnerID: owner.ID,
				Name:    originalVaultName,
			},
			CorrelationID: "audit-payload-vault-create",
		},
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = vaultStore.RenameOwned(
		context.Background(),
		vaultdomain.RenameStoreInput{
			OwnerID:       owner.ID,
			VaultID:       createdVault.ID,
			Name:          renamedVaultName,
			CorrelationID: "audit-payload-vault-rename",
		},
	)
	if err != nil {
		t.Fatalf("RenameOwned() error = %v", err)
	}

	createEnvelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{
			"label": "Secret Create Label",
			"value": "` + createMarker + `"
		}`),
	)
	if err != nil {
		t.Fatalf("create initial synthetic envelope: %v", err)
	}

	idempotency := mustItemCreateIdempotency(
		t,
		idempotencyKey,
		vaultdomain.ItemTypeSecureNote,
		createEnvelope,
	)

	createdItem, err := vaultStore.CreateItem(
		context.Background(),
		vaultdomain.CreateItemStoreInput{
			OwnerID:       owner.ID,
			VaultID:       createdVault.ID,
			Type:          vaultdomain.ItemTypeSecureNote,
			Envelope:      createEnvelope,
			Idempotency:   idempotency,
			CorrelationID: "audit-payload-item-create",
		},
	)
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}

	updateEnvelope, err := vaultdomain.NewSyntheticItemEnvelope(
		json.RawMessage(`{
			"label": "Secret Update Label",
			"value": "` + updateMarker + `"
		}`),
	)
	if err != nil {
		t.Fatalf("create updated synthetic envelope: %v", err)
	}

	updatedItem, err := vaultStore.UpdateItem(
		context.Background(),
		vaultdomain.UpdateItemStoreInput{
			OwnerID:         owner.ID,
			VaultID:         createdVault.ID,
			ItemID:          createdItem.ID,
			Type:            vaultdomain.ItemTypeAPIKey,
			Envelope:        updateEnvelope,
			ExpectedVersion: createdItem.Version,
			CorrelationID:   "audit-payload-item-update",
		},
	)
	if err != nil {
		t.Fatalf("UpdateItem() error = %v", err)
	}

	firstDeletedItem, err := vaultStore.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:         owner.ID,
			VaultID:         createdVault.ID,
			ItemID:          createdItem.ID,
			ExpectedVersion: updatedItem.Version,
			CorrelationID:   "audit-payload-item-first-delete",
		},
	)
	if err != nil {
		t.Fatalf("first SoftDeleteItem() error = %v", err)
	}

	restoredItem, err := vaultStore.RestoreItem(
		context.Background(),
		vaultdomain.RestoreItemStoreInput{
			OwnerID:         owner.ID,
			VaultID:         createdVault.ID,
			ItemID:          createdItem.ID,
			ExpectedVersion: firstDeletedItem.Version,
			CorrelationID:   "audit-payload-item-restore",
		},
	)
	if err != nil {
		t.Fatalf("RestoreItem() error = %v", err)
	}

	secondDeletedItem, err := vaultStore.SoftDeleteItem(
		context.Background(),
		vaultdomain.SoftDeleteItemStoreInput{
			OwnerID:         owner.ID,
			VaultID:         createdVault.ID,
			ItemID:          createdItem.ID,
			ExpectedVersion: restoredItem.Version,
			CorrelationID:   "audit-payload-item-second-delete",
		},
	)
	if err != nil {
		t.Fatalf("second SoftDeleteItem() error = %v", err)
	}

	err = vaultStore.PermanentDeleteItem(
		context.Background(),
		vaultdomain.PermanentDeleteItemStoreInput{
			OwnerID:         owner.ID,
			VaultID:         createdVault.ID,
			ItemID:          createdItem.ID,
			ExpectedVersion: secondDeletedItem.Version,
			CorrelationID:   "audit-payload-item-permanent-delete",
		},
	)
	if err != nil {
		t.Fatalf("PermanentDeleteItem() error = %v", err)
	}

	err = vaultStore.DeleteOwned(
		context.Background(),
		vaultdomain.DeleteStoreInput{
			OwnerID:       owner.ID,
			VaultID:       createdVault.ID,
			CorrelationID: "audit-payload-vault-delete",
		},
	)
	if err != nil {
		t.Fatalf("DeleteOwned() error = %v", err)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	rows, err := testDatabasePool.Query(
		queryContext,
		`
			SELECT
				event_type,
				aggregate_type,
				sanitized_payload::text
			FROM audit_outbox
			WHERE actor_id = $1::uuid
			ORDER BY
				created_at,
				id
		`,
		owner.ID,
	)
	if err != nil {
		t.Fatalf("query audit outbox payloads: %v", err)
	}
	defer rows.Close()

	forbiddenValues := []string{
		originalVaultName,
		renamedVaultName,
		createMarker,
		updateMarker,
		string(createEnvelope.Payload),
		string(updateEnvelope.Payload),
		string(createEnvelope.Nonce),
		string(updateEnvelope.Nonce),
		idempotencyKey,
		hex.EncodeToString(idempotency.KeyHash[:]),
		hex.EncodeToString(idempotency.RequestHash[:]),
	}

	expectedEventCounts := map[string]int{
		"vault.created":                  1,
		"vault.renamed":                  1,
		"vault.deleted":                  1,
		"vault_item.created":             1,
		"vault_item.updated":             1,
		"vault_item.deleted":             2,
		"vault_item.restored":            1,
		"vault_item.permanently_deleted": 1,
	}

	actualEventCounts := make(map[string]int)
	deletedVersionCounts := make(map[int]int)
	totalEventCount := 0

	for rows.Next() {
		var (
			eventType        string
			aggregateType    string
			sanitizedPayload string
		)

		if err := rows.Scan(
			&eventType,
			&aggregateType,
			&sanitizedPayload,
		); err != nil {
			t.Fatalf("scan audit outbox payload: %v", err)
		}

		totalEventCount++
		actualEventCounts[eventType]++

		for _, forbiddenValue := range forbiddenValues {
			if forbiddenValue != "" &&
				strings.Contains(sanitizedPayload, forbiddenValue) {
				t.Fatalf(
					"event %q exposed forbidden value %q",
					eventType,
					forbiddenValue,
				)
			}
		}

		switch aggregateType {
		case "vault":
			assertStoredVaultAuditPayload(
				t,
				eventType,
				sanitizedPayload,
			)

		case "vault_item":
			var metadata vaultItemAuditPayload

			if err := json.Unmarshal(
				[]byte(sanitizedPayload),
				&metadata,
			); err != nil {
				t.Fatalf(
					"decode %q audit payload: %v",
					eventType,
					err,
				)
			}

			switch eventType {
			case "vault_item.created":
				assertVaultItemAuditPayload(
					t,
					sanitizedPayload,
					createdVault.ID,
					vaultdomain.ItemTypeSecureNote,
					1,
					forbiddenValues...,
				)

			case "vault_item.updated":
				assertVaultItemAuditPayload(
					t,
					sanitizedPayload,
					createdVault.ID,
					vaultdomain.ItemTypeAPIKey,
					2,
					forbiddenValues...,
				)

			case "vault_item.deleted":
				if metadata.Version != 2 &&
					metadata.Version != 3 {
					t.Fatalf(
						"deleted audit version = %d, want 2 or 3",
						metadata.Version,
					)
				}

				deletedVersionCounts[metadata.Version]++

				assertVaultItemAuditPayload(
					t,
					sanitizedPayload,
					createdVault.ID,
					vaultdomain.ItemTypeAPIKey,
					metadata.Version,
					forbiddenValues...,
				)

			case "vault_item.restored":
				assertVaultItemAuditPayload(
					t,
					sanitizedPayload,
					createdVault.ID,
					vaultdomain.ItemTypeAPIKey,
					3,
					forbiddenValues...,
				)

			case "vault_item.permanently_deleted":
				assertVaultItemAuditPayload(
					t,
					sanitizedPayload,
					createdVault.ID,
					vaultdomain.ItemTypeAPIKey,
					3,
					forbiddenValues...,
				)

			default:
				t.Fatalf(
					"unexpected vault-item event type %q",
					eventType,
				)
			}

		default:
			t.Fatalf(
				"unexpected aggregate type %q for event %q",
				aggregateType,
				eventType,
			)
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate audit outbox payloads: %v", err)
	}

	if totalEventCount != 9 {
		t.Fatalf("audit event count = %d, want 9", totalEventCount)
	}

	if len(actualEventCounts) != len(expectedEventCounts) {
		t.Fatalf(
			"distinct event types = %d, want %d",
			len(actualEventCounts),
			len(expectedEventCounts),
		)
	}

	for eventType, wantCount := range expectedEventCounts {
		if actualEventCounts[eventType] != wantCount {
			t.Fatalf(
				"event %q count = %d, want %d",
				eventType,
				actualEventCounts[eventType],
				wantCount,
			)
		}
	}

	if deletedVersionCounts[2] != 1 {
		t.Fatalf(
			"version 2 deleted event count = %d, want 1",
			deletedVersionCounts[2],
		)
	}

	if deletedVersionCounts[3] != 1 {
		t.Fatalf(
			"version 3 deleted event count = %d, want 1",
			deletedVersionCounts[3],
		)
	}
}

func assertStoredVaultAuditPayload(
	t *testing.T,
	eventType string,
	payload string,
) {
	t.Helper()

	var decodedPayload map[string]json.RawMessage

	if err := json.Unmarshal(
		[]byte(payload),
		&decodedPayload,
	); err != nil {
		t.Fatalf(
			"decode %q vault audit payload: %v",
			eventType,
			err,
		)
	}

	if len(decodedPayload) != 1 {
		t.Fatalf(
			"%q vault audit fields = %d, want 1",
			eventType,
			len(decodedPayload),
		)
	}

	var metadata vaultAuditPayload

	if err := json.Unmarshal(
		[]byte(payload),
		&metadata,
	); err != nil {
		t.Fatalf(
			"decode %q vault audit metadata: %v",
			eventType,
			err,
		)
	}

	if metadata.SchemaVersion != auditPayloadSchemaVersion {
		t.Fatalf(
			"%q schema version = %d, want %d",
			eventType,
			metadata.SchemaVersion,
			auditPayloadSchemaVersion,
		)
	}

	if _, exists := decodedPayload["schemaVersion"]; !exists {
		t.Fatalf(
			"%q vault audit payload omitted schemaVersion",
			eventType,
		)
	}
}
