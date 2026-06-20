package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const routeTestPassword = "correct horse battery staple"

func TestRoutesRegisterAccount(t *testing.T) {
	t.Parallel()

	authService := &routeTestAuthService{
		registerAccount: auth.Account{
			ID:     "user-123",
			Email:  "martin@example.com",
			Status: "active",
		},
	}

	app := newApplicationWithAuthService(
		authService,
		zap.NewNop().Sugar(),
	)
	router := app.Routes()

	request := newAuthRouteRequest(
		"/v1/auth/register",
		`{
			"email": "martin@example.com",
			"password": "correct horse battery staple"
		}`,
	)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusCreated,
		)
	}

	if authService.registerCalls != 1 {
		t.Fatalf(
			"Register() calls = %d, want 1",
			authService.registerCalls,
		)
	}

	if authService.lastRegisterInput !=
		(auth.RegisterInput{
			Email:    "martin@example.com",
			Password: routeTestPassword,
		}) {
		t.Fatalf(
			"Register() input = %+v",
			authService.lastRegisterInput,
		)
	}

	var body struct {
		User auth.Account `json:"user"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"failed to decode response: %v",
			err,
		)
	}

	if body.User.ID != "user-123" ||
		body.User.Email != "martin@example.com" {
		t.Fatalf(
			"response user = %+v",
			body.User,
		)
	}
}

func TestRoutesLoginAccount(t *testing.T) {
	t.Parallel()

	authService := &routeTestAuthService{
		loginAccount: auth.Account{
			ID:     "user-123",
			Email:  "martin@example.com",
			Status: "active",
		},
	}

	app := newApplicationWithAuthService(
		authService,
		zap.NewNop().Sugar(),
	)
	router := app.Routes()

	request := newAuthRouteRequest(
		"/v1/auth/login",
		`{
			"email": "martin@example.com",
			"password": "correct horse battery staple"
		}`,
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

	if authService.loginCalls != 1 {
		t.Fatalf(
			"Login() calls = %d, want 1",
			authService.loginCalls,
		)
	}

	if authService.lastLoginInput !=
		(auth.LoginInput{
			Email:    "martin@example.com",
			Password: routeTestPassword,
		}) {
		t.Fatalf(
			"Login() input = %+v",
			authService.lastLoginInput,
		)
	}
}

func TestRoutesAuthRejectsUnsupportedMethod(
	t *testing.T,
) {
	t.Parallel()

	app := newApplicationWithAuthService(
		&routeTestAuthService{},
		zap.NewNop().Sugar(),
	)
	router := app.Routes()

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/auth/login",
		nil,
	)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code !=
		http.StatusMethodNotAllowed {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusMethodNotAllowed,
		)
	}

	var body errorResponseBody

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"failed to decode response: %v",
			err,
		)
	}

	if body.Error.Code !=
		"method_not_allowed" {
		t.Fatalf(
			"error code = %q, want %q",
			body.Error.Code,
			"method_not_allowed",
		)
	}

	if body.Error.RequestID == "" {
		t.Fatal(
			"expected request ID",
		)
	}
}

func TestRoutesAuthDoesNotLogSensitiveValues(
	t *testing.T,
) {
	const (
		passwordMarker = "synthetic-password-marker-12345"
		internalMarker = "synthetic-internal-auth-marker-67890"
	)

	core, observedLogs := observer.New(
		zap.DebugLevel,
	)
	logger := zap.New(core).Sugar()

	authService := &routeTestAuthService{
		loginErr: errors.New(
			internalMarker,
		),
	}

	app := newApplicationWithAuthService(
		authService,
		logger,
	)
	router := app.Routes()

	request := newAuthRouteRequest(
		"/v1/auth/login",
		`{
			"email": "martin@example.com",
			"password": "`+
			passwordMarker+
			`"
		}`,
	)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code !=
		http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusInternalServerError,
		)
	}

	for _, entry := range observedLogs.All() {
		logEntry := struct {
			Message string         `json:"message"`
			Context map[string]any `json:"context"`
		}{
			Message: entry.Message,
			Context: entry.ContextMap(),
		}

		encodedEntry, err := json.Marshal(
			logEntry,
		)
		if err != nil {
			t.Fatalf(
				"failed to encode observed log: %v",
				err,
			)
		}

		logText := string(encodedEntry)

		if strings.Contains(
			logText,
			passwordMarker,
		) {
			t.Fatal(
				"authentication logs exposed a password",
			)
		}

		if strings.Contains(
			logText,
			internalMarker,
		) {
			t.Fatal(
				"authentication logs exposed internal error details",
			)
		}
	}
}

func newApplicationWithAuthService(
	authService *routeTestAuthService,
	logger *zap.SugaredLogger,
) *Application {
	cfg := Config{
		Env:         "test",
		Addr:        ":8080",
		DatabaseURL: "postgres://test",
	}

	return NewApplication(
		cfg,
		logger,
		&testDatabasePinger{},
		authService,
	)
}

func newAuthRouteRequest(
	path string,
	body string,
) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost,
		path,
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	return request
}

type routeTestAuthService struct {
	registerAccount auth.Account
	registerErr     error
	loginAccount    auth.Account
	loginErr        error

	registerCalls int
	loginCalls    int

	lastRegisterInput auth.RegisterInput
	lastLoginInput    auth.LoginInput
}

func (service *routeTestAuthService) Register(
	_ context.Context,
	input auth.RegisterInput,
) (auth.Account, error) {
	service.registerCalls++
	service.lastRegisterInput = input

	if service.registerErr != nil {
		return auth.Account{}, service.registerErr
	}

	return service.registerAccount, nil
}

func (service *routeTestAuthService) Login(
	_ context.Context,
	input auth.LoginInput,
) (auth.Account, error) {
	service.loginCalls++
	service.lastLoginInput = input

	if service.loginErr != nil {
		return auth.Account{}, service.loginErr
	}

	return service.loginAccount, nil
}
