package api

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/go-chi/chi/v5"
)

type openAPIContract struct {
	document *openapi3.T
	router   routers.Router
}

type openAPIContractCase struct {
	name            string
	method          string
	path            string
	requestBody     string
	requestHeaders  http.Header
	status          int
	responseBody    string
	responseHeaders http.Header
}

func TestOpenAPIDocumentIsValid(t *testing.T) {
	contract := loadOpenAPIContract(t)

	if contract.document.Info.Title != "VaultForge API" {
		t.Fatalf("OpenAPI title = %q, want %q", contract.document.Info.Title, "VaultForge API")
	}

	if contract.document.OpenAPI != "3.0.3" {
		t.Fatalf("OpenAPI version = %q, want %q", contract.document.OpenAPI, "3.0.3")
	}
}

func TestOpenAPIContractCoversRegisteredRoutes(t *testing.T) {
	contract := loadOpenAPIContract(t)
	registered := make(map[string]struct{})

	router, ok := newTestApplication().Routes().(chi.Routes)
	if !ok {
		t.Fatal("application router does not expose Chi routes")
	}

	err := chi.Walk(router, func(
		method string,
		route string,
		_ http.Handler,
		_ ...func(http.Handler) http.Handler,
	) error {
		registered[routeContractKey(method, route)] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("walk registered routes: %v", err)
	}

	documented := documentedRouteContracts(contract.document)

	missing := routeContractDifference(registered, documented)
	unexpected := routeContractDifference(documented, registered)

	if len(missing) != 0 || len(unexpected) != 0 {
		t.Fatalf("OpenAPI route mismatch: missing=%v unexpected=%v", missing, unexpected)
	}
}

func TestOpenAPIValidatesCriticalRequestsAndResponses(t *testing.T) {
	contract := loadOpenAPIContract(t)

	const (
		accountID = "00000000-0000-0000-0000-000000009001"
		vaultID   = "00000000-0000-0000-0000-000000009002"
		itemID    = "00000000-0000-0000-0000-000000009003"
		timestamp = "2026-06-23T18:00:00Z"
	)

	authorization := make(http.Header)
	authorization.Set("Authorization", "Bearer synthetic-contract-access-token")

	jsonHeader := make(http.Header)
	jsonHeader.Set("Content-Type", "application/json")

	itemResponseHeader := jsonHeader.Clone()
	itemResponseHeader.Set("ETag", `"1"`)

	tests := []openAPIContractCase{
		{
			name:            "health",
			method:          http.MethodGet,
			path:            "/health",
			status:          http.StatusOK,
			responseBody:    `{"status":"ok","environment":"test"}`,
			responseHeaders: jsonHeader,
		},
		{
			name:        "register",
			method:      http.MethodPost,
			path:        "/v1/auth/register",
			requestBody: `{"email":"contract@example.test","password":"synthetic contract password"}`,
			requestHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
			status: http.StatusCreated,
			responseBody: `{"user":{"id":"` + accountID + `","email":"contract@example.test",` +
				`"status":"active","createdAt":"` + timestamp + `","updatedAt":"` + timestamp + `"}}`,
			responseHeaders: jsonHeader,
		},
		{
			name:        "login",
			method:      http.MethodPost,
			path:        "/v1/auth/login",
			requestBody: `{"email":"contract@example.test","password":"synthetic contract password"}`,
			requestHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
			status: http.StatusOK,
			responseBody: `{"user":{"id":"` + accountID + `","email":"contract@example.test",` +
				`"status":"active","createdAt":"` + timestamp + `","updatedAt":"` + timestamp + `"},` +
				`"tokenType":"Bearer","accessToken":"synthetic-contract-access-token",` +
				`"accessTokenExpiresAt":"2026-06-23T18:10:00Z",` +
				`"refreshTokenExpiresAt":"2026-07-23T18:00:00Z"}`,
			responseHeaders: jsonHeader,
		},
		{
			name:   "refresh",
			method: http.MethodPost,
			path:   "/v1/auth/refresh",
			requestHeaders: http.Header{
				"Cookie":       []string{"vaultforge_refresh=synthetic-refresh; vaultforge_csrf=synthetic-csrf"},
				"X-Csrf-Token": []string{"synthetic-csrf"},
			},
			status: http.StatusOK,
			responseBody: `{"tokenType":"Bearer","accessToken":"synthetic-refreshed-access-token",` +
				`"accessTokenExpiresAt":"2026-06-23T18:10:00Z",` +
				`"refreshTokenExpiresAt":"2026-07-23T18:00:00Z"}`,
			responseHeaders: jsonHeader,
		},
		{
			name:        "create vault",
			method:      http.MethodPost,
			path:        "/v1/vaults",
			requestBody: `{"name":"Synthetic Contract Vault"}`,
			requestHeaders: mergeContractHeaders(
				authorization,
				jsonHeader,
			),
			status: http.StatusCreated,
			responseBody: `{"vault":{"id":"` + vaultID + `","name":"Synthetic Contract Vault",` +
				`"createdAt":"` + timestamp + `","updatedAt":"` + timestamp + `"}}`,
			responseHeaders: jsonHeader,
		},
		{
			name:   "create item",
			method: http.MethodPost,
			path:   "/v1/vaults/" + vaultID + "/items",
			requestBody: `{"type":"secure_note","encryptedPayload":{"version":1,` +
				`"algorithm":"AES-256-GCM",` +
				`"blob":"AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHA=="}}`,
			requestHeaders: mergeContractHeaders(
				authorization,
				jsonHeader,
				http.Header{"Idempotency-Key": []string{"contract-item-create-1"}},
			),
			status: http.StatusCreated,
			responseBody: `{"item":{"id":"` + itemID + `","type":"secure_note",` +
				`"encryptedPayload":{"version":1,"algorithm":"AES-256-GCM",` +
				`"blob":"AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHA=="},` +
				`"version":1,"createdAt":"` + timestamp + `","updatedAt":"` + timestamp + `"}}`,
			responseHeaders: itemResponseHeader,
		},
		{
			name:   "update item",
			method: http.MethodPut,
			path:   "/v1/vaults/" + vaultID + "/items/" + itemID,
			requestBody: `{"type":"secure_note","encryptedPayload":{"version":1,` +
				`"algorithm":"AES-256-GCM",` +
				`"blob":"AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHA=="}}`,
			requestHeaders: mergeContractHeaders(
				authorization,
				jsonHeader,
				http.Header{"If-Match": []string{`"1"`}},
			),
			status: http.StatusOK,
			responseBody: `{"item":{"id":"` + itemID + `","type":"secure_note",` +
				`"encryptedPayload":{"version":1,"algorithm":"AES-256-GCM",` +
				`"blob":"AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHA=="},` +
				`"version":2,"createdAt":"` + timestamp + `","updatedAt":"2026-06-23T18:01:00Z"}}`,
			responseHeaders: http.Header{
				"Content-Type": []string{"application/json"},
				"ETag":         []string{`"2"`},
			},
		},
		{
			name:           "safe not found error",
			method:         http.MethodGet,
			path:           "/v1/vaults/" + vaultID,
			requestHeaders: authorization,
			status:         http.StatusNotFound,
			responseBody: `{"error":{"code":"not_found","message":"The requested resource was not found.",` +
				`"request_id":"contract-request-id"}}`,
			responseHeaders: jsonHeader,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			contract.validate(t, test)
		})
	}
}

