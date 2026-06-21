package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
	"go.uber.org/zap"
)

const (
	routeItemTestVaultID = "00000000-0000-0000-0000-000000001101"
	routeItemTestID      = "00000000-0000-0000-0000-000000001102"
)

func TestRoutesItemsRequireAuthentication(t *testing.T) {
	t.Parallel()

	app := newApplicationWithItemService(
		&routeTestItemService{},
	)
	router := app.Routes()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create item",
			method: http.MethodPost,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items",
			body: `{
				"type": "secure_note",
				"payload": {}
			}`,
		},
		{
			name:   "list items",
			method: http.MethodGet,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items",
		},
		{
			name:   "get item",
			method: http.MethodGet,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID,
		},
		{
			name:   "update item",
			method: http.MethodPut,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID,
			body: `{
				"type": "secure_note",
				"payload": {}
			}`,
		},
		{
			name:   "soft delete item",
			method: http.MethodDelete,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID,
		},
		{
			name:   "restore item",
			method: http.MethodPost,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID +
				"/restore",
		},
		{
			name:   "permanently delete item",
			method: http.MethodDelete,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID +
				"/permanent",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(
				test.method,
				test.path,
				strings.NewReader(test.body),
			)

			if test.body != "" {
				request.Header.Set(
					"Content-Type",
					"application/json",
				)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					recorder.Code,
					http.StatusUnauthorized,
					recorder.Body.String(),
				)
			}

			if recorder.Header().Get(
				"WWW-Authenticate",
			) != "Bearer" {
				t.Fatal(
					"unauthorized response omitted the Bearer challenge",
				)
			}

			var body errorResponseBody

			if err := json.NewDecoder(
				recorder.Body,
			).Decode(&body); err != nil {
				t.Fatalf(
					"decode unauthorized response: %v",
					err,
				)
			}

			if body.Error.Code != "unauthorized" {
				t.Fatalf(
					"error code = %q, want unauthorized",
					body.Error.Code,
				)
			}

			if body.Error.RequestID == "" {
				t.Fatal(
					"unauthorized response omitted its request ID",
				)
			}
		})
	}
}

func TestRoutesItemActionsAuthenticated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		idempotencyKey string
		ifMatch        string
		wantStatus     int
		wantCall       string
	}{
		{
			name:   "create item",
			method: http.MethodPost,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items",
			body: `{
				"type": "secure_note",
				"payload": {
					"value": "synthetic"
				}
			}`,
			idempotencyKey: "route-item-create-request",
			wantStatus:     http.StatusCreated,
			wantCall:       "create",
		},
		{
			name:   "list items",
			method: http.MethodGet,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items",
			wantStatus: http.StatusOK,
			wantCall:   "list",
		},
		{
			name:   "get item",
			method: http.MethodGet,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID,
			wantStatus: http.StatusOK,
			wantCall:   "get",
		},
		{
			name:   "update item",
			method: http.MethodPut,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID,
			body: `{
				"type": "secure_note",
				"payload": {
					"value": "updated-synthetic"
				}
			}`,
			ifMatch:    `"1"`,
			wantStatus: http.StatusOK,
			wantCall:   "update",
		},
		{
			name:   "soft delete item",
			method: http.MethodDelete,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID,
			ifMatch:    `"1"`,
			wantStatus: http.StatusOK,
			wantCall:   "soft-delete",
		},
		{
			name:   "restore item",
			method: http.MethodPost,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID +
				"/restore",
			ifMatch:    `"1"`,
			wantStatus: http.StatusOK,
			wantCall:   "restore",
		},
		{
			name:   "permanently delete item",
			method: http.MethodDelete,
			path: "/v1/vaults/" +
				routeItemTestVaultID +
				"/items/" +
				routeItemTestID +
				"/permanent",
			ifMatch:    `"1"`,
			wantStatus: http.StatusNoContent,
			wantCall:   "permanent-delete",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &routeTestItemService{}
			app := newApplicationWithItemService(service)
			router := app.Routes()

			request := newAuthenticatedItemRouteRequest(
				t,
				test.method,
				test.path,
				test.body,
			)

			if test.idempotencyKey != "" {
				request.Header.Set(
					"Idempotency-Key",
					test.idempotencyKey,
				)
			}

			if test.ifMatch != "" {
				request.Header.Set(
					"If-Match",
					test.ifMatch,
				)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					recorder.Code,
					test.wantStatus,
					recorder.Body.String(),
				)
			}

			if service.callCount(test.wantCall) != 1 {
				t.Fatalf(
					"%s calls = %d, want 1",
					test.wantCall,
					service.callCount(test.wantCall),
				)
			}

			if service.lastOwnerID(test.wantCall) !=
				routeSessionTestUserID {
				t.Fatalf(
					"owner ID = %q, want %q",
					service.lastOwnerID(test.wantCall),
					routeSessionTestUserID,
				)
			}

			if service.lastVaultID(test.wantCall) !=
				routeItemTestVaultID {
				t.Fatalf(
					"vault ID = %q, want %q",
					service.lastVaultID(test.wantCall),
					routeItemTestVaultID,
				)
			}

			if test.wantCall != "create" &&
				test.wantCall != "list" &&
				service.lastItemID(test.wantCall) !=
					routeItemTestID {
				t.Fatalf(
					"item ID = %q, want %q",
					service.lastItemID(test.wantCall),
					routeItemTestID,
				)
			}
		})
	}
}

