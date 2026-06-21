package vaulthandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestHandlerRenamesOwnedVault(
	t *testing.T,
) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		21,
		19,
		0,
		0,
		123456000,
		time.UTC,
	)

	updatedAt := createdAt.Add(time.Minute)

	service := &handlerTestVaultService{
		renameResult: vault.Vault{
			ID:        handlerTestVaultID,
			OwnerID:   handlerTestOwnerID,
			Name:      "Renamed Vault",
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	router := newHandlerTestRouter(service)

	request := newRenameVaultTestRequest(
		`{"name":"Renamed Vault"}`,
		"application/json",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if service.renameCalls != 1 {
		t.Fatalf("Rename() calls = %d, want 1", service.renameCalls)
	}

	if service.lastRenameInput.OwnerID !=
		handlerTestOwnerID {
		t.Fatalf("owner ID = %q, want %q",
			service.lastRenameInput.OwnerID,
			handlerTestOwnerID,
		)
	}

	if service.lastRenameInput.VaultID !=
		handlerTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			service.lastRenameInput.VaultID,
			handlerTestVaultID,
		)
	}

	if service.lastRenameInput.Name !=
		"Renamed Vault" {
		t.Fatalf("name = %q, want Renamed Vault", service.lastRenameInput.Name)
	}

	if service.lastRenameInput.CorrelationID == "" {
		t.Fatal("rename input did not contain a correlation ID")
	}

	var body vaultResponse

	err := json.NewDecoder(
		recorder.Body,
	).Decode(&body)
	if err != nil {
		t.Fatalf("decode rename response: %v", err)
	}

	if body.Vault.ID != handlerTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			body.Vault.ID,
			handlerTestVaultID,
		)
	}

	if body.Vault.Name != "Renamed Vault" {
		t.Fatalf("vault name = %q, want Renamed Vault", body.Vault.Name)
	}

	if !body.Vault.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"updated time = %v, want %v",
			body.Vault.UpdatedAt,
			updatedAt,
		)
	}

	if strings.Contains(
		recorder.Body.String(),
		handlerTestOwnerID,
	) {
		t.Fatal("rename response exposed the internal owner ID")
	}
}

func TestHandlerRenameRejectsInvalidRequest(
	t *testing.T,
) {
	t.Parallel()

	service := &handlerTestVaultService{}
	router := newHandlerTestRouter(service)

	request := newRenameVaultTestRequest(
		`{
			"name": "Renamed Vault",
			"ownerID": "client-selected-owner"
		}`,
		"application/json",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusBadRequest,
		"invalid_request",
	)

	if service.renameCalls != 0 {
		t.Fatal("service was called for an invalid rename request")
	}
}

func TestHandlerRenameMapsInvalidName(
	t *testing.T,
) {
	t.Parallel()

	service := &handlerTestVaultService{renameErr: vault.ErrVaultNameEmpty}

	router := newHandlerTestRouter(service)

	request := newRenameVaultTestRequest(`{"name":"   "}`, "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusUnprocessableEntity,
		"invalid_vault_name",
	)
}

func TestHandlerRenameMapsNotFoundSafely(
	t *testing.T,
) {
	t.Parallel()

	service := &handlerTestVaultService{renameErr: vault.ErrVaultNotFound}

	router := newHandlerTestRouter(service)

	request := newRenameVaultTestRequest(`{"name":"Renamed Vault"}`, "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusNotFound,
		"vault_not_found",
	)
}

func TestHandlerRenameMapsUnavailableService(
	t *testing.T,
) {
	t.Parallel()

	service := &handlerTestVaultService{renameErr: vault.ErrVaultUnavailable}

	router := newHandlerTestRouter(service)

	request := newRenameVaultTestRequest(`{"name":"Renamed Vault"}`, "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"vault_unavailable",
	)
}

func newRenameVaultTestRequest(
	body string,
	contentType string,
) *http.Request {
	request := httptest.NewRequest(
		http.MethodPatch,
		"/v1/vaults/"+handlerTestVaultID,
		strings.NewReader(body),
	)

	request.Header.Set("Authorization", "Bearer synthetic-access-token")

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	return request
}