func loadOpenAPIContract(t *testing.T) openAPIContract {
	t.Helper()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate OpenAPI contract test source")
	}

	specificationPath := filepath.Join(filepath.Dir(sourceFile), "..", "..", "openapi.yaml")

	loader := openapi3.NewLoader()

	document, err := loader.LoadFromFile(specificationPath)
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}

	if err := document.Validate(loader.Context); err != nil {
		t.Fatalf("validate OpenAPI document: %v", err)
	}

	contractRouter, err := legacy.NewRouter(document)
	if err != nil {
		t.Fatalf("create OpenAPI router: %v", err)
	}

	return openAPIContract{
		document: document,
		router:   contractRouter,
	}
}

func (contract openAPIContract) validate(t *testing.T, test openAPIContractCase) {
	t.Helper()

	request, err := http.NewRequest(
		test.method,
		"http://localhost:8080"+test.path,
		strings.NewReader(test.requestBody),
	)
	if err != nil {
		t.Fatalf("create %s contract request", test.name)
	}

	request.Header = test.requestHeaders.Clone()

	route, pathParameters, err := contract.router.FindRoute(request)
	if err != nil {
		t.Fatalf("match %s request to OpenAPI route", test.name)
	}

	options := &openapi3filter.Options{
		AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
		IncludeResponseStatus: true,
	}

	requestInput := &openapi3filter.RequestValidationInput{
		Request:    request,
		PathParams: pathParameters,
		Route:      route,
		Options:    options,
	}

	if err := openapi3filter.ValidateRequest(request.Context(), requestInput); err != nil {
		t.Fatalf(
			"%s request did not satisfy the OpenAPI contract for %s: %v",
			test.name,
			route.Operation.OperationID,
			err,
		)
	}

	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestInput,
		Status:                 test.status,
		Header:                 test.responseHeaders.Clone(),
		Body:                   io.NopCloser(bytes.NewBufferString(test.responseBody)),
		Options:                options,
	}

	if err := openapi3filter.ValidateResponse(request.Context(), responseInput); err != nil {
		t.Fatalf("%s response did not satisfy the OpenAPI contract: %v", test.name, err)
	}
}

func documentedRouteContracts(document *openapi3.T) map[string]struct{} {
	routes := make(map[string]struct{})

	for path, item := range document.Paths.Map() {
		operations := map[string]*openapi3.Operation{
			http.MethodDelete:  item.Delete,
			http.MethodGet:     item.Get,
			http.MethodHead:    item.Head,
			http.MethodOptions: item.Options,
			http.MethodPatch:   item.Patch,
			http.MethodPost:    item.Post,
			http.MethodPut:     item.Put,
			http.MethodTrace:   item.Trace,
		}

		for method, operation := range operations {
			if operation != nil {
				routes[routeContractKey(method, path)] = struct{}{}
			}
		}
	}

	return routes
}

func routeContractKey(method string, path string) string {
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		path = "/"
	}

	return strings.ToUpper(method) + " " + path
}

func routeContractDifference(left map[string]struct{}, right map[string]struct{}) []string {
	difference := make([]string, 0)

	for route := range left {
		if _, exists := right[route]; !exists {
			difference = append(difference, route)
		}
	}

	sort.Strings(difference)

	return difference
}

func mergeContractHeaders(headers ...http.Header) http.Header {
	merged := make(http.Header)

	for _, header := range headers {
		for name, values := range header {
			for _, value := range values {
				merged.Add(name, value)
			}
		}
	}

	return merged
}
