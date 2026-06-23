package authhandler

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessioncookie"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"go.uber.org/zap"
)

const handlerTestPassword = "correct horse battery staple"

type testErrorResponse struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func TestHandlerRegister(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		19,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	service := &fakeAuthService{
		registerAccount: auth.Account{
			ID:        "user-123",
			Email:     "martin@example.com",
			Status:    "active",
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	router := newTestRouter(service)

	requestBody := `{
		"email": "martin@example.com",
		"password": "correct horse battery staple"
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/register",
		strings.NewReader(requestBody),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
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

	if service.registerCalls != 1 {
		t.Fatalf(
			"Register() calls = %d, want 1",
			service.registerCalls,
		)
	}

	if service.lastRegisterInput !=
		(auth.RegisterInput{
			Email:    "martin@example.com",
			Password: handlerTestPassword,
		}) {
		t.Fatalf(
			"Register() input = %+v",
			service.lastRegisterInput,
		)
	}

	var body accountResponse

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

	responseBody := recorder.Body.String()

	if strings.Contains(
		responseBody,
		handlerTestPassword,
	) ||
		strings.Contains(
			responseBody,
			"password_hash",
		) ||
		strings.Contains(
			responseBody,
			"encoded-password-hash",
		) {
		t.Fatal(
			"registration response exposed password material",
		)
	}
}

func TestHandlerLogin(t *testing.T) {
	t.Parallel()

	loginResult := newHandlerTestLoginResult(t)
	service := &fakeAuthService{loginResult: loginResult}
	router := newTestRouter(service)
	request := newJSONRequest(
		http.MethodPost,
		"/v1/auth/login",
		`{
			"email": "martin@example.com",
			"password": "correct horse battery staple"
		}`,
	)
	request.Header.Set("User-Agent", "Thunder Client")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if service.loginCalls != 1 {
		t.Fatalf("Login() calls = %d, want 1", service.loginCalls)
	}

	expectedInput := session.LoginInput{
		Email:     "martin@example.com",
		Password:  handlerTestPassword,
		UserAgent: "Thunder Client",
	}
	if service.lastLoginInput != expectedInput {
		t.Fatalf("Login() input = %+v, want %+v", service.lastLoginInput, expectedInput)
	}

	var body loginResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.User.ID != loginResult.Account.ID {
		t.Fatalf("response user ID = %q", body.User.ID)
	}
	if body.TokenType != "Bearer" {
		t.Fatalf("token type = %q, want Bearer", body.TokenType)
	}
	if body.AccessToken != loginResult.AccessToken.Value() {
		t.Fatal("response access token did not match")
	}
	if !body.RefreshTokenExpiresAt.Equal(loginResult.RefreshTokenExpiresAt) {
		t.Fatal("response refresh-token expiration did not match")
	}

	config := sessioncookie.NewConfig(false)
	cookies := recorder.Result().Cookies()
	refreshCookie := handlerTestCookie(t, cookies, config.RefreshCookieName())
	csrfCookie := handlerTestCookie(t, cookies, config.CSRFCookieName())
	if refreshCookie.Value != loginResult.RefreshToken.Value() || !refreshCookie.HttpOnly {
		t.Fatal("login did not set the HttpOnly refresh cookie")
	}
	if csrfCookie.Value == "" || csrfCookie.HttpOnly {
		t.Fatal("login did not set a browser-readable CSRF cookie")
	}

	responseBody := recorder.Body.String()
	if strings.Contains(responseBody, handlerTestPassword) ||
		strings.Contains(responseBody, "password_hash") ||
		strings.Contains(responseBody, loginResult.RefreshToken.Value()) {
		t.Fatal("login response exposed sensitive authentication material")
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("login response did not disable caching")
	}
}

func TestHandlerRefresh(t *testing.T) {
	t.Parallel()

	loginResult := newHandlerTestLoginResult(t)
	refreshResult := session.RefreshResult{
		AccessToken:           loginResult.AccessToken,
		RefreshToken:          loginResult.RefreshToken,
		RefreshTokenExpiresAt: loginResult.RefreshTokenExpiresAt,
	}
	service := &fakeAuthService{refreshResult: refreshResult}
	router := newTestRouter(service)
	request := newHandlerRefreshRequest(t, loginResult.RefreshToken.Value())

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if service.refreshCalls != 1 {
		t.Fatalf("Refresh() calls = %d, want 1", service.refreshCalls)
	}
	if service.lastRefreshInput.RefreshToken != loginResult.RefreshToken.Value() {
		t.Fatal("refresh input did not contain the cookie token")
	}

	var body refreshResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode refresh response: %v", err)
	}
	if body.TokenType != "Bearer" {
		t.Fatalf("token type = %q, want Bearer", body.TokenType)
	}
	if body.AccessToken != refreshResult.AccessToken.Value() {
		t.Fatal("response access token did not match")
	}
	if !body.AccessTokenExpiresAt.Equal(refreshResult.AccessToken.ExpiresAt()) {
		t.Fatal("response access-token expiration did not match")
	}
	if !body.RefreshTokenExpiresAt.Equal(refreshResult.RefreshTokenExpiresAt) {
		t.Fatal("response refresh-token expiration did not match")
	}

	config := sessioncookie.NewConfig(false)
	cookies := recorder.Result().Cookies()
	refreshCookie := handlerTestCookie(t, cookies, config.RefreshCookieName())
	if refreshCookie.Value != refreshResult.RefreshToken.Value() {
		t.Fatal("refresh did not rotate the refresh cookie")
	}
	if strings.Contains(recorder.Body.String(), refreshResult.RefreshToken.Value()) {
		t.Fatal("refresh response exposed the replacement refresh token")
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("refresh response did not disable caching")
	}
}

func TestHandlerRejectsInvalidRequestBodies(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantCode    string
	}{
		{
			name:        "missing content type",
			contentType: "",
			body:        `{}`,
			wantStatus:  http.StatusUnsupportedMediaType,
			wantCode:    "unsupported_media_type",
		},
		{
			name:        "malformed JSON",
			contentType: "application/json",
			body:        `{"email":`,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
		},
		{
			name:        "unknown field",
			contentType: "application/json",
			body: `{
				"email": "martin@example.com",
				"password": "correct horse battery staple",
				"admin": true
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:        "oversized body",
			contentType: "application/json",
			body: `{
				"email": "martin@example.com",
				"password": "` +
				strings.Repeat(
					"a",
					int(maxAuthRequestBodyBytes),
				) +
				`"
			}`,
			wantStatus: http.StatusRequestEntityTooLarge,
			wantCode:   "request_body_too_large",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAuthService{}
			router := newTestRouter(service)

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/auth/register",
				strings.NewReader(test.body),
			)

			if test.contentType != "" {
				request.Header.Set(
					"Content-Type",
					test.contentType,
				)
			}

			recorder := httptest.NewRecorder()

			router.ServeHTTP(
				recorder,
				request,
			)

			assertErrorResponse(
				t,
				recorder,
				test.wantStatus,
				test.wantCode,
			)

			if service.registerCalls != 0 {
				t.Fatal(
					"Register() was called for an invalid request",
				)
			}
		})
	}
}

