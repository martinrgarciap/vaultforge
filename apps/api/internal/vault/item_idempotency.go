package vault

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
)

const (
	ItemCreateOperation        = "vault_item.create"
	MaxItemIdempotencyKeyBytes = 255
)

type ItemCreateIdempotency struct {
	KeyHash     [sha256.Size]byte
	RequestHash [sha256.Size]byte
}

func NewItemCreateIdempotency(
	key string,
	itemType ItemType,
	envelope ItemEnvelope,
) (ItemCreateIdempotency, error) {
	normalizedKey := strings.TrimSpace(key)

	if normalizedKey == "" || len(normalizedKey) > MaxItemIdempotencyKeyBytes {
		return ItemCreateIdempotency{}, ErrItemIdempotencyKeyInvalid
	}

	if !itemType.Valid() {
		return ItemCreateIdempotency{}, ErrItemTypeInvalid
	}

	requestBytes, err := itemCreateRequestHashBytes(itemType, envelope)
	if err != nil {
		return ItemCreateIdempotency{}, err
	}

	return ItemCreateIdempotency{
		KeyHash:     sha256.Sum256([]byte(normalizedKey)),
		RequestHash: sha256.Sum256(requestBytes),
	}, nil
}

func itemCreateRequestHashBytes(
	itemType ItemType,
	envelope ItemEnvelope,
) ([]byte, error) {
	if IsSyntheticItemNonce(envelope.Nonce) {
		normalizedPayload, err := NormalizeSyntheticItemPayload(envelope.Payload)
		if err != nil {
			return nil, err
		}

		return json.Marshal(
			struct {
				Type    ItemType        `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}{
				Type:    itemType,
				Payload: normalizedPayload,
			},
		)
	}

	encryptedEnvelope := EncryptedItemEnvelope(envelope)

	blob, err := encryptedEnvelope.Blob()
	if err != nil {
		return nil, err
	}

	return json.Marshal(
		struct {
			Type             ItemType `json:"type"`
			EncryptedPayload struct {
				Version   int    `json:"version"`
				Algorithm string `json:"algorithm"`
				Blob      []byte `json:"blob"`
			} `json:"encryptedPayload"`
		}{
			Type: itemType,
			EncryptedPayload: struct {
				Version   int    `json:"version"`
				Algorithm string `json:"algorithm"`
				Blob      []byte `json:"blob"`
			}{
				Version:   ItemEncryptedPayloadVersion,
				Algorithm: ItemEncryptedPayloadAlgorithm,
				Blob:      blob,
			},
		},
	)
}
