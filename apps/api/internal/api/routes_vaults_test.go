package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
	"go.uber.org/zap"
)

const routeVaultTestID = "00000000-0000-0000-0000-000000000901"

func TestRoutesVaultsRequireAuthentication(t *testing.T) {
	t.Parallel()

	app := newApplicationWithVaultService(&routeTestVaultService{})
	router := app.Routes()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create vault",
			method: http.MethodPost,
			path:   "/v1/vaults",
			body:   `{"name":"Development"}`,
		},
		{
			name:   "list vaults",
			method: http.MethodGet,
			path:   "/v1/vaults",
		},
		{
			name:   "get vault",
			method: http.MethodGet,
			path:   "/v1/vaults/" + routeVaultTestID,
		},
		{
			name:   "rename vault",
			method: http.MethodPatch,
			path:   "/v1/vaults/" + routeVaultTestID,
			body:   `{"name":"Renamed Vault"}`,
		},
		{
			name:   "delete vault",
			method: http.MethodDelete,
			path:   "/v1/vaults/" + routeVaultTestID,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var body *strings.Reader
			if test.body != "" {
				body = strings.NewReader(test.body)
			} else {
				body = strings.NewReader("")
			}

			request := httptest.NewRequest(test.method, test.path, body)
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}

			if recorder.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("unauthorized response did not include a Bearer challenge")
			}
		})
	}
}

func TestRoutesVaultActionsAuthenticated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCall   string
	}{
		{
			name:       "create vault",
			method:     http.MethodPost,
			path:       "/v1/vaults",
			body:       `{"name":"Development"}`,
			wantStatus: http.StatusCreated,
			wantCall:   "create",
		},
		{
			name:       "list vaults",
			method:     http.MethodGet,
			path:       "/v1/vaults",
			wantStatus: http.StatusOK,
			wantCall:   "list",
		},
		{
			name:       "get vault",
			method:     http.MethodGet,
			path:       "/v1/vaults/" + routeVaultTestID,
			wantStatus: http.StatusOK,
			wantCall:   "get",
		},
		{
			name:       "rename vault",
			method:     http.MethodPatch,
			path:       "/v1/vaults/" + routeVaultTestID,
			body:       `{"name":"Renamed Vault"}`,
			wantStatus: http.StatusOK,
			wantCall:   "rename",
		},
		{
			name:       "delete vault",
			method:     http.MethodDelete,
			path:       "/v1/vaults/" + routeVaultTestID,
			wantStatus: http.StatusNoContent,
			wantCall:   "delete",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &routeTestVaultService{}
			app := newApplicationWithVaultService(service)
			router := app.Routes()

			request := newAuthenticatedVaultRouteRequest(
				t,
				test.method,
				test.path,
				test.body,
			)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}

			if service.callCount(test.wantCall) != 1 {
				t.Fatalf("%s calls = %d, want 1", test.wantCall, service.callCount(test.wantCall))
			}
		})
	}
}

func newApplicationWithVaultService(
	vaultService *routeTestVaultService,
) *Application {
	return newApplicationWithVaultServiceAndSecurityEnforcer(
		vaultService,
		newAllowingTestRequestLimiter(),
	)
}

func newApplicationWithVaultServiceAndSecurityEnforcer(
	vaultService *routeTestVaultService,
	securityEnforcer SecurityEnforcer,
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
		securityEnforcer,
		authService,
		newTestLoginSessionService(authService),
		nil,
		vaultService,
		nil,
	)
}

func newAuthenticatedVaultRouteRequest(
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
		t.Fatalf("issue vault route access token: %v", err)
	}

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+accessToken.Value())

	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	return request
}

