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

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessioncookie"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
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
	app := newApplicationWithAuthService(authService, zap.NewNop().Sugar())
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
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if authService.loginCalls != 1 {
		t.Fatalf("Login() calls = %d, want 1", authService.loginCalls)
	}
	if authService.lastLoginInput != (auth.LoginInput{
		Email:    "martin@example.com",
		Password: routeTestPassword,
	}) {
		t.Fatalf("Login() input = %+v", authService.lastLoginInput)
	}

	var body struct {
		User                  auth.Account `json:"user"`
		TokenType             string       `json:"tokenType"`
		AccessToken           string       `json:"accessToken"`
		AccessTokenExpiresAt  time.Time    `json:"accessTokenExpiresAt"`
		RefreshTokenExpiresAt time.Time    `json:"refreshTokenExpiresAt"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if body.User.ID != "user-123" {
		t.Fatalf("response user ID = %q, want user-123", body.User.ID)
	}
	if body.TokenType != "Bearer" || body.AccessToken == "" {
		t.Fatal("login response did not include bearer access-token data")
	}
	if body.AccessTokenExpiresAt.IsZero() || body.RefreshTokenExpiresAt.IsZero() {
		t.Fatal("login response did not include token expiration metadata")
	}

	config := sessioncookie.NewConfig(false)
	refreshCookie := routeAuthCookie(t, recorder.Result().Cookies(), config.RefreshCookieName())
	csrfCookie := routeAuthCookie(t, recorder.Result().Cookies(), config.CSRFCookieName())
	if refreshCookie.Value == "" || !refreshCookie.HttpOnly {
		t.Fatal("login did not set the HttpOnly refresh cookie")
	}
	if csrfCookie.Value == "" || csrfCookie.HttpOnly {
		t.Fatal("login did not set the browser-readable CSRF cookie")
	}
	if strings.Contains(recorder.Body.String(), refreshCookie.Value) {
		t.Fatal("login JSON exposed the refresh token")
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
	app := newApplicationWithAuthService(authService, zap.NewNop().Sugar())
	router := app.Routes()
	presentedRefreshToken, err := session.NewRefreshTokenGenerator().Generate(context.Background())
	if err != nil {
		t.Fatalf("generate presented refresh token: %v", err)
	}
	request := newRouteRefreshRequest(t, presentedRefreshToken.Value())
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		TokenType             string    `json:"tokenType"`
		AccessToken           string    `json:"accessToken"`
		AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
		RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode refresh response: %v", err)
	}
	if body.TokenType != "Bearer" || body.AccessToken == "" {
		t.Fatal("refresh response did not include bearer access-token data")
	}
	if body.AccessTokenExpiresAt.IsZero() || body.RefreshTokenExpiresAt.IsZero() {
		t.Fatal("refresh response did not include token expiration metadata")
	}

	config := sessioncookie.NewConfig(false)
	replacementCookie := routeAuthCookie(t, recorder.Result().Cookies(), config.RefreshCookieName())
	if replacementCookie.Value == "" || replacementCookie.Value == presentedRefreshToken.Value() {
		t.Fatal("refresh did not rotate the refresh cookie")
	}
	if strings.Contains(recorder.Body.String(), replacementCookie.Value) {
		t.Fatal("refresh JSON exposed the replacement refresh token")
	}
}

func TestRoutesRefreshRejectsInvalidTokenGenerically(t *testing.T) {
	t.Parallel()

	app := newApplicationWithAuthService(&routeTestAuthService{}, zap.NewNop().Sugar())
	router := app.Routes()
	request := newRouteRefreshRequest(t, "malformed-refresh-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}

	var body errorResponseBody
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error.Code != "invalid_refresh_token" {
		t.Fatalf("error code = %q, want invalid_refresh_token", body.Error.Code)
	}

	responseBody := recorder.Body.String()
	if strings.Contains(responseBody, "malformed") ||
		strings.Contains(responseBody, "replay") ||
		strings.Contains(responseBody, "revoked") ||
		strings.Contains(responseBody, "disabled") {
		t.Fatal("refresh response exposed internal token state")
	}
}

func TestRoutesRefreshRejectsMissingCSRF(t *testing.T) {
	t.Parallel()

	app := newApplicationWithAuthService(&routeTestAuthService{}, zap.NewNop().Sugar())
	router := app.Routes()
	config := sessioncookie.NewConfig(false)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{
		Name:  config.RefreshCookieName(),
		Value: "synthetic-refresh-token",
	})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	var body errorResponseBody
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error.Code != "csrf_validation_failed" {
		t.Fatalf("error code = %q, want csrf_validation_failed", body.Error.Code)
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

func newRouteRefreshRequest(t *testing.T, refreshToken string) *http.Request {
	t.Helper()

	config := sessioncookie.NewConfig(false)
	csrfToken, err := sessioncookie.NewCSRFTokenGenerator().Generate(context.Background())
	if err != nil {
		t.Fatalf("generate CSRF token: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: config.RefreshCookieName(), Value: refreshToken})
	request.AddCookie(&http.Cookie{Name: config.CSRFCookieName(), Value: csrfToken.Value()})
	request.Header.Set(config.CSRFHeaderName(), csrfToken.Value())
	return request
}

func routeAuthCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("cookie %q was not found", name)
	return nil
}

func newApplicationWithAuthService(
	authService *routeTestAuthService,
	logger *zap.SugaredLogger,
) *Application {
	return newApplicationWithAuthServiceAndSecurityEnforcer(
		authService,
		logger,
		newAllowingTestRequestLimiter(),
	)
}

func newApplicationWithAuthServiceAndSecurityEnforcer(
	authService *routeTestAuthService,
	logger *zap.SugaredLogger,
	securityEnforcer SecurityEnforcer,
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
		securityEnforcer,
		authService,
		newTestLoginSessionService(authService),
		nil,
		nil,
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

func TestRoutesAuthRequestLimitsByPeerIP(
	t *testing.T,
) {
	t.Parallel()

	testCases := []struct {
		name      string
		path      string
		wantScope string
	}{
		{
			name:      "registration",
			path:      "/v1/auth/register",
			wantScope: ratelimit.ScopeRegistration,
		},
		{
			name:      "login",
			path:      "/v1/auth/login",
			wantScope: ratelimit.ScopeLogin,
		},
		{
			name:      "refresh",
			path:      "/v1/auth/refresh",
			wantScope: ratelimit.ScopeRefresh,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			securityEnforcer := &testRequestLimiter{
				decision: ratelimit.Decision{
					Allowed:    false,
					RetryAfter: 75 * time.Second,
				},
			}

			authService := &routeTestAuthService{}

			app :=
				newApplicationWithAuthServiceAndSecurityEnforcer(
					authService,
					zap.NewNop().Sugar(),
					securityEnforcer,
				)

			request := httptest.NewRequest(
				http.MethodPost,
				testCase.path,
				nil,
			)
			request.RemoteAddr =
				"203.0.113.10:54321"

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
			) != "75" {
				t.Fatalf(
					"Retry-After = %q, want 75",
					recorder.Header().Get(
						"Retry-After",
					),
				)
			}

			if securityEnforcer.calls != 1 {
				t.Fatalf(
					"limiter calls = %d, want 1",
					securityEnforcer.calls,
				)
			}

			if securityEnforcer.lastScope !=
				testCase.wantScope {
				t.Fatalf(
					"scope = %q, want %q",
					securityEnforcer.lastScope,
					testCase.wantScope,
				)
			}

			if len(
				securityEnforcer.lastIdentity,
			) != 1 ||
				securityEnforcer.lastIdentity[0] !=
					"203.0.113.10" {
				t.Fatalf(
					"identity = %#v",
					securityEnforcer.lastIdentity,
				)
			}

			if authService.registerCalls != 0 ||
				authService.loginCalls != 0 {
				t.Fatal(
					"denied authentication request reached a service",
				)
			}
		})
	}
}

func TestRoutesAuthFailsClosedWhenRateLimitUnavailable(
	t *testing.T,
) {
	t.Parallel()

	securityEnforcer := &testRequestLimiter{
		err: ratelimit.ErrUnavailable,
	}

	authService := &routeTestAuthService{}

	app :=
		newApplicationWithAuthServiceAndSecurityEnforcer(
			authService,
			zap.NewNop().Sugar(),
			securityEnforcer,
		)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/login",
		nil,
	)
	request.RemoteAddr = "203.0.113.10:54321"

	recorder := httptest.NewRecorder()
	app.Routes().ServeHTTP(recorder, request)

	assertRouteRateLimitError(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"authentication_unavailable",
	)

	if recorder.Header().Get("Retry-After") != "" {
		t.Fatal(
			"dependency failure unexpectedly included Retry-After",
		)
	}

	if authService.loginCalls != 0 {
		t.Fatal(
			"unavailable rate-limit service reached authentication",
		)
	}
}
