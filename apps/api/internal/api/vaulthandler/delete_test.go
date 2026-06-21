package vaulthandler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestHandlerDeletesOwnedVault(t *testing.T) {
	t.Parallel()

	service := &handlerTestVaultService{}
	router := newHandlerTestRouter(service)

	request := newDeleteVaultTestRequest(handlerTestVaultID)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}

	if recorder.Body.Len() != 0 {
		t.Fatalf("response body length = %d, want 0", recorder.Body.Len())
	}

	if service.deleteCalls != 1 {
		t.Fatalf("Delete() calls = %d, want 1", service.deleteCalls)
	}

	if service.lastDeleteInput.OwnerID != handlerTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			service.lastDeleteInput.OwnerID,
			handlerTestOwnerID,
		)
	}

	if service.lastDeleteInput.VaultID != handlerTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			service.lastDeleteInput.VaultID,
			handlerTestVaultID,
		)
	}

	if service.lastDeleteInput.CorrelationID == "" {
		t.Fatal("delete input did not contain a correlation ID")
	}
}

func TestHandlerDeleteMapsNotFoundSafely(t *testing.T) {
	t.Parallel()

	service := &handlerTestVaultService{deleteErr: vault.ErrVaultNotFound}
	router := newHandlerTestRouter(service)

	request := newDeleteVaultTestRequest(handlerTestVaultID)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusNotFound,
		"vault_not_found",
	)
}

func TestHandlerDeleteMapsUnavailableService(t *testing.T) {
	t.Parallel()

	service := &handlerTestVaultService{deleteErr: vault.ErrVaultUnavailable}
	router := newHandlerTestRouter(service)

	request := newDeleteVaultTestRequest(handlerTestVaultID)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"vault_unavailable",
	)
}

func TestHandlerDeleteUsesSafeNotFoundForInvalidID(t *testing.T) {
	t.Parallel()

	service := &handlerTestVaultService{deleteErr: vault.ErrVaultNotFound}
	router := newHandlerTestRouter(service)

	request := newDeleteVaultTestRequest("invalid-vault-id")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusNotFound,
		"vault_not_found",
	)

	if service.lastDeleteInput.OwnerID != handlerTestOwnerID {
		t.Fatal("delete did not use the authenticated owner")
	}
}

func newDeleteVaultTestRequest(vaultID string) *http.Request {
	request := httptest.NewRequest(
		http.MethodDelete,
		"/v1/vaults/"+vaultID,
		nil,
	)

	request.Header.Set("Authorization", "Bearer synthetic-access-token")

	return request
}
