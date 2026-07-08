package itemhandler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestHandlerUpdatesVaultItem(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(
		2026,
		time.June,
		23,
		23,
		0,
		0,
		123456000,
		time.UTC,
	)

	service := &itemHandlerTestService{
		updateResult: vault.Item{
			ID:      itemHandlerCreateTestItemID,
			VaultID: itemHandlerCreateTestVaultID,
			Type:    vault.ItemTypeAPIKey,
			Payload: bytes.Repeat(
				[]byte{0x41},
				vault.ItemEncryptedPayloadTagBytes+4,
			),
			Nonce:     []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			Version:   3,
			CreatedAt: updatedAt.Add(-time.Hour),
			UpdatedAt: updatedAt,
		},
	}

	router := newItemHandlerTestRouter(service)

	request := newUpdateItemTestRequest(
		validEncryptedItemRequestBody(t, vault.ItemTypeAPIKey),
		"application/json",
		`"2"`,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			http.StatusOK,
			recorder.Body.String(),
		)
	}

	if service.updateCalls != 1 {
		t.Fatalf(
			"UpdateItem() calls = %d, want 1",
			service.updateCalls,
		)
	}

	input := service.lastUpdateInput

	if input.OwnerID != itemHandlerTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			input.OwnerID,
			itemHandlerTestOwnerID,
		)
	}

	if input.VaultID != itemHandlerCreateTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			input.VaultID,
			itemHandlerCreateTestVaultID,
		)
	}

	if input.ItemID != itemHandlerCreateTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			input.ItemID,
			itemHandlerCreateTestItemID,
		)
	}

	if input.Type != vault.ItemTypeAPIKey {
		t.Fatalf(
			"item type = %q, want %q",
			input.Type,
			vault.ItemTypeAPIKey,
		)
	}

	if input.ExpectedVersion != 2 {
		t.Fatalf(
			"expected version = %d, want 2",
			input.ExpectedVersion,
		)
	}

	if input.CorrelationID == "" {
		t.Fatal(
			"update input omitted the correlation ID",
		)
	}

	if recorder.Header().Get("ETag") != `"3"` {
		t.Fatalf(
			"ETag = %q, want %q",
			recorder.Header().Get("ETag"),
			`"3"`,
		)
	}

	var body itemResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode update response: %v",
			err,
		)
	}

	if body.Item.ID != itemHandlerCreateTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			body.Item.ID,
			itemHandlerCreateTestItemID,
		)
	}

	if body.Item.Version != 3 {
		t.Fatalf(
			"item version = %d, want 3",
			body.Item.Version,
		)
	}

	if strings.Contains(
		recorder.Body.String(),
		itemHandlerTestOwnerID,
	) {
		t.Fatal(
			"update response exposed the owner ID",
		)
	}

	if strings.Contains(
		recorder.Body.String(),
		itemHandlerCreateTestVaultID,
	) {
		t.Fatal(
			"update response exposed the parent vault ID",
		)
	}
}

func TestHandlerUpdateRequiresIfMatch(t *testing.T) {
	t.Parallel()

	service := &itemHandlerTestService{}
	router := newItemHandlerTestRouter(service)

	request := newUpdateItemTestRequest(
		`{"type":"secure_note","payload":{}}`,
		"application/json",
		"",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusPreconditionRequired,
		"if_match_required",
	)

	if service.updateCalls != 0 {
		t.Fatal(
			"service was called without If-Match",
		)
	}
}

func TestHandlerUpdateRejectsInvalidIfMatch(t *testing.T) {
	t.Parallel()

	service := &itemHandlerTestService{}
	router := newItemHandlerTestRouter(service)

	request := newUpdateItemTestRequest(
		`{"type":"secure_note","payload":{}}`,
		"application/json",
		"1",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusBadRequest,
		"invalid_if_match",
	)

	if service.updateCalls != 0 {
		t.Fatal(
			"service was called with invalid If-Match",
		)
	}
}

func TestHandlerUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	service := &itemHandlerTestService{}
	router := newItemHandlerTestRouter(service)

	request := newUpdateItemTestRequest(
		`{
			"type": "secure_note",
			"payload": {},
			"expectedVersion": 2
		}`,
		"application/json",
		`"2"`,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusBadRequest,
		"invalid_request",
	)

	if service.updateCalls != 0 {
		t.Fatal(
			"service was called for an invalid update body",
		)
	}
}

func TestHandlerUpdateMapsServiceErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "item not found",
			serviceErr: vault.ErrItemNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "item_not_found",
		},
		{
			name:       "invalid item type",
			serviceErr: vault.ErrItemTypeInvalid,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_item_type",
		},
		{
			name:       "invalid item payload",
			serviceErr: vault.ErrItemPayloadInvalid,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_item_payload",
		},
		{
			name:       "version conflict",
			serviceErr: vault.ErrItemConflict,
			wantStatus: http.StatusPreconditionFailed,
			wantCode:   "item_version_conflict",
		},
		{
			name:       "unavailable service",
			serviceErr: vault.ErrItemUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "item_unavailable",
		},
		{
			name:       "unexpected failure",
			serviceErr: errors.New("synthetic update failure"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &itemHandlerTestService{
				updateErr: test.serviceErr,
			}
			router := newItemHandlerTestRouter(service)

			request := newUpdateItemTestRequest(
				validEncryptedItemRequestBody(t, vault.ItemTypeSecureNote),
				"application/json",
				`"1"`,
			)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			assertItemHandlerTestError(
				t,
				recorder,
				test.wantStatus,
				test.wantCode,
			)

			if strings.Contains(
				recorder.Body.String(),
				test.serviceErr.Error(),
			) {
				t.Fatal(
					"response exposed the service error",
				)
			}
		})
	}
}

func TestHandlerUpdateRejectsPlaintextPayload(t *testing.T) {
	t.Parallel()

	service := &itemHandlerTestService{}
	router := newItemHandlerTestRouter(service)

	request := newUpdateItemTestRequest(
		`{
			"type": "secure_note",
			"payload": {
				"secret": "plaintext-must-not-be-accepted"
			}
		}`,
		"application/json",
		`"1"`,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusBadRequest,
		"invalid_request",
	)

	if service.updateCalls != 0 {
		t.Fatal("service was called for a plaintext payload update request")
	}
}

func newUpdateItemTestRequest(
	body string,
	contentType string,
	ifMatch string,
) *http.Request {
	request := httptest.NewRequest(
		http.MethodPut,
		"/v1/vaults/"+
			itemHandlerCreateTestVaultID+
			"/items/"+
			itemHandlerCreateTestItemID,
		strings.NewReader(body),
	)

	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
	)
	request.Header.Set(
		"X-Request-ID",
		itemHandlerTestRequestID,
	)

	if contentType != "" {
		request.Header.Set(
			"Content-Type",
			contentType,
		)
	}

	if ifMatch != "" {
		request.Header.Set(
			"If-Match",
			ifMatch,
		)
	}

	return request
}