func TestHandlerMapsRegistrationErrors(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid email",
			serviceErr: auth.ErrEmailInvalid,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_email",
		},
		{
			name:       "invalid password",
			serviceErr: auth.ErrPasswordTooShort,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_password",
		},
		{
			name:       "email unavailable",
			serviceErr: auth.ErrEmailUnavailable,
			wantStatus: http.StatusConflict,
			wantCode:   "email_unavailable",
		},
		{
			name:       "authentication unavailable",
			serviceErr: auth.ErrAuthenticationUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "authentication_unavailable",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAuthService{
				registerErr: test.serviceErr,
			}
			router := newTestRouter(service)

			request := newJSONRequest(
				http.MethodPost,
				"/v1/auth/register",
				`{
					"email": "martin@example.com",
					"password": "correct horse battery staple"
				}`,
			)

			recorder := httptest.NewRecorder()

			router.ServeHTTP(
				recorder,
				request,
			)

			assertErrorResponse(
				t,
				recorder,
				test.wantStatus,
				test.wantCode,
			)
		})
	}
}

func TestHandlerMapsInvalidCredentialsGenerically(
	t *testing.T,
) {
	t.Parallel()

	service := &fakeAuthService{
		loginErr: auth.ErrInvalidCredentials,
	}
	router := newTestRouter(service)

	request := newJSONRequest(
		http.MethodPost,
		"/v1/auth/login",
		`{
			"email": "unknown@example.com",
			"password": "incorrect password value"
		}`,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assertErrorResponse(
		t,
		recorder,
		http.StatusUnauthorized,
		"invalid_credentials",
	)

	responseBody := recorder.Body.String()

	if strings.Contains(
		responseBody,
		"unknown@example.com",
	) ||
		strings.Contains(
			responseBody,
			"account not found",
		) ||
		strings.Contains(
			responseBody,
			"disabled",
		) {
		t.Fatal(
			"login response revealed account details",
		)
	}
}

func TestHandlerMapsAuthenticationFailureSafely(
	t *testing.T,
) {
	t.Parallel()

	service := &fakeAuthService{
		loginErr: errors.New(
			"synthetic unexpected internal detail",
		),
	}
	router := newTestRouter(service)

	request := newJSONRequest(
		http.MethodPost,
		"/v1/auth/login",
		`{
			"email": "martin@example.com",
			"password": "correct horse battery staple"
		}`,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assertErrorResponse(
		t,
		recorder,
		http.StatusInternalServerError,
		"internal_error",
	)

	if strings.Contains(
		recorder.Body.String(),
		"synthetic unexpected internal detail",
	) {
		t.Fatal(
			"handler exposed an internal error",
		)
	}
}

func TestHandlerMapsInvalidRefreshTokenGenerically(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{refreshErr: session.ErrRefreshTokenInvalid}
	router := newTestRouter(service)
	request := newHandlerRefreshRequest(t, "synthetic-invalid-refresh-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, "invalid_refresh_token")
	responseBody := recorder.Body.String()
	if strings.Contains(responseBody, "replay") ||
		strings.Contains(responseBody, "revoked") ||
		strings.Contains(responseBody, "disabled") ||
		strings.Contains(responseBody, "database") {
		t.Fatal("refresh response exposed internal token state")
	}

	config := sessioncookie.NewConfig(false)
	cookies := recorder.Result().Cookies()
	if handlerTestCookie(t, cookies, config.RefreshCookieName()).MaxAge != -1 {
		t.Fatal("invalid refresh did not clear the refresh cookie")
	}
	if handlerTestCookie(t, cookies, config.CSRFCookieName()).MaxAge != -1 {
		t.Fatal("invalid refresh did not clear the CSRF cookie")
	}
}

func TestHandlerMapsRefreshDependencyFailureSafely(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{refreshErr: session.ErrSessionUnavailable}
	router := newTestRouter(service)
	request := newHandlerRefreshRequest(t, "synthetic-refresh-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusServiceUnavailable, "authentication_unavailable")
}

func TestHandlerRefreshRejectsMissingCSRF(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{}
	router := newTestRouter(service)
	config := sessioncookie.NewConfig(false)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{
		Name:  config.RefreshCookieName(),
		Value: "synthetic-refresh-token",
	})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusForbidden, "csrf_validation_failed")
	if service.refreshCalls != 0 {
		t.Fatalf("Refresh() calls = %d, want 0", service.refreshCalls)
	}
}

func newHandlerRefreshRequest(t *testing.T, refreshToken string) *http.Request {
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

func handlerTestCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("cookie %q was not found", name)
	return nil
}

func TestHandlerRefreshRejectsNonEmptyBody(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{}
	router := newTestRouter(service)

	request := newJSONRequest(
		http.MethodPost,
		"/v1/auth/refresh",
		`{"refreshToken":"not-accepted-here"}`,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertErrorResponse(
		t,
		recorder,
		http.StatusBadRequest,
		"invalid_request",
	)

	if service.refreshCalls != 0 {
		t.Fatalf("Refresh() calls = %d, want 0", service.refreshCalls)
	}

	if len(recorder.Result().Cookies()) != 0 {
		t.Fatal("malformed refresh request unexpectedly changed browser cookies")
	}
}

func newTestRouter(
	service *fakeAuthService,
) http.Handler {
	return newTestRouterWithLoginProtector(
		service,
		&fakeLoginProtector{},
	)
}

func newTestRouterWithLoginProtector(
	service *fakeAuthService,
	loginProtector *fakeLoginProtector,
) http.Handler {
	handler := New(
		service,
		service,
		loginProtector,
		sessioncookie.NewManager(
			sessioncookie.NewConfig(false),
		),
		zap.NewNop().Sugar(),
	)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)

	router.Post(
		"/v1/auth/register",
		handler.Register,
	)
	router.Post(
		"/v1/auth/login",
		handler.Login,
	)

	router.Post(
		"/v1/auth/refresh",
		handler.Refresh,
	)

	return router
}

func newJSONRequest(
	method string,
	path string,
	body string,
) *http.Request {
	request := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	return request
}

func assertErrorResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			wantStatus,
		)
	}

	var body testErrorResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"failed to decode error response: %v",
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
			"expected a safe error message",
		)
	}

	if body.Error.RequestID == "" {
		t.Fatal(
			"expected a request ID",
		)
	}
}

