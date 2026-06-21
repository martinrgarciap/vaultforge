package vaulthandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
	"go.uber.org/zap"
)

const (
	handlerTestOwnerID   = "00000000-0000-0000-0000-000000000801"
	handlerTestSessionID = "00000000-0000-0000-0000-000000000802"
	handlerTestVaultID   = "00000000-0000-0000-0000-000000000803"
	handlerTestRequestID = "vault-handler-test-request-id"
)

func TestHandlerCreatesVault(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		21,
		16,
		0,
		0,
		123456000,
		time.UTC,
	)

	service := &handlerTestVaultService{
		createResult: vault.Vault{
			ID:        handlerTestVaultID,
			OwnerID:   handlerTestOwnerID,
			Name:      "Development",
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	router := newHandlerTestRouter(service)
	request := newCreateVaultTestRequest(`{"name":"Development"}`, "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	if service.createCalls != 1 {
		t.Fatalf("Create() calls = %d, want 1", service.createCalls)
	}

	if service.lastCreateInput.OwnerID != handlerTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			service.lastCreateInput.OwnerID,
			handlerTestOwnerID,
		)
	}

	if service.lastCreateInput.Name != "Development" {
		t.Fatalf("vault name = %q, want Development", service.lastCreateInput.Name)
	}

	if service.lastCreateInput.CorrelationID != handlerTestRequestID {
		t.Fatalf(
			"correlation ID = %q, want %q",
			service.lastCreateInput.CorrelationID,
			handlerTestRequestID,
		)
	}

	var body vaultResponse

	err := json.NewDecoder(recorder.Body).Decode(&body)
	if err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	if body.Vault.ID != handlerTestVaultID {
		t.Fatalf("vault ID = %q, want %q", body.Vault.ID, handlerTestVaultID)
	}

	if body.Vault.Name != "Development" {
		t.Fatalf("vault name = %q, want Development", body.Vault.Name)
	}

	if !body.Vault.CreatedAt.Equal(createdAt) {
		t.Fatalf("created time = %v, want %v", body.Vault.CreatedAt, createdAt)
	}

	if strings.Contains(recorder.Body.String(), handlerTestOwnerID) {
		t.Fatal("vault response exposed the internal owner ID")
	}

}

func TestHandlerCreateRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
		wantCode    string
	}{
		{
			name:       "missing content type",
			body:       `{"name":"Development"}`,
			wantStatus: http.StatusUnsupportedMediaType,
			wantCode:   "unsupported_media_type",
		},
		{
			name:        "malformed JSON",
			body:        `{"name":`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
		},
		{
			name: "unknown field",
			body: `{
			"name": "Development",
			"ownerID": "client-selected-owner"
		}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
		},
		{
			name:        "body too large",
			body:        `{"name":"` + strings.Repeat("a", 5000) + `"}`,
			contentType: "application/json",
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantCode:    "request_body_too_large",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &handlerTestVaultService{}
			router := newHandlerTestRouter(service)
			request := newCreateVaultTestRequest(test.body, test.contentType)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			assertHandlerTestError(t, recorder, test.wantStatus, test.wantCode)

			if service.createCalls != 0 {
				t.Fatal("service was called for an invalid request")
			}
		})
	}

}

func TestHandlerCreateMapsInvalidName(t *testing.T) {
	t.Parallel()

	service := &handlerTestVaultService{createErr: vault.ErrVaultNameEmpty}
	router := newHandlerTestRouter(service)
	request := newCreateVaultTestRequest(`{"name":"   "}`, "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusUnprocessableEntity,
		"invalid_vault_name",
	)

}

func TestHandlerCreateMapsUnavailableService(t *testing.T) {
	t.Parallel()

	service := &handlerTestVaultService{createErr: vault.ErrVaultUnavailable}
	router := newHandlerTestRouter(service)
	request := newCreateVaultTestRequest(`{"name":"Development"}`, "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"vault_unavailable",
	)

}

func TestHandlerCreateDoesNotExposeInternalFailure(t *testing.T) {
	t.Parallel()

	const internalMarker = "synthetic internal vault failure"

	service := &handlerTestVaultService{createErr: errors.New(internalMarker)}
	router := newHandlerTestRouter(service)
	request := newCreateVaultTestRequest(`{"name":"Development"}`, "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusInternalServerError,
		"internal_error",
	)

	if strings.Contains(recorder.Body.String(), internalMarker) {
		t.Fatal("vault response exposed an internal error")
	}

}

func TestHandlerCreateRejectsMissingPrincipal(t *testing.T) {
	t.Parallel()

	service := &handlerTestVaultService{}
	handler := New(service, zap.NewNop().Sugar())

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Post("/v1/vaults", handler.Create)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/vaults",
		strings.NewReader(`{"name":"Development"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusUnauthorized,
		"unauthorized",
	)

	if recorder.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("unauthorized response did not include WWW-Authenticate")
	}

	if service.createCalls != 0 {
		t.Fatal("service was called without a principal")
	}

}

