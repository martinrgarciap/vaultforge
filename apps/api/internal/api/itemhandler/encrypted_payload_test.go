package itemhandler

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func validItemEncryptedBlob() []byte {
	blob := make(
		[]byte,
		vault.ItemEncryptedPayloadNonceBytes+vault.ItemEncryptedPayloadTagBytes+4,
	)

	for index := range blob {
		blob[index] = byte(index + 1)
	}

	return blob
}

func validItemEncryptedPayloadResource() itemEncryptedPayloadResource {
	return itemEncryptedPayloadResource{
		Version:   vault.ItemEncryptedPayloadVersion,
		Algorithm: vault.ItemEncryptedPayloadAlgorithm,
		Blob:      base64.StdEncoding.EncodeToString(validItemEncryptedBlob()),
	}
}

func TestEncryptedItemEnvelopeFromResourceParsesValidResource(t *testing.T) {
	t.Parallel()

	resource := validItemEncryptedPayloadResource()

	envelope, err := encryptedItemEnvelopeFromResource(resource)
	if err != nil {
		t.Fatalf("encryptedItemEnvelopeFromResource() error = %v", err)
	}

	blob, err := envelope.Blob()
	if err != nil {
		t.Fatalf("Blob() error = %v", err)
	}

	wantBlob, err := base64.StdEncoding.DecodeString(resource.Blob)
	if err != nil {
		t.Fatalf("decode test blob: %v", err)
	}

	if !bytes.Equal(blob, wantBlob) {
		t.Fatal("parsed envelope blob did not match resource blob")
	}
}

func TestNewItemEncryptedPayloadResourceSerializesEnvelope(t *testing.T) {
	t.Parallel()

	envelope, err := vault.NewEncryptedItemEnvelope(validItemEncryptedBlob())
	if err != nil {
		t.Fatalf("NewEncryptedItemEnvelope() error = %v", err)
	}

	resource, err := newItemEncryptedPayloadResource(envelope)
	if err != nil {
		t.Fatalf("newItemEncryptedPayloadResource() error = %v", err)
	}

	if resource.Version != vault.ItemEncryptedPayloadVersion {
		t.Fatalf(
			"version = %d, want %d",
			resource.Version,
			vault.ItemEncryptedPayloadVersion,
		)
	}

	if resource.Algorithm != vault.ItemEncryptedPayloadAlgorithm {
		t.Fatalf(
			"algorithm = %q, want %q",
			resource.Algorithm,
			vault.ItemEncryptedPayloadAlgorithm,
		)
	}

	roundTripEnvelope, err := encryptedItemEnvelopeFromResource(resource)
	if err != nil {
		t.Fatalf("round trip parse error = %v", err)
	}

	roundTripBlob, err := roundTripEnvelope.Blob()
	if err != nil {
		t.Fatalf("round trip Blob() error = %v", err)
	}

	originalBlob, err := envelope.Blob()
	if err != nil {
		t.Fatalf("original Blob() error = %v", err)
	}

	if !bytes.Equal(roundTripBlob, originalBlob) {
		t.Fatal("serialized resource did not round trip")
	}
}

func TestEncryptedItemEnvelopeFromResourceRejectsInvalidResources(t *testing.T) {
	t.Parallel()

	validResource := validItemEncryptedPayloadResource()

	tests := []struct {
		name     string
		resource itemEncryptedPayloadResource
		wantErr  error
	}{
		{
			name: "unsupported version",
			resource: itemEncryptedPayloadResource{
				Version:   vault.ItemEncryptedPayloadVersion + 1,
				Algorithm: validResource.Algorithm,
				Blob:      validResource.Blob,
			},
			wantErr: vault.ErrItemEncryptedPayloadInvalid,
		},
		{
			name: "unsupported algorithm",
			resource: itemEncryptedPayloadResource{
				Version:   validResource.Version,
				Algorithm: "AES-128-GCM",
				Blob:      validResource.Blob,
			},
			wantErr: vault.ErrItemEncryptedPayloadInvalid,
		},
		{
			name: "empty blob",
			resource: itemEncryptedPayloadResource{
				Version:   validResource.Version,
				Algorithm: validResource.Algorithm,
				Blob:      "",
			},
			wantErr: vault.ErrItemEncryptedPayloadEmpty,
		},
		{
			name: "malformed base64",
			resource: itemEncryptedPayloadResource{
				Version:   validResource.Version,
				Algorithm: validResource.Algorithm,
				Blob:      "not base64",
			},
			wantErr: vault.ErrItemEncryptedPayloadInvalid,
		},
		{
			name: "base64 with surrounding whitespace",
			resource: itemEncryptedPayloadResource{
				Version:   validResource.Version,
				Algorithm: validResource.Algorithm,
				Blob:      " " + validResource.Blob,
			},
			wantErr: vault.ErrItemEncryptedPayloadInvalid,
		},
		{
			name: "too short decoded blob",
			resource: itemEncryptedPayloadResource{
				Version:   validResource.Version,
				Algorithm: validResource.Algorithm,
				Blob: base64.StdEncoding.EncodeToString(
					make(
						[]byte,
						vault.ItemEncryptedPayloadNonceBytes+
							vault.ItemEncryptedPayloadTagBytes-
							1,
					),
				),
			},
			wantErr: vault.ErrItemEncryptedPayloadInvalid,
		},
		{
			name: "too large decoded blob",
			resource: itemEncryptedPayloadResource{
				Version:   validResource.Version,
				Algorithm: validResource.Algorithm,
				Blob: base64.StdEncoding.EncodeToString(
					make([]byte, vault.MaxEncryptedItemBlobBytes+1),
				),
			},
			wantErr: vault.ErrItemEncryptedPayloadTooLarge,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := encryptedItemEnvelopeFromResource(test.resource)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"encryptedItemEnvelopeFromResource() error = %v, want %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestNewItemEncryptedPayloadResourceRejectsInvalidEnvelope(t *testing.T) {
	t.Parallel()

	_, err := newItemEncryptedPayloadResource(
		vault.EncryptedItemEnvelope{
			Nonce:   make([]byte, vault.ItemEncryptedPayloadNonceBytes-1),
			Payload: make([]byte, vault.ItemEncryptedPayloadTagBytes),
		},
	)

	if !errors.Is(err, vault.ErrItemEncryptedPayloadInvalid) {
		t.Fatalf(
			"newItemEncryptedPayloadResource() error = %v, want %v",
			err,
			vault.ErrItemEncryptedPayloadInvalid,
		)
	}
}
