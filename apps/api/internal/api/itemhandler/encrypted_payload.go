package itemhandler

import (
	"encoding/base64"
	"strings"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

type itemEncryptedPayloadResource struct {
	Version   int    `json:"version"`
	Algorithm string `json:"algorithm"`
	Blob      string `json:"blob"`
}

func (resource itemEncryptedPayloadResource) empty() bool {
	return resource.Version == 0 &&
		resource.Algorithm == "" &&
		resource.Blob == ""
}

func newItemEncryptedPayloadResource(
	envelope vault.EncryptedItemEnvelope,
) (itemEncryptedPayloadResource, error) {
	blob, err := envelope.Blob()
	if err != nil {
		return itemEncryptedPayloadResource{}, err
	}

	return itemEncryptedPayloadResource{
		Version:   vault.ItemEncryptedPayloadVersion,
		Algorithm: vault.ItemEncryptedPayloadAlgorithm,
		Blob:      base64.StdEncoding.EncodeToString(blob),
	}, nil
}

func requiredEncryptedItemEnvelopeFromResource(
	resource itemEncryptedPayloadResource,
) (vault.EncryptedItemEnvelope, error) {
	if resource.empty() {
		return vault.EncryptedItemEnvelope{}, vault.ErrItemEncryptedPayloadEmpty
	}

	return encryptedItemEnvelopeFromResource(resource)
}

func encryptedItemEnvelopeFromResource(
	resource itemEncryptedPayloadResource,
) (vault.EncryptedItemEnvelope, error) {
	if resource.Version != vault.ItemEncryptedPayloadVersion {
		return vault.EncryptedItemEnvelope{}, vault.ErrItemEncryptedPayloadInvalid
	}

	if resource.Algorithm != vault.ItemEncryptedPayloadAlgorithm {
		return vault.EncryptedItemEnvelope{}, vault.ErrItemEncryptedPayloadInvalid
	}

	if resource.Blob == "" {
		return vault.EncryptedItemEnvelope{}, vault.ErrItemEncryptedPayloadEmpty
	}

	if strings.TrimSpace(resource.Blob) != resource.Blob {
		return vault.EncryptedItemEnvelope{}, vault.ErrItemEncryptedPayloadInvalid
	}

	blob, err := base64.StdEncoding.Strict().DecodeString(resource.Blob)
	if err != nil {
		return vault.EncryptedItemEnvelope{}, vault.ErrItemEncryptedPayloadInvalid
	}

	return vault.NewEncryptedItemEnvelope(blob)
}
