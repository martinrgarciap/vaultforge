package itemhandler

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const itemHandlerTestVaultID = "00000000-0000-0000-0000-000000002002"
const itemHandlerTestItemID = "00000000-0000-0000-0000-000000002003"

func TestNewItemResourceMapsPublicFields(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		23,
		17,
		0,
		0,
		123456000,
		time.UTC,
	)

	updatedAt := createdAt.Add(time.Minute)
	deletedAt := updatedAt.Add(time.Minute)

	storedItem := vault.Item{
		ID:        itemHandlerTestItemID,
		VaultID:   itemHandlerTestVaultID,
		Type:      vault.ItemTypeAPIKey,
		Payload:   append([]byte{}, bytes.Repeat([]byte{0x41}, vault.ItemEncryptedPayloadTagBytes+4)...),
		Nonce:     []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		Version:   3,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		DeletedAt: &deletedAt,
	}

	resource, err := newItemResource(storedItem)
	if err != nil {
		t.Fatalf("newItemResource() error = %v", err)
	}

	if resource.ID != storedItem.ID {
		t.Fatalf("resource ID = %q, want %q", resource.ID, storedItem.ID)
	}

	if resource.Type != storedItem.Type {
		t.Fatalf(
			"resource type = %q, want %q",
			resource.Type,
			storedItem.Type,
		)
	}

	if resource.EncryptedPayload.Version != vault.ItemEncryptedPayloadVersion {
		t.Fatalf(
			"encrypted payload version = %d, want %d",
			resource.EncryptedPayload.Version,
			vault.ItemEncryptedPayloadVersion,
		)
	}

	if resource.Version != storedItem.Version {
		t.Fatalf(
			"resource version = %d, want %d",
			resource.Version,
			storedItem.Version,
		)
	}

	if resource.DeletedAt == nil ||
		!resource.DeletedAt.Equal(deletedAt) {
		t.Fatalf(
			"resource deletion time = %v, want %v",
			resource.DeletedAt,
			deletedAt,
		)
	}

	encoded, err := json.Marshal(
		itemResponse{
			Item: resource,
		},
	)
	if err != nil {
		t.Fatalf("encode item response: %v", err)
	}

	responseBody := string(encoded)

	if strings.Contains(responseBody, itemHandlerTestVaultID) {
		t.Fatal("item response exposed the parent vault ID")
	}

	if strings.Contains(responseBody, "owner") {
		t.Fatal("item response exposed an owner field")
	}

	if strings.Contains(responseBody, `"payload"`) {
		t.Fatal("item response included a plaintext payload field")
	}
}

func TestNewItemListResponseEncodesNextCursor(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(
		2026,
		time.June,
		23,
		18,
		0,
		0,
		0,
		time.UTC,
	)

	page := vault.ItemPage{
		Items: []vault.Item{
			{
				ID:        itemHandlerTestItemID,
				VaultID:   itemHandlerTestVaultID,
				Type:      vault.ItemTypeSecureNote,
				Payload:   append([]byte{}, bytes.Repeat([]byte{0x41}, vault.ItemEncryptedPayloadTagBytes+4)...),
				Nonce:     []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
				Version:   1,
				CreatedAt: updatedAt.Add(-time.Minute),
				UpdatedAt: updatedAt,
			},
		},
		NextCursor: &vault.ItemCursor{
			UpdatedAt: updatedAt,
			ID:        itemHandlerTestItemID,
		},
	}

	response, err := newItemListResponse(page)
	if err != nil {
		t.Fatalf("newItemListResponse() error = %v", err)
	}

	if len(response.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(response.Items))
	}

	if response.NextCursor == "" {
		t.Fatal("list response did not contain a next cursor")
	}

	decodedCursor, err := decodeItemCursor(response.NextCursor)
	if err != nil {
		t.Fatalf("decode response cursor: %v", err)
	}

	if decodedCursor.ID != itemHandlerTestItemID {
		t.Fatalf(
			"cursor ID = %q, want %q",
			decodedCursor.ID,
			itemHandlerTestItemID,
		)
	}

	if !decodedCursor.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"cursor time = %v, want %v",
			decodedCursor.UpdatedAt,
			updatedAt,
		)
	}
}

func TestNewItemListResponseUsesEmptyArrayWithoutCursor(t *testing.T) {
	t.Parallel()

	response, err := newItemListResponse(
		vault.ItemPage{
			Items: nil,
		},
	)
	if err != nil {
		t.Fatalf("newItemListResponse() error = %v", err)
	}

	if response.Items == nil {
		t.Fatal("empty item response contained a nil slice")
	}

	if len(response.Items) != 0 {
		t.Fatalf("item count = %d, want 0", len(response.Items))
	}

	if response.NextCursor != "" {
		t.Fatalf(
			"next cursor = %q, want empty",
			response.NextCursor,
		)
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("encode empty item list: %v", err)
	}

	if string(encoded) != `{"items":[]}` {
		t.Fatalf(
			"empty list response = %s, want %s",
			encoded,
			`{"items":[]}`,
		)
	}
}

func TestNewItemListResponseRejectsInvalidNextCursor(t *testing.T) {
	t.Parallel()

	_, err := newItemListResponse(
		vault.ItemPage{
			Items: []vault.Item{},
			NextCursor: &vault.ItemCursor{
				ID: itemHandlerTestItemID,
			},
		},
	)

	if err == nil {
		t.Fatal("newItemListResponse() error = nil, want an error")
	}
}
