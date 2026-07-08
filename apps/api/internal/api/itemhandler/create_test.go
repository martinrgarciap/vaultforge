package itemhandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
	"go.uber.org/zap"
)

func TestHandlerCreatesVaultItem(t *testing.T) {
	t.Parallel()

	service := &itemHandlerTestService{
		createResult: itemHandlerCreateResult(),
	}

	router := newItemHandlerTestRouter(service)

	request := newCreateItemTestRequest(
		validEncryptedItemRequestBody(t, vault.ItemTypeAPIKey),
		"application/json",
		"item-create-request-123",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			http.StatusCreated,
			recorder.Body.String(),
		)
	}

	if service.createCalls != 1 {
		t.Fatalf(
			"CreateItem() calls = %d, want 1",
			service.createCalls,
		)
	}

	if service.lastCreateInput.OwnerID !=
		itemHandlerTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			service.lastCreateInput.OwnerID,
			itemHandlerTestOwnerID,
		)
	}

	if service.lastCreateInput.VaultID !=
		itemHandlerCreateTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			service.lastCreateInput.VaultID,
			itemHandlerCreateTestVaultID,
		)
	}

	if service.lastCreateInput.Type !=
		vault.ItemTypeAPIKey {
		t.Fatalf(
			"item type = %q, want %q",
			service.lastCreateInput.Type,
			vault.ItemTypeAPIKey,
		)
	}

	if service.lastCreateInput.IdempotencyKey !=
		"item-create-request-123" {
		t.Fatalf(
			"idempotency key = %q, want %q",
			service.lastCreateInput.IdempotencyKey,
			"item-create-request-123",
		)
	}

	if service.lastCreateInput.CorrelationID !=
		itemHandlerTestRequestID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			service.lastCreateInput.CorrelationID,
			itemHandlerTestRequestID,
		)
	}

	if recorder.Header().Get("ETag") != `"1"` {
		t.Fatalf(
			"ETag = %q, want %q",
			recorder.Header().Get("ETag"),
			`"1"`,
		)
	}

	wantLocation := "/v1/vaults/" +
		itemHandlerCreateTestVaultID +
		"/items/" +
		itemHandlerCreateTestItemID

	if recorder.Header().Get("Location") != wantLocation {
		t.Fatalf(
			"Location = %q, want %q",
			recorder.Header().Get("Location"),
			wantLocation,
		)
	}

	var body itemResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode create item response: %v",
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

	if body.Item.Version != 1 {
		t.Fatalf(
			"item version = %d, want 1",
			body.Item.Version,
		)
	}

	responseBody := recorder.Body.String()

	if strings.Contains(
		responseBody,
		itemHandlerTestOwnerID,
	) {
		t.Fatal(
			"item response exposed the owner ID",
		)
	}

	if strings.Contains(
		responseBody,
		itemHandlerCreateTestVaultID,
	) {
		t.Fatal(
			"item response exposed the parent vault ID",
		)
	}
}

func TestHandlerCreateRequiresIdempotencyKey(
	t *testing.T,
) {
	t.Parallel()

	service := &itemHandlerTestService{}
	router := newItemHandlerTestRouter(service)

	request := newCreateItemTestRequest(
		`{"type":"secure_note","payload":{}}`,
		"application/json",
		"",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusBadRequest,
		"idempotency_key_required",
	)

	if service.createCalls != 0 {
		t.Fatal(
			"service was called without an idempotency key",
		)
	}
}

func TestHandlerCreateRejectsInvalidJSON(
	t *testing.T,
) {
	t.Parallel()

	service := &itemHandlerTestService{}
	router := newItemHandlerTestRouter(service)

	request := newCreateItemTestRequest(
		`{
			"type": "secure_note",
			"payload": {},
			"ownerID": "client-selected-owner"
		}`,
		"application/json",
		"item-create-request-123",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusBadRequest,
		"invalid_request",
	)

	if service.createCalls != 0 {
		t.Fatal(
			"service was called for invalid JSON",
		)
	}
}

func TestHandlerCreateMapsServiceErrors(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid item type",
			serviceErr: vault.ErrItemTypeInvalid,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_item_type",
		},
		{
			name:       "invalid item payload",
			serviceErr: vault.ErrItemPayloadNotObject,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_item_payload",
		},
		{
			name:       "missing vault",
			serviceErr: vault.ErrVaultNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "vault_not_found",
		},
		{
			name:       "idempotency conflict",
			serviceErr: vault.ErrItemIdempotencyConflict,
			wantStatus: http.StatusConflict,
			wantCode:   "idempotency_conflict",
		},
		{
			name:       "unavailable service",
			serviceErr: vault.ErrItemUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "item_unavailable",
		},
		{
			name:       "unexpected failure",
			serviceErr: errors.New("synthetic internal failure"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &itemHandlerTestService{
				createErr: test.serviceErr,
			}
			router := newItemHandlerTestRouter(service)

			request := newCreateItemTestRequest(
				validEncryptedItemRequestBody(t, vault.ItemTypeSecureNote),
				"application/json",
				"item-create-request-123",
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
					"response exposed the internal service error",
				)
			}
		})
	}
}

func TestHandlerCreateRejectsMissingPrincipal(
	t *testing.T,
) {
	t.Parallel()

	service := &itemHandlerTestService{}
	handler := New(service, zap.NewNop().Sugar())

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Post(
		"/v1/vaults/{vaultID}/items",
		handler.Create,
	)

	request := newCreateItemTestRequest(
		validEncryptedItemRequestBody(t, vault.ItemTypeSecureNote),
		"application/json",
		"item-create-request-123",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusUnauthorized,
		"unauthorized",
	)

	if recorder.Header().Get(
		"WWW-Authenticate",
	) != "Bearer" {
		t.Fatal(
			"unauthorized response omitted WWW-Authenticate",
		)
	}

	if service.createCalls != 0 {
		t.Fatal(
			"service was called without a principal",
		)
	}
}

func TestHandlerCreateRejectsPlaintextPayload(t *testing.T) {
	t.Parallel()

	service := &itemHandlerTestService{}
	router := newItemHandlerTestRouter(service)

	request := newCreateItemTestRequest(
		`{
			"type": "secure_note",
			"payload": {
				"secret": "plaintext-must-not-be-accepted"
			}
		}`,
		"application/json",
		"item-create-request-123",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusBadRequest,
		"invalid_request",
	)

	if service.createCalls != 0 {
		t.Fatal("service was called for a plaintext payload create request")
	}
}

func validEncryptedItemRequestBody(t *testing.T, itemType vault.ItemType) string {
	t.Helper()

	body, err := json.Marshal(struct {
		Type             vault.ItemType               `json:"type"`
		EncryptedPayload itemEncryptedPayloadResource `json:"encryptedPayload"`
	}{
		Type:             itemType,
		EncryptedPayload: validItemEncryptedPayloadResource(),
	})
	if err != nil {
		t.Fatalf("marshal encrypted item request body: %v", err)
	}

	return string(body)
}
