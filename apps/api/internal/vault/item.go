package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
	"unicode/utf8"
)

const MaxSyntheticItemPayloadBytes = 64 * 1024

var syntheticItemNonce = []byte(
	"vaultforge-synthetic-nonce-v1",
)

type Item struct {
	ID        string
	VaultID   string
	Type      ItemType
	Payload   json.RawMessage
	Nonce     []byte
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type ItemEnvelope struct {
	Payload []byte
	Nonce   []byte
}

type SyntheticItemEnvelope = ItemEnvelope

func (item Item) Deleted() bool {
	return item.DeletedAt != nil
}

func (item Item) Envelope() ItemEnvelope {
	if len(item.Nonce) == 0 {
		return ItemEnvelope{
			Payload: bytes.Clone(item.Payload),
			Nonce:   SyntheticItemNonce(),
		}
	}

	return ItemEnvelope{
		Payload: bytes.Clone(item.Payload),
		Nonce:   bytes.Clone(item.Nonce),
	}
}

func NewSyntheticItemEnvelope(payload json.RawMessage) (SyntheticItemEnvelope, error) {
	normalizedPayload, err := NormalizeSyntheticItemPayload(payload)
	if err != nil {
		return SyntheticItemEnvelope{}, err
	}

	return SyntheticItemEnvelope{
		Payload: normalizedPayload,
		Nonce:   SyntheticItemNonce(),
	}, nil
}

func NewItemEnvelopeFromEncrypted(envelope EncryptedItemEnvelope) (ItemEnvelope, error) {
	blob, err := envelope.Blob()
	if err != nil {
		return ItemEnvelope{}, err
	}

	normalizedEnvelope, err := NewEncryptedItemEnvelope(blob)
	if err != nil {
		return ItemEnvelope{}, err
	}

	return ItemEnvelope(normalizedEnvelope), nil
}

func NewItemEnvelopeFromStorage(payload []byte, nonce []byte) (ItemEnvelope, error) {
	if IsSyntheticItemNonce(nonce) {
		normalizedPayload, err := NormalizeSyntheticItemPayload(payload)
		if err != nil {
			return ItemEnvelope{}, err
		}

		return ItemEnvelope{
			Payload: normalizedPayload,
			Nonce:   SyntheticItemNonce(),
		}, nil
	}

	envelope := EncryptedItemEnvelope{
		Payload: bytes.Clone(payload),
		Nonce:   bytes.Clone(nonce),
	}

	blob, err := envelope.Blob()
	if err != nil {
		return ItemEnvelope{}, err
	}

	normalizedEnvelope, err := NewEncryptedItemEnvelope(blob)
	if err != nil {
		return ItemEnvelope{}, err
	}

	return ItemEnvelope(normalizedEnvelope), nil
}

func NormalizeSyntheticItemPayload(payload json.RawMessage) (json.RawMessage, error) {
	trimmedPayload := bytes.TrimSpace(payload)

	if len(trimmedPayload) == 0 {
		return nil, ErrItemPayloadEmpty
	}

	if len(trimmedPayload) > MaxSyntheticItemPayloadBytes {
		return nil, ErrItemPayloadTooLarge
	}

	if !utf8.Valid(trimmedPayload) {
		return nil, ErrItemPayloadInvalid
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmedPayload))
	decoder.UseNumber()

	var value any

	if err := decoder.Decode(&value); err != nil {
		return nil, ErrItemPayloadInvalid
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, ErrItemPayloadInvalid
	}

	object, ok := value.(map[string]any)
	if !ok || object == nil {
		return nil, ErrItemPayloadNotObject
	}

	normalizedPayload, err := json.Marshal(object)
	if err != nil {
		return nil, ErrItemPayloadInvalid
	}

	return json.RawMessage(normalizedPayload), nil
}

func SyntheticItemNonce() []byte {
	return bytes.Clone(syntheticItemNonce)
}

func IsSyntheticItemNonce(nonce []byte) bool {
	return bytes.Equal(nonce, syntheticItemNonce)
}
