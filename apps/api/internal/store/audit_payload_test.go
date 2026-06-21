package store

import (
	"encoding/json"
	"strings"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestNewVaultAuditPayloadCreatesVersionedMetadata(t *testing.T) {
	t.Parallel()

	payload, err := newVaultAuditPayload()
	if err != nil {
		t.Fatalf("newVaultAuditPayload() error = %v", err)
	}

	const wantPayload = `{"schemaVersion":1}`

	if payload != wantPayload {
		t.Fatalf("vault audit payload = %s, want %s", payload, wantPayload)
	}
}

func TestNewVaultItemAuditPayloadCreatesOnlySanitizedMetadata(t *testing.T) {
	t.Parallel()

	const vaultID = "00000000-0000-0000-0000-000000001001"

	payload, err := newVaultItemAuditPayload(
		vaultID,
		vaultdomain.ItemTypeSecureNote,
		3,
	)
	if err != nil {
		t.Fatalf("newVaultItemAuditPayload() error = %v", err)
	}

	var decodedPayload map[string]any

	if err := json.Unmarshal([]byte(payload), &decodedPayload); err != nil {
		t.Fatalf("decode vault item audit payload: %v", err)
	}

	if len(decodedPayload) != 4 {
		t.Fatalf(
			"vault item audit payload fields = %d, want 4",
			len(decodedPayload),
		)
	}

	if decodedPayload["schemaVersion"] != float64(auditPayloadSchemaVersion) {
		t.Fatalf(
			"schema version = %v, want %d",
			decodedPayload["schemaVersion"],
			auditPayloadSchemaVersion,
		)
	}

	if decodedPayload["vaultId"] != vaultID {
		t.Fatalf(
			"vault ID = %v, want %q",
			decodedPayload["vaultId"],
			vaultID,
		)
	}

	if decodedPayload["itemType"] != string(vaultdomain.ItemTypeSecureNote) {
		t.Fatalf(
			"item type = %v, want %q",
			decodedPayload["itemType"],
			vaultdomain.ItemTypeSecureNote,
		)
	}

	if decodedPayload["version"] != float64(3) {
		t.Fatalf("version = %v, want 3", decodedPayload["version"])
	}

	for _, forbiddenField := range []string{
		"payload",
		"encryptedPayload",
		"nonce",
		"name",
		"salt",
		"wrappedKey",
		"idempotencyKey",
		"idempotencyKeyHash",
		"requestHash",
	} {
		if _, exists := decodedPayload[forbiddenField]; exists {
			t.Fatalf(
				"vault item audit payload contained forbidden field %q",
				forbiddenField,
			)
		}
	}
}

func TestNewVaultItemAuditPayloadRejectsInvalidMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		vaultID  string
		itemType vaultdomain.ItemType
		version  int
	}{
		{
			name:     "blank vault ID",
			vaultID:  " ",
			itemType: vaultdomain.ItemTypeSecureNote,
			version:  1,
		},
		{
			name:     "invalid item type",
			vaultID:  "00000000-0000-0000-0000-000000001001",
			itemType: vaultdomain.ItemType("invalid"),
			version:  1,
		},
		{
			name:     "invalid version",
			vaultID:  "00000000-0000-0000-0000-000000001001",
			itemType: vaultdomain.ItemTypeSecureNote,
			version:  0,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := newVaultItemAuditPayload(
				test.vaultID,
				test.itemType,
				test.version,
			)

			if err == nil {
				t.Fatal("newVaultItemAuditPayload() error = nil, want an error")
			}
		})
	}
}

func assertVaultItemAuditPayload(
	t *testing.T,
	payload string,
	wantVaultID string,
	wantItemType vaultdomain.ItemType,
	wantVersion int,
	forbiddenValues ...string,
) {
	t.Helper()

	var decodedPayload map[string]json.RawMessage

	if err := json.Unmarshal([]byte(payload), &decodedPayload); err != nil {
		t.Fatalf("decode vault item audit payload: %v", err)
	}

	if len(decodedPayload) != 4 {
		t.Fatalf(
			"vault item audit payload fields = %d, want 4",
			len(decodedPayload),
		)
	}

	var metadata vaultItemAuditPayload

	if err := json.Unmarshal([]byte(payload), &metadata); err != nil {
		t.Fatalf("decode vault item audit metadata: %v", err)
	}

	if metadata.SchemaVersion != auditPayloadSchemaVersion {
		t.Fatalf(
			"audit schema version = %d, want %d",
			metadata.SchemaVersion,
			auditPayloadSchemaVersion,
		)
	}

	if metadata.VaultID != wantVaultID {
		t.Fatalf(
			"audit vault ID = %q, want %q",
			metadata.VaultID,
			wantVaultID,
		)
	}

	if metadata.ItemType != wantItemType {
		t.Fatalf(
			"audit item type = %q, want %q",
			metadata.ItemType,
			wantItemType,
		)
	}

	if metadata.Version != wantVersion {
		t.Fatalf(
			"audit item version = %d, want %d",
			metadata.Version,
			wantVersion,
		)
	}

	for _, forbiddenField := range []string{
		"payload",
		"encryptedPayload",
		"nonce",
		"name",
		"salt",
		"wrappedKey",
		"idempotencyKey",
		"idempotencyKeyHash",
		"requestHash",
	} {
		if _, exists := decodedPayload[forbiddenField]; exists {
			t.Fatalf(
				"vault item audit payload contained forbidden field %q",
				forbiddenField,
			)
		}
	}

	for _, forbiddenValue := range forbiddenValues {
		if forbiddenValue != "" && strings.Contains(payload, forbiddenValue) {
			t.Fatalf(
				"vault item audit payload exposed forbidden value %q",
				forbiddenValue,
			)
		}
	}
}
