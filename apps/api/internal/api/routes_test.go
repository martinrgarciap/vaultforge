package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

type errorResponseBody struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

type testDatabasePinger struct {
	err   error
	calls int
}

func (pinger *testDatabasePinger) Ping(
	_ context.Context,
) error {
	pinger.calls++

	return pinger.err
}

func newTestApplication() *Application {
	return newTestApplicationWithDatabaseError(nil)
}

func newTestApplicationWithDatabaseError(
	databaseError error,
) *Application {
	cfg := Config{
		Env:         "test",
		Addr:        ":8080",
		DatabaseURL: "postgres://test",
	}

	logger := zap.NewNop().Sugar()

	return NewApplication(
		cfg,
		logger,
		&testDatabasePinger{
			err: databaseError,
		},
		&routeTestAuthService{},
		newTestSessionService(),
		nil,
		nil,
	)
}

func TestRoutesHealthCheck(t *testing.T) {
	app := newTestApplication()
	router := app.Routes()

	request := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	contentType := recorder.Header().Get("Content-Type")

	if contentType != "application/json" {
		t.Errorf(
			"expected content type application/json, got %q",
			contentType,
		)
	}

	if header := recorder.Header().Get("X-Content-Type-Options"); header != "nosniff" {
		t.Errorf(
			"expected X-Content-Type-Options nosniff, got %q",
			header,
		)
	}

	var body struct {
		Status      string `json:"status"`
		Environment string `json:"environment"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf(
			"expected health status ok, got %q",
			body.Status,
		)
	}

	if body.Environment != "test" {
		t.Errorf(
			"expected environment test, got %q",
			body.Environment,
		)
	}
}

func TestRoutesNotFound(t *testing.T) {
	app := newTestApplication()
	router := app.Routes()

	request := httptest.NewRequest(
		http.MethodGet,
		"/does-not-exist",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNotFound,
			recorder.Code,
		)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf(
			"expected content type application/json, got %q",
			contentType,
		)
	}

	if header := recorder.Header().Get("X-Content-Type-Options"); header != "nosniff" {
		t.Errorf(
			"expected X-Content-Type-Options nosniff, got %q",
			header,
		)
	}

	var body errorResponseBody

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Error.Code != "not_found" {
		t.Errorf(
			"expected error code not_found, got %q",
			body.Error.Code,
		)
	}

	if body.Error.Message == "" {
		t.Error("expected an error message")
	}

	if body.Error.RequestID == "" {
		t.Error("expected response to include a request ID")
	}
}

func TestRoutesMethodNotAllowed(t *testing.T) {
	app := newTestApplication()
	router := app.Routes()

	request := httptest.NewRequest(
		http.MethodPost,
		"/health",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf(
			"expected content type application/json, got %q",
			contentType,
		)
	}

	if header := recorder.Header().Get("X-Content-Type-Options"); header != "nosniff" {
		t.Errorf(
			"expected X-Content-Type-Options nosniff, got %q",
			header,
		)
	}

	var body errorResponseBody

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Error.Code != "method_not_allowed" {
		t.Errorf(
			"expected error code method_not_allowed, got %q",
			body.Error.Code,
		)
	}

	if body.Error.RequestID == "" {
		t.Error("expected response to include a request ID")
	}
}

func TestRoutesLivenessReturnsOKWhenDatabaseFails(
	t *testing.T,
) {
	app := newTestApplicationWithDatabaseError(
		errors.New("database unavailable"),
	)

	router := app.Routes()

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/live",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	var body struct {
		Status      string `json:"status"`
		Environment string `json:"environment"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf(
			"failed to decode response body: %v",
			err,
		)
	}

	if body.Status != "ok" {
		t.Errorf(
			"expected health status ok, got %q",
			body.Status,
		)
	}

	if body.Environment != "test" {
		t.Errorf(
			"expected environment test, got %q",
			body.Environment,
		)
	}
}

func TestRoutesReadinessReturnsOKWhenDatabaseIsAvailable(
	t *testing.T,
) {
	app := newTestApplication()
	router := app.Routes()

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/ready",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	var body struct {
		Status      string `json:"status"`
		Environment string `json:"environment"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf(
			"failed to decode response body: %v",
			err,
		)
	}

	if body.Status != "ok" {
		t.Errorf(
			"expected health status ok, got %q",
			body.Status,
		)
	}

	if body.Environment != "test" {
		t.Errorf(
			"expected environment test, got %q",
			body.Environment,
		)
	}
}

func TestRoutesReadinessReturnsServiceUnavailableWhenDatabaseFails(
	t *testing.T,
) {
	const sensitiveErrorDetails = "database connection failed with sensitive details"

	app := newTestApplicationWithDatabaseError(
		errors.New(sensitiveErrorDetails),
	)

	router := app.Routes()

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/ready",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusServiceUnavailable,
			recorder.Code,
		)
	}

	responseBody := recorder.Body.String()

	if responseBody == "" {
		t.Fatal("expected a response body")
	}

	if strings.Contains(
		responseBody,
		sensitiveErrorDetails,
	) {
		t.Fatal("readiness response exposed database details")
	}

	var body struct {
		Status      string `json:"status"`
		Environment string `json:"environment"`
	}

	if err := json.Unmarshal(
		[]byte(responseBody),
		&body,
	); err != nil {
		t.Fatalf(
			"failed to decode response body: %v",
			err,
		)
	}

	if body.Status != "unavailable" {
		t.Errorf(
			"expected status unavailable, got %q",
			body.Status,
		)
	}

	if body.Environment != "test" {
		t.Errorf(
			"expected environment test, got %q",
			body.Environment,
		)
	}
}