func routeTestVault() vault.Vault {
	createdAt := time.Date(2026, time.June, 21, 21, 0, 0, 0, time.UTC)

	return vault.Vault{
		ID:        routeVaultTestID,
		OwnerID:   routeSessionTestUserID,
		Name:      "Development",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

type routeTestVaultService struct {
	createCalls int
	listCalls   int
	getCalls    int
	renameCalls int
	deleteCalls int
}

func (service *routeTestVaultService) Create(
	context.Context,
	vault.CreateInput,
) (vault.Vault, error) {
	service.createCalls++
	return routeTestVault(), nil
}

func (service *routeTestVaultService) List(
	context.Context,
	string,
) ([]vault.Vault, error) {
	service.listCalls++
	return []vault.Vault{routeTestVault()}, nil
}

func (service *routeTestVaultService) Get(
	context.Context,
	string,
	string,
) (vault.Vault, error) {
	service.getCalls++
	return routeTestVault(), nil
}

func (service *routeTestVaultService) Rename(
	context.Context,
	vault.RenameInput,
) (vault.Vault, error) {
	service.renameCalls++

	renamedVault := routeTestVault()
	renamedVault.Name = "Renamed Vault"

	return renamedVault, nil
}

func (service *routeTestVaultService) Delete(
	context.Context,
	vault.DeleteInput,
) error {
	service.deleteCalls++
	return nil
}

func (service *routeTestVaultService) callCount(operation string) int {
	switch operation {
	case "create":
		return service.createCalls
	case "list":
		return service.listCalls
	case "get":
		return service.getCalls
	case "rename":
		return service.renameCalls
	case "delete":
		return service.deleteCalls
	default:
		return 0
	}
}

func TestRoutesVaultMutationsAreRateLimited(
	t *testing.T,
) {
	t.Parallel()

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/v1/vaults",
			body:   `{"name":"Development"}`,
		},
		{
			name:   "rename",
			method: http.MethodPatch,
			path: "/v1/vaults/" +
				routeVaultTestID,
			body: `{"name":"Renamed Vault"}`,
		},
		{
			name:   "delete",
			method: http.MethodDelete,
			path: "/v1/vaults/" +
				routeVaultTestID,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			securityEnforcer := &testRequestLimiter{
				decision: ratelimit.Decision{
					Allowed:    false,
					RetryAfter: 40 * time.Second,
				},
			}

			service := &routeTestVaultService{}

			app :=
				newApplicationWithVaultServiceAndSecurityEnforcer(
					service,
					securityEnforcer,
				)

			request := newAuthenticatedVaultRouteRequest(
				t,
				testCase.method,
				testCase.path,
				testCase.body,
			)

			recorder := httptest.NewRecorder()
			app.Routes().ServeHTTP(
				recorder,
				request,
			)

			assertRouteRateLimitError(
				t,
				recorder,
				http.StatusTooManyRequests,
				"rate_limit_exceeded",
			)

			if recorder.Header().Get(
				"Retry-After",
			) != "40" {
				t.Fatalf(
					"Retry-After = %q, want 40",
					recorder.Header().Get(
						"Retry-After",
					),
				)
			}

			if service.createCalls != 0 ||
				service.renameCalls != 0 ||
				service.deleteCalls != 0 {
				t.Fatal(
					"denied vault mutation reached the service",
				)
			}

			if securityEnforcer.calls != 1 {
				t.Fatalf(
					"limiter calls = %d, want 1",
					securityEnforcer.calls,
				)
			}

			if securityEnforcer.lastScope !=
				ratelimit.ScopeMutation {
				t.Fatalf(
					"scope = %q, want %q",
					securityEnforcer.lastScope,
					ratelimit.ScopeMutation,
				)
			}

			if len(
				securityEnforcer.lastIdentity,
			) != 1 ||
				securityEnforcer.lastIdentity[0] !=
					routeSessionTestUserID {
				t.Fatalf(
					"identity = %#v",
					securityEnforcer.lastIdentity,
				)
			}
		})
	}
}

func TestRoutesVaultMutationFailsClosedWhenRateLimitUnavailable(
	t *testing.T,
) {
	t.Parallel()

	securityEnforcer := &testRequestLimiter{
		err: ratelimit.ErrUnavailable,
	}

	service := &routeTestVaultService{}

	app :=
		newApplicationWithVaultServiceAndSecurityEnforcer(
			service,
			securityEnforcer,
		)

	request := newAuthenticatedVaultRouteRequest(
		t,
		http.MethodPost,
		"/v1/vaults",
		`{"name":"Development"}`,
	)

	recorder := httptest.NewRecorder()
	app.Routes().ServeHTTP(recorder, request)

	assertRouteRateLimitError(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"service_unavailable",
	)

	if service.createCalls != 0 {
		t.Fatal(
			"unavailable enforcement reached the vault service",
		)
	}
}

func TestRoutesReadOnlyVaultRequestIgnoresRateLimitFailure(
	t *testing.T,
) {
	t.Parallel()

	securityEnforcer := &testRequestLimiter{
		err: ratelimit.ErrUnavailable,
	}

	service := &routeTestVaultService{}

	app :=
		newApplicationWithVaultServiceAndSecurityEnforcer(
			service,
			securityEnforcer,
		)

	request := newAuthenticatedVaultRouteRequest(
		t,
		http.MethodGet,
		"/v1/vaults",
		"",
	)

	recorder := httptest.NewRecorder()
	app.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			http.StatusOK,
			recorder.Body.String(),
		)
	}

	if service.listCalls != 1 {
		t.Fatalf(
			"List() calls = %d, want 1",
			service.listCalls,
		)
	}

	if securityEnforcer.calls != 0 {
		t.Fatalf(
			"read-only route called limiter %d times",
			securityEnforcer.calls,
		)
	}
}
