package vault

import (
	"bytes"
	"errors"
	"testing"
)

func validEncryptedItemBlob() []byte {
	blob := make(
		[]byte,
		ItemEncryptedPayloadNonceBytes+ItemEncryptedPayloadTagBytes+4,
	)

	for index := range blob {
		blob[index] = byte(index + 1)
	}

	return blob
}

func TestNewEncryptedItemEnvelopeSplitsBlob(t *testing.T) {
	t.Parallel()

	blob := validEncryptedItemBlob()

	envelope, err := NewEncryptedItemEnvelope(blob)
	if err != nil {
		t.Fatalf("NewEncryptedItemEnvelope() error = %v", err)
	}

	if !bytes.Equal(envelope.Nonce, blob[:ItemEncryptedPayloadNonceBytes]) {
		t.Fatal("envelope nonce did not match the first blob bytes")
	}

	if !bytes.Equal(envelope.Payload, blob[ItemEncryptedPayloadNonceBytes:]) {
		t.Fatal("envelope payload did not match the remaining blob bytes")
	}
}

func TestNewEncryptedItemEnvelopeReturnsIndependentCopies(t *testing.T) {
	t.Parallel()

	blob := validEncryptedItemBlob()

	envelope, err := NewEncryptedItemEnvelope(blob)
	if err != nil {
		t.Fatalf("NewEncryptedItemEnvelope() error = %v", err)
	}

	blob[0] = 255
	blob[ItemEncryptedPayloadNonceBytes] = 254

	if envelope.Nonce[0] == 255 {
		t.Fatal("envelope nonce shared mutable blob storage")
	}

	if envelope.Payload[0] == 254 {
		t.Fatal("envelope payload shared mutable blob storage")
	}
}

func TestEncryptedItemEnvelopeBlobJoinsNonceAndPayload(t *testing.T) {
	t.Parallel()

	originalBlob := validEncryptedItemBlob()

	envelope, err := NewEncryptedItemEnvelope(originalBlob)
	if err != nil {
		t.Fatalf("NewEncryptedItemEnvelope() error = %v", err)
	}

	joinedBlob, err := envelope.Blob()
	if err != nil {
		t.Fatalf("Blob() error = %v", err)
	}

	if !bytes.Equal(joinedBlob, originalBlob) {
		t.Fatal("joined blob did not match original blob")
	}
}

func TestEncryptedItemEnvelopeBlobReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	envelope, err := NewEncryptedItemEnvelope(validEncryptedItemBlob())
	if err != nil {
		t.Fatalf("NewEncryptedItemEnvelope() error = %v", err)
	}

	joinedBlob, err := envelope.Blob()
	if err != nil {
		t.Fatalf("Blob() error = %v", err)
	}

	joinedBlob[0] = 255

	if envelope.Nonce[0] == 255 {
		t.Fatal("Blob() exposed mutable envelope nonce storage")
	}
}

func TestNewEncryptedItemEnvelopeRejectsInvalidBlobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		blob    []byte
		wantErr error
	}{
		{
			name:    "empty",
			blob:    nil,
			wantErr: ErrItemEncryptedPayloadEmpty,
		},
		{
			name: "too short",
			blob: make(
				[]byte,
				ItemEncryptedPayloadNonceBytes+ItemEncryptedPayloadTagBytes-1,
			),
			wantErr: ErrItemEncryptedPayloadInvalid,
		},
		{
			name: "too large",
			blob: make(
				[]byte,
				MaxEncryptedItemBlobBytes+1,
			),
			wantErr: ErrItemEncryptedPayloadTooLarge,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewEncryptedItemEnvelope(test.blob)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"NewEncryptedItemEnvelope() error = %v, want %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestEncryptedItemEnvelopeBlobRejectsInvalidEnvelope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		envelope EncryptedItemEnvelope
		wantErr  error
	}{
		{
			name: "invalid nonce length",
			envelope: EncryptedItemEnvelope{
				Nonce:   make([]byte, ItemEncryptedPayloadNonceBytes-1),
				Payload: make([]byte, ItemEncryptedPayloadTagBytes),
			},
			wantErr: ErrItemEncryptedPayloadInvalid,
		},
		{
			name: "payload shorter than tag",
			envelope: EncryptedItemEnvelope{
				Nonce:   make([]byte, ItemEncryptedPayloadNonceBytes),
				Payload: make([]byte, ItemEncryptedPayloadTagBytes-1),
			},
			wantErr: ErrItemEncryptedPayloadInvalid,
		},
		{
			name: "too large",
			envelope: EncryptedItemEnvelope{
				Nonce: make([]byte, ItemEncryptedPayloadNonceBytes),
				Payload: make(
					[]byte,
					MaxEncryptedItemBlobBytes-ItemEncryptedPayloadNonceBytes+1,
				),
			},
			wantErr: ErrItemEncryptedPayloadTooLarge,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := test.envelope.Blob()

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Blob() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}
