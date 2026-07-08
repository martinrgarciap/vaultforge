package itemhandler

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestNewItemResourceForEncryptedItemDoesNotExposePlaintextPayload(t *testing.T) {
	t.Parallel()

	const plaintextMarker = "DO-NOT-LEAK-PLAINTEXT-MARKER"

	nonce := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	ciphertext := append(
		bytes.Repeat([]byte{0x41}, vault.ItemEncryptedPayloadTagBytes),
		[]byte(plaintextMarker)...,
	)

	storedItem := vault.Item{
		ID:        itemHandlerTestItemID,
		VaultID:   itemHandlerTestVaultID,
		Type:      vault.ItemTypeSecureNote,
		Payload:   ciphertext,
		Nonce:     nonce,
		Version:   2,
		CreatedAt: time.Date(2026, time.July, 8, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.July, 8, 12, 5, 0, 0, time.UTC),
	}

	resource, err := newItemResource(storedItem)
	if err != nil {
		t.Fatalf("newItemResource() error = %v", err)
	}

	envelope, err := encryptedItemEnvelopeFromResource(resource.EncryptedPayload)
	if err != nil {
		t.Fatalf("parse encrypted payload resource: %v", err)
	}

	blob, err := envelope.Blob()
	if err != nil {
		t.Fatalf("join encrypted envelope blob: %v", err)
	}

	wantBlob := append(append([]byte{}, nonce...), ciphertext...)

	if !bytes.Equal(blob, wantBlob) {
		t.Fatal("encrypted payload resource did not preserve nonce plus ciphertext bytes")
	}

	encoded, err := json.Marshal(itemResponse{Item: resource})
	if err != nil {
		t.Fatalf("encode item response: %v", err)
	}

	responseBody := string(encoded)

	if strings.Contains(responseBody, `"payload"`) {
		t.Fatal("encrypted item response included plaintext payload field")
	}

	if strings.Contains(responseBody, plaintextMarker) {
		t.Fatal("encrypted item response exposed raw plaintext marker")
	}

	if !strings.Contains(responseBody, `"encryptedPayload"`) {
		t.Fatal("encrypted item response omitted encryptedPayload field")
	}
}

func TestNewItemResourceRejectsPlaintextStoredItem(t *testing.T) {
	t.Parallel()

	storedItem := vault.Item{
		ID:        itemHandlerTestItemID,
		VaultID:   itemHandlerTestVaultID,
		Type:      vault.ItemTypeSecureNote,
		Payload:   []byte(`{"secret":"plaintext-must-not-be-exposed"}`),
		Nonce:     vault.SyntheticItemNonce(),
		Version:   1,
		CreatedAt: time.Date(2026, time.July, 8, 13, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.July, 8, 13, 5, 0, 0, time.UTC),
	}

	_, err := newItemResource(storedItem)
	if err == nil {
		t.Fatal("newItemResource() error = nil, want an error")
	}
}
