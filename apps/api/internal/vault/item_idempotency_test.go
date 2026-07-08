package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNewItemCreateIdempotencyCreatesStableHashes(t *testing.T) {
	t.Parallel()

	firstEnvelope, err := NewSyntheticItemEnvelope(
		json.RawMessage(`{
			"token": "synthetic-token",
			"label": "Development"
		}`),
	)
	if err != nil {
		t.Fatalf("create first envelope: %v", err)
	}

	secondEnvelope, err := NewSyntheticItemEnvelope(
		json.RawMessage(`{"label":"Development","token":"synthetic-token"}`),
	)
	if err != nil {
		t.Fatalf("create second envelope: %v", err)
	}

	first, err := NewItemCreateIdempotency(
		" item-create-request-1 ",
		ItemTypeAPIKey,
		firstEnvelope,
	)
	if err != nil {
		t.Fatalf("NewItemCreateIdempotency() first error = %v", err)
	}

	second, err := NewItemCreateIdempotency(
		"item-create-request-1",
		ItemTypeAPIKey,
		secondEnvelope,
	)
	if err != nil {
		t.Fatalf("NewItemCreateIdempotency() second error = %v", err)
	}

	if !bytes.Equal(first.KeyHash[:], second.KeyHash[:]) {
		t.Fatal("equivalent idempotency keys produced different hashes")
	}

	if !bytes.Equal(first.RequestHash[:], second.RequestHash[:]) {
		t.Fatal("equivalent create requests produced different hashes")
	}
}

func TestNewItemCreateIdempotencySeparatesDifferentKeys(t *testing.T) {
	t.Parallel()

	envelope, err := NewSyntheticItemEnvelope(json.RawMessage(`{"value":"synthetic"}`))
	if err != nil {
		t.Fatalf("create envelope: %v", err)
	}

	first, err := NewItemCreateIdempotency("request-one", ItemTypeSecureNote, envelope)
	if err != nil {
		t.Fatalf("create first idempotency value: %v", err)
	}

	second, err := NewItemCreateIdempotency("request-two", ItemTypeSecureNote, envelope)
	if err != nil {
		t.Fatalf("create second idempotency value: %v", err)
	}

	if bytes.Equal(first.KeyHash[:], second.KeyHash[:]) {
		t.Fatal("different idempotency keys produced the same key hash")
	}

	if !bytes.Equal(first.RequestHash[:], second.RequestHash[:]) {
		t.Fatal("identical create requests produced different request hashes")
	}
}

func TestNewItemCreateIdempotencySeparatesDifferentRequests(t *testing.T) {
	t.Parallel()

	firstEnvelope, err := NewSyntheticItemEnvelope(json.RawMessage(`{"value":"first"}`))
	if err != nil {
		t.Fatalf("create first envelope: %v", err)
	}

	secondEnvelope, err := NewSyntheticItemEnvelope(json.RawMessage(`{"value":"second"}`))
	if err != nil {
		t.Fatalf("create second envelope: %v", err)
	}

	first, err := NewItemCreateIdempotency(
		"shared-request",
		ItemTypeSecureNote,
		firstEnvelope,
	)
	if err != nil {
		t.Fatalf("create first idempotency value: %v", err)
	}

	second, err := NewItemCreateIdempotency(
		"shared-request",
		ItemTypeSecureNote,
		secondEnvelope,
	)
	if err != nil {
		t.Fatalf("create second idempotency value: %v", err)
	}

	if !bytes.Equal(first.KeyHash[:], second.KeyHash[:]) {
		t.Fatal("identical keys produced different key hashes")
	}

	if bytes.Equal(first.RequestHash[:], second.RequestHash[:]) {
		t.Fatal("different payloads produced the same request hash")
	}

	differentType, err := NewItemCreateIdempotency(
		"shared-request",
		ItemTypeAPIKey,
		firstEnvelope,
	)
	if err != nil {
		t.Fatalf("create different-type idempotency value: %v", err)
	}

	if bytes.Equal(first.RequestHash[:], differentType.RequestHash[:]) {
		t.Fatal("different item types produced the same request hash")
	}
}

func TestNewItemCreateIdempotencyRejectsInvalidKeys(t *testing.T) {
	t.Parallel()

	envelope, err := NewSyntheticItemEnvelope(json.RawMessage(`{"value":"synthetic"}`))
	if err != nil {
		t.Fatalf("create envelope: %v", err)
	}

	for _, key := range []string{
		"",
		"   ",
		strings.Repeat("a", MaxItemIdempotencyKeyBytes+1),
	} {
		_, err := NewItemCreateIdempotency(key, ItemTypeSecureNote, envelope)

		if !errors.Is(err, ErrItemIdempotencyKeyInvalid) {
			t.Fatalf(
				"NewItemCreateIdempotency(%q) error = %v, want %v",
				key,
				err,
				ErrItemIdempotencyKeyInvalid,
			)
		}
	}
}

func TestNewItemCreateIdempotencyRejectsInvalidRequestData(t *testing.T) {
	t.Parallel()

	validEnvelope, err := NewSyntheticItemEnvelope(json.RawMessage(`{"value":"synthetic"}`))
	if err != nil {
		t.Fatalf("create envelope: %v", err)
	}

	tests := []struct {
		name     string
		itemType ItemType
		envelope SyntheticItemEnvelope
		wantErr  error
	}{
		{
			name:     "invalid item type",
			itemType: ItemType("invalid"),
			envelope: validEnvelope,
			wantErr:  ErrItemTypeInvalid,
		},
		{
			name:     "invalid payload",
			itemType: ItemTypeSecureNote,
			envelope: SyntheticItemEnvelope{
				Payload: json.RawMessage(`not-json`),
				Nonce:   validEnvelope.Nonce,
			},
			wantErr: ErrItemPayloadInvalid,
		},
		{
			name:     "invalid nonce",
			itemType: ItemTypeSecureNote,
			envelope: SyntheticItemEnvelope{
				Payload: validEnvelope.Payload,
				Nonce:   []byte("not-the-synthetic-nonce"),
			},
			wantErr: ErrItemEncryptedPayloadInvalid,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewItemCreateIdempotency(
				"valid-key",
				test.itemType,
				test.envelope,
			)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"NewItemCreateIdempotency() error = %v, want %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}
