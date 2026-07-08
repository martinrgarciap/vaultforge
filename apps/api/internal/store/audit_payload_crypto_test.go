package store

import (
	"encoding/json"
	"strings"
	"testing"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestVaultItemAuditPayloadDoesNotIncludeEncryptedOrPlaintextMaterial(t *testing.T) {
	t.Parallel()

	const forbiddenMarker = "DO-NOT-LEAK-ITEM-MATERIAL"

	payload, err := newVaultItemAuditPayload(
		"00000000-0000-0000-0000-000000002002",
		vaultdomain.ItemTypeSecureNote,
		3,
	)
	if err != nil {
		t.Fatalf("newVaultItemAuditPayload() error = %v", err)
	}

	forbiddenValues := []string{
		"payload",
		"encryptedPayload",
		"encrypted_payload",
		"nonce",
		"wrappedKey",
		"wrapped_key",
		"salt",
		forbiddenMarker,
	}

	for _, forbiddenValue := range forbiddenValues {
		if strings.Contains(payload, forbiddenValue) {
			t.Fatalf(
				"audit payload exposed forbidden value %q in %s",
				forbiddenValue,
				payload,
			)
		}
	}

	var decoded map[string]any

	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("decode audit payload: %v", err)
	}

	if len(decoded) != 4 {
		t.Fatalf("audit payload field count = %d, want 4: %v", len(decoded), decoded)
	}

	if decoded["schemaVersion"] == nil ||
		decoded["vaultId"] == nil ||
		decoded["itemType"] == nil ||
		decoded["version"] == nil {
		t.Fatalf("audit payload missing expected metadata fields: %v", decoded)
	}
}
