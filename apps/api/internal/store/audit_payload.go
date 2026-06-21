package store

import (
	"encoding/json"
	"errors"
	"strings"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const auditPayloadSchemaVersion = 1

type vaultAuditPayload struct {
	SchemaVersion int `json:"schemaVersion"`
}

type vaultItemAuditPayload struct {
	SchemaVersion int                  `json:"schemaVersion"`
	VaultID       string               `json:"vaultId"`
	ItemType      vaultdomain.ItemType `json:"itemType"`
	Version       int                  `json:"version"`
}

func newVaultAuditPayload() (string, error) {
	encodedPayload, err := json.Marshal(
		vaultAuditPayload{
			SchemaVersion: auditPayloadSchemaVersion,
		},
	)
	if err != nil {
		return "", err
	}

	return string(encodedPayload), nil
}

func newVaultItemAuditPayload(
	vaultID string,
	itemType vaultdomain.ItemType,
	version int,
) (string, error) {
	if strings.TrimSpace(vaultID) == "" {
		return "", errors.New("vault item audit payload vault ID is blank")
	}

	if !itemType.Valid() {
		return "", errors.New("vault item audit payload item type is invalid")
	}

	if version < vaultdomain.InitialItemVersion {
		return "", errors.New("vault item audit payload version is invalid")
	}

	encodedPayload, err := json.Marshal(
		vaultItemAuditPayload{
			SchemaVersion: auditPayloadSchemaVersion,
			VaultID:       vaultID,
			ItemType:      itemType,
			Version:       version,
		},
	)
	if err != nil {
		return "", err
	}

	return string(encodedPayload), nil
}
