package itemhandler

import (
	"context"
	"encoding/json"
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
	itemHandlerTestOwnerID       = "00000000-0000-0000-0000-000000002101"
	itemHandlerTestSessionID     = "00000000-0000-0000-0000-000000002102"
	itemHandlerCreateTestVaultID = "00000000-0000-0000-0000-000000002103"
	itemHandlerCreateTestItemID  = "00000000-0000-0000-0000-000000002104"
	itemHandlerTestRequestID     = "item-handler-create-request"
)

type itemHandlerTestService struct {
	createResult       vault.Item
	createErr          error
	listResult         vault.ItemPage
	listErr            error
	getResult          vault.Item
	getErr             error
	updateResult       vault.Item
	updateErr          error
	softDeleteResult   vault.Item
	softDeleteErr      error
	restoreResult      vault.Item
	restoreErr         error
	permanentDeleteErr error

	createCalls          int
	listCalls            int
	getCalls             int
	updateCalls          int
	softDeleteCalls      int
	restoreCalls         int
	permanentDeleteCalls int

	lastCreateInput          vault.CreateItemInput
	lastListInput            vault.ListItemsInput
	lastGetInput             vault.GetItemInput
	lastUpdateInput          vault.UpdateItemInput
	lastSoftDeleteInput      vault.SoftDeleteItemInput
	lastRestoreInput         vault.RestoreItemInput
	lastPermanentDeleteInput vault.PermanentDeleteItemInput
}

var _ ItemService = (*itemHandlerTestService)(nil)

func (service *itemHandlerTestService) CreateItem(
	_ context.Context,
	input vault.CreateItemInput,
) (vault.Item, error) {
	service.createCalls++
	service.lastCreateInput = input

	if service.createErr != nil {
		return vault.Item{}, service.createErr
	}

	return service.createResult, nil
}

func (service *itemHandlerTestService) ListItems(
	_ context.Context,
	input vault.ListItemsInput,
) (vault.ItemPage, error) {
	service.listCalls++
	service.lastListInput = input

	if service.listErr != nil {
		return vault.ItemPage{}, service.listErr
	}

	return service.listResult, nil
}

func (service *itemHandlerTestService) GetItem(
	_ context.Context,
	input vault.GetItemInput,
) (vault.Item, error) {
	service.getCalls++
	service.lastGetInput = input

	if service.getErr != nil {
		return vault.Item{}, service.getErr
	}

	return service.getResult, nil
}

func (service *itemHandlerTestService) UpdateItem(
	_ context.Context,
	input vault.UpdateItemInput,
) (vault.Item, error) {
	service.updateCalls++
	service.lastUpdateInput = input

	if service.updateErr != nil {
		return vault.Item{}, service.updateErr
	}

	return service.updateResult, nil
}

func (service *itemHandlerTestService) SoftDeleteItem(
	_ context.Context,
	input vault.SoftDeleteItemInput,
) (vault.Item, error) {
	service.softDeleteCalls++
	service.lastSoftDeleteInput = input

	if service.softDeleteErr != nil {
		return vault.Item{}, service.softDeleteErr
	}

	return service.softDeleteResult, nil
}

func (service *itemHandlerTestService) RestoreItem(
	_ context.Context,
	input vault.RestoreItemInput,
) (vault.Item, error) {
	service.restoreCalls++
	service.lastRestoreInput = input

	if service.restoreErr != nil {
		return vault.Item{}, service.restoreErr
	}

	return service.restoreResult, nil
}

func (service *itemHandlerTestService) PermanentDeleteItem(
	_ context.Context,
	input vault.PermanentDeleteItemInput,
) error {
	service.permanentDeleteCalls++
	service.lastPermanentDeleteInput = input

	return service.permanentDeleteErr
}

type itemHandlerTestAuthenticator struct {
	principal session.Principal
}

func (authenticator *itemHandlerTestAuthenticator) AuthenticateAccessToken(
	context.Context,
	string,
) (session.Principal, error) {
	return authenticator.principal, nil
}

func newItemHandlerTestRouter(
	service ItemService,
) http.Handler {
	logger := zap.NewNop().Sugar()
	handler := New(service, logger)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(
		appmiddleware.RequireAuthentication(
			&itemHandlerTestAuthenticator{
				principal: session.Principal{
					UserID:    itemHandlerTestOwnerID,
					SessionID: itemHandlerTestSessionID,
				},
			},
			logger,
		),
	)

	router.Post(
		"/v1/vaults/{vaultID}/items",
		handler.Create,
	)

	router.Get(
		"/v1/vaults/{vaultID}/items",
		handler.List,
	)

	router.Get(
		"/v1/vaults/{vaultID}/items/{itemID}",
		handler.Get,
	)

	router.Put(
		"/v1/vaults/{vaultID}/items/{itemID}",
		handler.Update,
	)

	router.Delete(
		"/v1/vaults/{vaultID}/items/{itemID}",
		handler.SoftDelete,
	)

	router.Post(
		"/v1/vaults/{vaultID}/items/{itemID}/restore",
		handler.Restore,
	)

	router.Delete(
		"/v1/vaults/{vaultID}/items/{itemID}/permanent",
		handler.PermanentDelete,
	)

	return router
}

func newCreateItemTestRequest(
	body string,
	contentType string,
	idempotencyKey string,
) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/vaults/"+
			itemHandlerCreateTestVaultID+
			"/items",
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

	if idempotencyKey != "" {
		request.Header.Set(
			"Idempotency-Key",
			idempotencyKey,
		)
	}

	return request
}

func assertItemHandlerTestError(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			wantStatus,
			recorder.Body.String(),
		)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode item error response: %v",
			err,
		)
	}

	if body.Error.Code != wantCode {
		t.Fatalf(
			"error code = %q, want %q",
			body.Error.Code,
			wantCode,
		)
	}

	if body.Error.Message == "" {
		t.Fatal(
			"item error response omitted its safe message",
		)
	}

	if body.Error.RequestID == "" {
		t.Fatal(
			"item error response omitted its request ID",
		)
	}
}

func itemHandlerCreateResult() vault.Item {
	createdAt := time.Date(
		2026,
		time.June,
		23,
		20,
		0,
		0,
		123456000,
		time.UTC,
	)

	return vault.Item{
		ID:      itemHandlerCreateTestItemID,
		VaultID: itemHandlerCreateTestVaultID,
		Type:    vault.ItemTypeAPIKey,
		Payload: json.RawMessage(
			`{"label":"Development","token":"synthetic-token"}`,
		),
		Version:   1,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}
