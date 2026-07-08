package vault

import "bytes"

const (
	ItemEncryptedPayloadVersion    = 1
	ItemEncryptedPayloadAlgorithm  = "AES-256-GCM"
	ItemEncryptedPayloadNonceBytes = 12
	ItemEncryptedPayloadTagBytes   = 16

	MaxEncryptedItemBlobBytes       = MaxSyntheticItemPayloadBytes + ItemEncryptedPayloadNonceBytes + ItemEncryptedPayloadTagBytes
	MaxEncryptedItemBlobBase64Bytes = ((MaxEncryptedItemBlobBytes + 2) / 3) * 4
)

type EncryptedItemEnvelope struct {
	Payload []byte
	Nonce   []byte
}

func NewEncryptedItemEnvelope(blob []byte) (EncryptedItemEnvelope, error) {
	if len(blob) == 0 {
		return EncryptedItemEnvelope{}, ErrItemEncryptedPayloadEmpty
	}

	if len(blob) > MaxEncryptedItemBlobBytes {
		return EncryptedItemEnvelope{}, ErrItemEncryptedPayloadTooLarge
	}

	if len(blob) < ItemEncryptedPayloadNonceBytes+ItemEncryptedPayloadTagBytes {
		return EncryptedItemEnvelope{}, ErrItemEncryptedPayloadInvalid
	}

	return EncryptedItemEnvelope{
		Nonce:   bytes.Clone(blob[:ItemEncryptedPayloadNonceBytes]),
		Payload: bytes.Clone(blob[ItemEncryptedPayloadNonceBytes:]),
	}, nil
}

func (envelope EncryptedItemEnvelope) Blob() ([]byte, error) {
	if len(envelope.Nonce) != ItemEncryptedPayloadNonceBytes {
		return nil, ErrItemEncryptedPayloadInvalid
	}

	if len(envelope.Payload) < ItemEncryptedPayloadTagBytes {
		return nil, ErrItemEncryptedPayloadInvalid
	}

	totalLength := len(envelope.Nonce) + len(envelope.Payload)
	if totalLength > MaxEncryptedItemBlobBytes {
		return nil, ErrItemEncryptedPayloadTooLarge
	}

	blob := make([]byte, 0, totalLength)
	blob = append(blob, envelope.Nonce...)
	blob = append(blob, envelope.Payload...)

	return blob, nil
}
