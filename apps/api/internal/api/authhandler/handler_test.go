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
	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
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

	loginResult :=
		newHandlerTestLoginResult(t)

	service := &fakeAuthService{
		loginResult: loginResult,
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

	request.Header.Set(
		"User-Agent",
		"Thunder Client",
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

	if service.loginCalls != 1 {
		t.Fatalf(
			"Login() calls = %d, want 1",
			service.loginCalls,
		)
	}

	expectedInput := session.LoginInput{
		Email:     "martin@example.com",
		Password:  handlerTestPassword,
		UserAgent: "Thunder Client",
	}

	if service.lastLoginInput != expectedInput {
		t.Fatalf(
			"Login() input = %+v, want %+v",
			service.lastLoginInput,
			expectedInput,
		)
	}

	var body loginResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"failed to decode response: %v",
			err,
		)
	}

	if body.User.ID !=
		loginResult.Account.ID {
		t.Fatalf(
			"response user ID = %q",
			body.User.ID,
		)
	}

	if body.TokenType != "Bearer" {
		t.Fatalf(
			"token type = %q, want Bearer",
			body.TokenType,
		)
	}

	if body.AccessToken !=
		loginResult.AccessToken.Value() {
		t.Fatal(
			"response access token did not match",
		)
	}

	if body.RefreshToken !=
		loginResult.RefreshToken.Value() {
		t.Fatal(
			"response refresh token did not match",
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
		) {
		t.Fatal(
			"login response exposed password material",
		)
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

func newTestRouter(
	service *fakeAuthService,
) http.Handler {
	handler := New(
		service,
		service,
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

	registerCalls int
	loginCalls    int

	lastRegisterInput auth.RegisterInput
	lastLoginInput    session.LoginInput
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
