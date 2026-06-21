package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
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

	var body struct {
		User                  auth.Account `json:"user"`
		TokenType             string       `json:"tokenType"`
		AccessToken           string       `json:"accessToken"`
		AccessTokenExpiresAt  time.Time    `json:"accessTokenExpiresAt"`
		RefreshToken          string       `json:"refreshToken"`
		RefreshTokenExpiresAt time.Time    `json:"refreshTokenExpiresAt"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"failed to decode login response: %v",
			err,
		)
	}

	if body.User.ID != "user-123" {
		t.Fatalf(
			"response user ID = %q, want %q",
			body.User.ID,
			"user-123",
		)
	}

	if body.TokenType != "Bearer" {
		t.Fatalf(
			"token type = %q, want %q",
			body.TokenType,
			"Bearer",
		)
	}

	if body.AccessToken == "" {
		t.Fatal(
			"expected a non-empty access token",
		)
	}

	if body.AccessTokenExpiresAt.IsZero() {
		t.Fatal(
			"expected an access-token expiration",
		)
	}

	if body.RefreshToken == "" {
		t.Fatal(
			"expected a non-empty refresh token",
		)
	}

	if body.RefreshTokenExpiresAt.IsZero() {
		t.Fatal(
			"expected a refresh-token expiration",
		)
	}

}

func TestRoutesRefreshSession(t *testing.T) {
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

	presentedRefreshToken, err :=
		session.NewRefreshTokenGenerator().
			Generate(context.Background())
	if err != nil {
		t.Fatalf(
			"generate presented refresh token: %v",
			err,
		)
	}

	requestBody, err := json.Marshal(
		struct {
			RefreshToken string `json:"refreshToken"`
		}{
			RefreshToken: presentedRefreshToken.Value(),
		},
	)
	if err != nil {
		t.Fatalf(
			"encode refresh request: %v",
			err,
		)
	}

	request := newAuthRouteRequest(
		"/v1/auth/refresh",
		string(requestBody),
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

	var body struct {
		TokenType             string    `json:"tokenType"`
		AccessToken           string    `json:"accessToken"`
		AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
		RefreshToken          string    `json:"refreshToken"`
		RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"failed to decode refresh response: %v",
			err,
		)
	}

	if body.TokenType != "Bearer" {
		t.Fatalf(
			"token type = %q, want %q",
			body.TokenType,
			"Bearer",
		)
	}

	if body.AccessToken == "" {
		t.Fatal(
			"expected a non-empty access token",
		)
	}

	if body.AccessTokenExpiresAt.IsZero() {
		t.Fatal(
			"expected an access-token expiration",
		)
	}

	if body.RefreshToken == "" {
		t.Fatal(
			"expected a non-empty replacement refresh token",
		)
	}

	if body.RefreshToken ==
		presentedRefreshToken.Value() {
		t.Fatal(
			"replacement refresh token matched the presented token",
		)
	}

	if body.RefreshTokenExpiresAt.IsZero() {
		t.Fatal(
			"expected a refresh-token expiration",
		)
	}
}

func TestRoutesRefreshRejectsInvalidTokenGenerically(
	t *testing.T,
) {
	t.Parallel()

	app := newApplicationWithAuthService(
		&routeTestAuthService{},
		zap.NewNop().Sugar(),
	)

	router := app.Routes()

	request := newAuthRouteRequest(
		"/v1/auth/refresh",
		`{
			"refreshToken": "malformed-refresh-token"
		}`,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusUnauthorized,
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
		"invalid_refresh_token" {
		t.Fatalf(
			"error code = %q, want %q",
			body.Error.Code,
			"invalid_refresh_token",
		)
	}

	responseBody := recorder.Body.String()

	if strings.Contains(responseBody, "malformed") ||
		strings.Contains(responseBody, "replay") ||
		strings.Contains(responseBody, "revoked") ||
		strings.Contains(responseBody, "disabled") {
		t.Fatal(
			"refresh response exposed internal token state",
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
		http.StatusServiceUnavailable {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusServiceUnavailable,
		)
	}

	var responseBody errorResponseBody

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&responseBody); err != nil {
		t.Fatalf(
			"failed to decode response: %v",
			err,
		)
	}

	if responseBody.Error.Code !=
		"authentication_unavailable" {
		t.Fatalf(
			"error code = %q, want %q",
			responseBody.Error.Code,
			"authentication_unavailable",
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
		newTestLoginSessionService(
			authService,
		),
		nil,
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