func newHandlerTestLoginResult(
	t *testing.T,
) session.LoginResult {
	t.Helper()

	seed := bytes.Repeat(
		[]byte{0x62},
		ed25519.SeedSize,
	)

	tokenConfig, err := session.NewTokenConfig(
		"vaultforge-handler-test",
		"vaultforge-handler-test",
		"handler-test-ed25519-v1",
		seed,
		session.DefaultTokenLifetimes(),
	)
	if err != nil {
		t.Fatalf(
			"create handler token configuration: %v",
			err,
		)
	}

	manager, err :=
		tokenConfig.NewAccessTokenManager()
	if err != nil {
		t.Fatalf(
			"create handler token manager: %v",
			err,
		)
	}

	accessToken, err := manager.Issue(
		context.Background(),
		session.Principal{
			UserID:    "user-123",
			SessionID: "session-456",
		},
	)
	if err != nil {
		t.Fatalf(
			"issue handler access token: %v",
			err,
		)
	}

	refreshToken, err :=
		session.NewRefreshTokenGenerator().
			Generate(context.Background())
	if err != nil {
		t.Fatalf(
			"generate handler refresh token: %v",
			err,
		)
	}

	return session.LoginResult{
		Account: auth.Account{
			ID:     "user-123",
			Email:  "martin@example.com",
			Status: "active",
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		RefreshTokenExpiresAt: time.Now().
			UTC().
			Add(24 * time.Hour),
	}
}

type fakeAuthService struct {
	registerAccount auth.Account
	registerErr     error
	loginResult     session.LoginResult
	loginErr        error
	refreshResult   session.RefreshResult
	refreshErr      error

	registerCalls int
	loginCalls    int
	refreshCalls  int

	lastRegisterInput auth.RegisterInput
	lastLoginInput    session.LoginInput
	lastRefreshInput  session.RefreshInput
}

func (service *fakeAuthService) Register(
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

func (service *fakeAuthService) Login(
	_ context.Context,
	input session.LoginInput,
) (session.LoginResult, error) {
	service.loginCalls++
	service.lastLoginInput = input

	if service.loginErr != nil {
		return session.LoginResult{},
			service.loginErr
	}

	return service.loginResult, nil
}

func (service *fakeAuthService) Refresh(
	_ context.Context,
	input session.RefreshInput,
) (session.RefreshResult, error) {
	service.refreshCalls++
	service.lastRefreshInput = input

	if service.refreshErr != nil {
		return session.RefreshResult{},
			service.refreshErr
	}

	return service.refreshResult, nil
}

type fakeLoginProtector struct {
	checkDecision  ratelimit.LockoutDecision
	recordDecision ratelimit.LockoutDecision
	checkErr       error
	recordErr      error
	clearErr       error

	checkCalls         int
	recordFailureCalls int
	clearCalls         int

	lastIdentity []string
}

func (protector *fakeLoginProtector) Check(
	_ context.Context,
	identityParts ...string,
) (ratelimit.LockoutDecision, error) {
	protector.checkCalls++
	protector.lastIdentity = append(
		[]string(nil),
		identityParts...,
	)

	return protector.checkDecision,
		protector.checkErr
}

func (protector *fakeLoginProtector) RecordFailure(
	_ context.Context,
	identityParts ...string,
) (ratelimit.LockoutDecision, error) {
	protector.recordFailureCalls++
	protector.lastIdentity = append(
		[]string(nil),
		identityParts...,
	)

	return protector.recordDecision,
		protector.recordErr
}

func (protector *fakeLoginProtector) Clear(
	_ context.Context,
	identityParts ...string,
) error {
	protector.clearCalls++
	protector.lastIdentity = append(
		[]string(nil),
		identityParts...,
	)

	return protector.clearErr
}

func TestHandlerLoginRejectsActiveLockout(
	t *testing.T,
) {
	t.Parallel()

	service := &fakeAuthService{
		loginResult: newHandlerTestLoginResult(t),
	}

	protector := &fakeLoginProtector{
		checkDecision: ratelimit.LockoutDecision{
			Locked:     true,
			RetryAfter: 90 * time.Second,
		},
	}

	router := newTestRouterWithLoginProtector(
		service,
		protector,
	)

	request := newJSONRequest(
		http.MethodPost,
		"/v1/auth/login",
		`{
			"email": "martin@example.com",
			"password": "correct horse battery staple"
		}`,
	)
	request.RemoteAddr = "203.0.113.10:54321"

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertErrorResponse(
		t,
		recorder,
		http.StatusTooManyRequests,
		"login_locked",
	)

	if recorder.Header().Get("Retry-After") != "90" {
		t.Fatalf(
			"Retry-After = %q, want 90",
			recorder.Header().Get("Retry-After"),
		)
	}

	if service.loginCalls != 0 {
		t.Fatal("locked request reached the session service")
	}
}

func TestHandlerLoginRecordsThresholdFailure(
	t *testing.T,
) {
	t.Parallel()

	service := &fakeAuthService{
		loginErr: auth.ErrInvalidCredentials,
	}

	protector := &fakeLoginProtector{
		recordDecision: ratelimit.LockoutDecision{
			Locked:     true,
			Failures:   5,
			RetryAfter: 15 * time.Minute,
		},
	}

	router := newTestRouterWithLoginProtector(
		service,
		protector,
	)

	request := newJSONRequest(
		http.MethodPost,
		"/v1/auth/login",
		`{
			"email": "  Martin@Example.COM  ",
			"password": "incorrect password value"
		}`,
	)
	request.RemoteAddr = "203.0.113.10:54321"

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertErrorResponse(
		t,
		recorder,
		http.StatusTooManyRequests,
		"login_locked",
	)

	if protector.recordFailureCalls != 1 {
		t.Fatalf(
			"RecordFailure() calls = %d, want 1",
			protector.recordFailureCalls,
		)
	}

	expectedIdentity := []string{
		"martin@example.com",
		"203.0.113.10",
	}

	if len(protector.lastIdentity) != 2 ||
		protector.lastIdentity[0] != expectedIdentity[0] ||
		protector.lastIdentity[1] != expectedIdentity[1] {
		t.Fatalf(
			"identity = %#v, want %#v",
			protector.lastIdentity,
			expectedIdentity,
		)
	}
}

func TestHandlerLoginClearsProtectionAfterSuccess(
	t *testing.T,
) {
	t.Parallel()

	service := &fakeAuthService{
		loginResult: newHandlerTestLoginResult(t),
	}
	protector := &fakeLoginProtector{}

	router := newTestRouterWithLoginProtector(
		service,
		protector,
	)

	request := newJSONRequest(
		http.MethodPost,
		"/v1/auth/login",
		`{
			"email": "martin@example.com",
			"password": "correct horse battery staple"
		}`,
	)
	request.RemoteAddr = "203.0.113.10:54321"

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			http.StatusOK,
			recorder.Body.String(),
		)
	}

	if protector.clearCalls != 1 {
		t.Fatalf(
			"Clear() calls = %d, want 1",
			protector.clearCalls,
		)
	}
}

func TestHandlerLoginFailsClosedWhenProtectionUnavailable(
	t *testing.T,
) {
	t.Parallel()

	service := &fakeAuthService{
		loginResult: newHandlerTestLoginResult(t),
	}

	protector := &fakeLoginProtector{
		checkErr: ratelimit.ErrUnavailable,
	}

	router := newTestRouterWithLoginProtector(
		service,
		protector,
	)

	request := newJSONRequest(
		http.MethodPost,
		"/v1/auth/login",
		`{
			"email": "martin@example.com",
			"password": "correct horse battery staple"
		}`,
	)
	request.RemoteAddr = "203.0.113.10:54321"

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertErrorResponse(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"authentication_unavailable",
	)

	if service.loginCalls != 0 {
		t.Fatal(
			"unavailable login protection reached the session service",
		)
	}
}