func TestRoutesItemsRejectDoubledVaultPath(t *testing.T) {
	t.Parallel()

	service := &routeTestItemService{}
	app := newApplicationWithItemService(service)
	router := app.Routes()

	request := newAuthenticatedItemRouteRequest(
		t,
		http.MethodGet,
		"/v1/vaults/vaults/"+
			routeItemTestVaultID+
			"/items/"+
			routeItemTestID,
		"",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNotFound,
		)
	}

	if service.totalCalls() != 0 {
		t.Fatal(
			"item service was called for the doubled vault path",
		)
	}
}

func newApplicationWithItemService(
	itemService *routeTestItemService,
) *Application {
	authService := &routeTestAuthService{}

	return NewApplication(
		Config{
			Env:         "test",
			Addr:        ":8080",
			DatabaseURL: "postgres://test",
		},
		zap.NewNop().Sugar(),
		&testDatabasePinger{},
		authService,
		newTestLoginSessionService(authService),
		nil,
		itemService,
	)
}

func newAuthenticatedItemRouteRequest(
	t *testing.T,
	method string,
	path string,
	body string,
) *http.Request {
	t.Helper()

	accessToken, err := newTestAccessTokenManager().Issue(
		context.Background(),
		session.Principal{
			UserID:    routeSessionTestUserID,
			SessionID: routeSessionTestCurrentID,
		},
	)
	if err != nil {
		t.Fatalf(
			"issue item route access token: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	request.Header.Set(
		"Authorization",
		"Bearer "+accessToken.Value(),
	)

	if body != "" {
		request.Header.Set(
			"Content-Type",
			"application/json",
		)
	}

	return request
}

type routeTestItemService struct {
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

func (service *routeTestItemService) CreateItem(
	_ context.Context,
	input vault.CreateItemInput,
) (vault.Item, error) {
	service.createCalls++
	service.lastCreateInput = input

	return routeTestItem(1, false), nil
}

func (service *routeTestItemService) ListItems(
	_ context.Context,
	input vault.ListItemsInput,
) (vault.ItemPage, error) {
	service.listCalls++
	service.lastListInput = input

	return vault.ItemPage{
		Items: []vault.Item{
			routeTestItem(1, false),
		},
	}, nil
}

func (service *routeTestItemService) GetItem(
	_ context.Context,
	input vault.GetItemInput,
) (vault.Item, error) {
	service.getCalls++
	service.lastGetInput = input

	return routeTestItem(1, false), nil
}

func (service *routeTestItemService) UpdateItem(
	_ context.Context,
	input vault.UpdateItemInput,
) (vault.Item, error) {
	service.updateCalls++
	service.lastUpdateInput = input

	return routeTestItem(
		input.ExpectedVersion+1,
		false,
	), nil
}

func (service *routeTestItemService) SoftDeleteItem(
	_ context.Context,
	input vault.SoftDeleteItemInput,
) (vault.Item, error) {
	service.softDeleteCalls++
	service.lastSoftDeleteInput = input

	return routeTestItem(
		input.ExpectedVersion,
		true,
	), nil
}

func (service *routeTestItemService) RestoreItem(
	_ context.Context,
	input vault.RestoreItemInput,
) (vault.Item, error) {
	service.restoreCalls++
	service.lastRestoreInput = input

	return routeTestItem(
		input.ExpectedVersion+1,
		false,
	), nil
}

func (service *routeTestItemService) PermanentDeleteItem(
	_ context.Context,
	input vault.PermanentDeleteItemInput,
) error {
	service.permanentDeleteCalls++
	service.lastPermanentDeleteInput = input

	return nil
}

func routeTestItem(
	version int,
	deleted bool,
) vault.Item {
	createdAt := time.Date(
		2026,
		time.June,
		24,
		12,
		0,
		0,
		123456000,
		time.UTC,
	)

	updatedAt := createdAt.Add(time.Minute)

	item := vault.Item{
		ID:      routeItemTestID,
		VaultID: routeItemTestVaultID,
		Type:    vault.ItemTypeSecureNote,
		Payload: json.RawMessage(
			`{"value":"synthetic"}`,
		),
		Version:   version,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if deleted {
		item.DeletedAt = &updatedAt
	}

	return item
}

func (service *routeTestItemService) callCount(
	operation string,
) int {
	switch operation {
	case "create":
		return service.createCalls
	case "list":
		return service.listCalls
	case "get":
		return service.getCalls
	case "update":
		return service.updateCalls
	case "soft-delete":
		return service.softDeleteCalls
	case "restore":
		return service.restoreCalls
	case "permanent-delete":
		return service.permanentDeleteCalls
	default:
		return 0
	}
}

func (service *routeTestItemService) totalCalls() int {
	return service.createCalls +
		service.listCalls +
		service.getCalls +
		service.updateCalls +
		service.softDeleteCalls +
		service.restoreCalls +
		service.permanentDeleteCalls
}

func (service *routeTestItemService) lastOwnerID(
	operation string,
) string {
	switch operation {
	case "create":
		return service.lastCreateInput.OwnerID
	case "list":
		return service.lastListInput.OwnerID
	case "get":
		return service.lastGetInput.OwnerID
	case "update":
		return service.lastUpdateInput.OwnerID
	case "soft-delete":
		return service.lastSoftDeleteInput.OwnerID
	case "restore":
		return service.lastRestoreInput.OwnerID
	case "permanent-delete":
		return service.lastPermanentDeleteInput.OwnerID
	default:
		return ""
	}
}

func (service *routeTestItemService) lastVaultID(
	operation string,
) string {
	switch operation {
	case "create":
		return service.lastCreateInput.VaultID
	case "list":
		return service.lastListInput.VaultID
	case "get":
		return service.lastGetInput.VaultID
	case "update":
		return service.lastUpdateInput.VaultID
	case "soft-delete":
		return service.lastSoftDeleteInput.VaultID
	case "restore":
		return service.lastRestoreInput.VaultID
	case "permanent-delete":
		return service.lastPermanentDeleteInput.VaultID
	default:
		return ""
	}
}

func (service *routeTestItemService) lastItemID(
	operation string,
) string {
	switch operation {
	case "get":
		return service.lastGetInput.ItemID
	case "update":
		return service.lastUpdateInput.ItemID
	case "soft-delete":
		return service.lastSoftDeleteInput.ItemID
	case "restore":
		return service.lastRestoreInput.ItemID
	case "permanent-delete":
		return service.lastPermanentDeleteInput.ItemID
	default:
		return ""
	}
}
