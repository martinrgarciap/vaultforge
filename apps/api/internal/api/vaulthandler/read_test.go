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

const handlerTestSecondVaultID = "00000000-0000-0000-0000-000000000804"

func TestHandlerListsOwnedVaults(
	t *testing.T,
) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		21,
		17,
		0,
		0,
		123456000,
		time.UTC,
	)

	service := &handlerTestVaultService{
		listResult: []vault.Vault{
			{
				ID:        handlerTestVaultID,
				OwnerID:   handlerTestOwnerID,
				Name:      "Development",
				CreatedAt: createdAt,
				UpdatedAt: createdAt.Add(time.Minute),
			},
			{
				ID:        handlerTestSecondVaultID,
				OwnerID:   handlerTestOwnerID,
				Name:      "Personal",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
		},
	}

	router := newHandlerTestRouter(service)

	request := newAuthorizedVaultTestRequest(
		http.MethodGet,
		"/v1/vaults",
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

	if service.listCalls != 1 {
		t.Fatalf(
			"List() calls = %d, want 1",
			service.listCalls,
		)
	}

	if service.lastListOwnerID !=
		handlerTestOwnerID {
		t.Fatalf(
			"list owner ID = %q, want %q",
			service.lastListOwnerID,
			handlerTestOwnerID,
		)
	}

	var body vaultListResponse

	err := json.NewDecoder(
		recorder.Body,
	).Decode(&body)
	if err != nil {
		t.Fatalf(
			"decode list response: %v",
			err,
		)
	}

	if len(body.Vaults) != 2 {
		t.Fatalf(
			"vault count = %d, want 2",
			len(body.Vaults),
		)
	}

	if body.Vaults[0].ID !=
		handlerTestVaultID {
		t.Fatalf(
			"first vault ID = %q, want %q",
			body.Vaults[0].ID,
			handlerTestVaultID,
		)
	}

	if body.Vaults[1].ID !=
		handlerTestSecondVaultID {
		t.Fatalf(
			"second vault ID = %q, want %q",
			body.Vaults[1].ID,
			handlerTestSecondVaultID,
		)
	}

	if strings.Contains(
		recorder.Body.String(),
		handlerTestOwnerID,
	) {
		t.Fatal(
			"vault list exposed the internal owner ID",
		)
	}
}

func TestHandlerListsEmptyVaultCollection(
	t *testing.T,
) {
	t.Parallel()

	service := &handlerTestVaultService{
		listResult: nil,
	}

	router := newHandlerTestRouter(service)

	request := newAuthorizedVaultTestRequest(
		http.MethodGet,
		"/v1/vaults",
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

	var body vaultListResponse

	err := json.NewDecoder(
		recorder.Body,
	).Decode(&body)
	if err != nil {
		t.Fatalf(
			"decode empty list response: %v",
			err,
		)
	}

	if body.Vaults == nil {
		t.Fatal(
			"empty vault collection was null",
		)
	}

	if len(body.Vaults) != 0 {
		t.Fatalf(
			"vault count = %d, want 0",
			len(body.Vaults),
		)
	}
}

func TestHandlerGetsOwnedVault(
	t *testing.T,
) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		21,
		18,
		0,
		0,
		123456000,
		time.UTC,
	)

	service := &handlerTestVaultService{
		getResult: vault.Vault{
			ID:        handlerTestVaultID,
			OwnerID:   handlerTestOwnerID,
			Name:      "Development",
			CreatedAt: createdAt,
			UpdatedAt: createdAt.Add(time.Minute),
		},
	}

	router := newHandlerTestRouter(service)

	request := newAuthorizedVaultTestRequest(
		http.MethodGet,
		"/v1/vaults/"+handlerTestVaultID,
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

	if service.getCalls != 1 {
		t.Fatalf(
			"Get() calls = %d, want 1",
			service.getCalls,
		)
	}

	if service.lastGetOwnerID !=
		handlerTestOwnerID {
		t.Fatalf(
			"get owner ID = %q, want %q",
			service.lastGetOwnerID,
			handlerTestOwnerID,
		)
	}

	if service.lastGetVaultID !=
		handlerTestVaultID {
		t.Fatalf(
			"get vault ID = %q, want %q",
			service.lastGetVaultID,
			handlerTestVaultID,
		)
	}

	var body vaultResponse

	err := json.NewDecoder(
		recorder.Body,
	).Decode(&body)
	if err != nil {
		t.Fatalf(
			"decode get response: %v",
			err,
		)
	}

	if body.Vault.ID != handlerTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			body.Vault.ID,
			handlerTestVaultID,
		)
	}

	if body.Vault.Name != "Development" {
		t.Fatalf(
			"vault name = %q, want Development",
			body.Vault.Name,
		)
	}

	if strings.Contains(
		recorder.Body.String(),
		handlerTestOwnerID,
	) {
		t.Fatal(
			"vault response exposed the internal owner ID",
		)
	}
}

func TestHandlerGetMapsNotFoundSafely(
	t *testing.T,
) {
	t.Parallel()

	service := &handlerTestVaultService{
		getErr: vault.ErrVaultNotFound,
	}

	router := newHandlerTestRouter(service)

	request := newAuthorizedVaultTestRequest(
		http.MethodGet,
		"/v1/vaults/"+
			"00000000-0000-0000-0000-000000009999",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusNotFound,
		"vault_not_found",
	)
}

func TestHandlerListMapsUnavailableService(
	t *testing.T,
) {
	t.Parallel()

	service := &handlerTestVaultService{
		listErr: vault.ErrVaultUnavailable,
	}

	router := newHandlerTestRouter(service)

	request := newAuthorizedVaultTestRequest(
		http.MethodGet,
		"/v1/vaults",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"vault_unavailable",
	)
}

func newAuthorizedVaultTestRequest(
	method string,
	target string,
) *http.Request {
	request := httptest.NewRequest(
		method,
		target,
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
	)

	return request
}