func TestHandlerCreateRejectsUnavailableDependency(t *testing.T) {
	t.Parallel()

	handler := New(nil, zap.NewNop().Sugar())

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Post("/v1/vaults", handler.Create)

	request := httptest.NewRequest(http.MethodPost, "/v1/vaults", nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertHandlerTestError(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"vault_unavailable",
	)

}

func newHandlerTestRouter(service VaultService) http.Handler {
	logger := zap.NewNop().Sugar()
	handler := New(service, logger)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(
		appmiddleware.RequireAuthentication(
			&handlerTestAuthenticator{principal: handlerTestPrincipal()},
			logger,
		),
	)

	router.Post("/v1/vaults", handler.Create)
	router.Get("/v1/vaults", handler.List)
	router.Get("/v1/vaults/{vaultID}", handler.Get)
	router.Patch("/v1/vaults/{vaultID}", handler.Rename)
	router.Delete("/v1/vaults/{vaultID}", handler.Delete)

	return router

}

func newCreateVaultTestRequest(body string, contentType string) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/vaults",
		strings.NewReader(body),
	)

	request.Header.Set("Authorization", "Bearer synthetic-access-token")
	request.Header.Set("X-Request-ID", handlerTestRequestID)

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	return request

}

func handlerTestPrincipal() session.Principal {
	return session.Principal{
		UserID:    handlerTestOwnerID,
		SessionID: handlerTestSessionID,
	}
}

func assertHandlerTestError(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d", recorder.Code, wantStatus)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}

	err := json.NewDecoder(recorder.Body).Decode(&body)
	if err != nil {
		t.Fatalf("decode vault error response: %v", err)
	}

	if body.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", body.Error.Code, wantCode)
	}

	if body.Error.Message == "" {
		t.Fatal("vault error did not include a safe message")
	}

	if body.Error.RequestID == "" {
		t.Fatal("vault error did not include a request ID")
	}

}

type handlerTestAuthenticator struct {
	principal session.Principal
}

func (authenticator *handlerTestAuthenticator) AuthenticateAccessToken(
	context.Context,
	string,
) (session.Principal, error) {
	return authenticator.principal, nil
}

type handlerTestVaultService struct {
	createResult vault.Vault
	createErr    error
	listResult   []vault.Vault
	listErr      error
	getResult    vault.Vault
	getErr       error
	renameResult vault.Vault
	renameErr    error
	deleteErr    error

	createCalls int
	listCalls   int
	getCalls    int
	renameCalls int
	deleteCalls int

	lastCreateInput vault.CreateInput
	lastListOwnerID string
	lastGetOwnerID  string
	lastGetVaultID  string
	lastRenameInput vault.RenameInput
	lastDeleteInput vault.DeleteInput
}

var _ VaultService = (*handlerTestVaultService)(nil)

func (service *handlerTestVaultService) Create(
	_ context.Context,
	input vault.CreateInput,
) (vault.Vault, error) {
	service.createCalls++
	service.lastCreateInput = input

	if service.createErr != nil {
		return vault.Vault{}, service.createErr
	}

	return service.createResult, nil

}

func (service *handlerTestVaultService) List(
	_ context.Context,
	ownerID string,
) ([]vault.Vault, error) {
	service.listCalls++
	service.lastListOwnerID = ownerID

	if service.listErr != nil {
		return nil, service.listErr
	}

	return service.listResult, nil

}

func (service *handlerTestVaultService) Get(
	_ context.Context,
	ownerID string,
	vaultID string,
) (vault.Vault, error) {
	service.getCalls++
	service.lastGetOwnerID = ownerID
	service.lastGetVaultID = vaultID

	if service.getErr != nil {
		return vault.Vault{}, service.getErr
	}

	return service.getResult, nil

}

func (service *handlerTestVaultService) Rename(
	_ context.Context,
	input vault.RenameInput,
) (vault.Vault, error) {
	service.renameCalls++
	service.lastRenameInput = input

	if service.renameErr != nil {
		return vault.Vault{}, service.renameErr
	}

	return service.renameResult, nil

}

func (service *handlerTestVaultService) Delete(
	_ context.Context,
	input vault.DeleteInput,
) error {
	service.deleteCalls++
	service.lastDeleteInput = input

	return service.deleteErr

}
