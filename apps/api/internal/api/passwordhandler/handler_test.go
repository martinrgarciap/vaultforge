package passwordhandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/passwordclient"
	"go.uber.org/zap"
)

type handlerTestPasswordService struct {
	generateResult passwordclient.GenerateResult
	generateErr    error
	generateCalls  int
	lastGenerate   passwordclient.GenerateInput

	strengthResult passwordclient.CheckStrengthResult
	strengthErr    error
	strengthCalls  int
	lastStrength   passwordclient.CheckStrengthInput
}

func (service *handlerTestPasswordService) Generate(
	_ context.Context,
	input passwordclient.GenerateInput,
) (passwordclient.GenerateResult, error) {
	service.generateCalls++
	service.lastGenerate = input

	if service.generateErr != nil {
		return passwordclient.GenerateResult{}, service.generateErr
	}

	return service.generateResult, nil
}

func (service *handlerTestPasswordService) CheckStrength(
	_ context.Context,
	input passwordclient.CheckStrengthInput,
) (passwordclient.CheckStrengthResult, error) {
	service.strengthCalls++
	service.lastStrength = input

	if service.strengthErr != nil {
		return passwordclient.CheckStrengthResult{}, service.strengthErr
	}

	return service.strengthResult, nil
}

func TestGenerateReturnsPasswordResult(t *testing.T) {
	t.Parallel()

	service := &handlerTestPasswordService{
		generateResult: passwordclient.GenerateResult{
			Password:    "A1!synthetic-demo",
			EntropyBits: 96.5,
		},
	}

	router := newHandlerTestRouter(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/passwords/generate",
		strings.NewReader(`{
			"length": 18,
			"includeUppercase": true,
			"includeLowercase": true,
			"includeDigits": true,
			"includeSymbols": true,
			"excludeChars": "O0l1"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("generate response did not disable caching")
	}

	if service.generateCalls != 1 {
		t.Fatalf("Generate() calls = %d, want 1", service.generateCalls)
	}

	if service.lastGenerate.Length != 18 {
		t.Fatalf("length = %d, want 18", service.lastGenerate.Length)
	}

	if !service.lastGenerate.IncludeUppercase ||
		!service.lastGenerate.IncludeLowercase ||
		!service.lastGenerate.IncludeDigits ||
		!service.lastGenerate.IncludeSymbols {
		t.Fatalf("character class options were not forwarded correctly: %+v", service.lastGenerate)
	}

	if service.lastGenerate.ExcludeChars != "O0l1" {
		t.Fatalf("exclude chars = %q, want O0l1", service.lastGenerate.ExcludeChars)
	}

	var body generateResponse

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode generate response: %v", err)
	}

	if body.Password != "A1!synthetic-demo" {
		t.Fatalf("password = %q, want generated password", body.Password)
	}

	if body.EntropyBits != 96.5 {
		t.Fatalf("entropy bits = %v, want 96.5", body.EntropyBits)
	}
}

func TestCheckStrengthReturnsRatingWithoutEchoingPassword(t *testing.T) {
	t.Parallel()

	service := &handlerTestPasswordService{
		strengthResult: passwordclient.CheckStrengthResult{
			Score:             4,
			Label:             "very strong",
			EntropyBits:       110.25,
			CrackTimeEstimate: "centuries",
			Suggestions: []string{
				"Use a password manager.",
			},
		},
	}

	router := newHandlerTestRouter(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/passwords/strength",
		strings.NewReader(`{
			"password": "synthetic-only-password"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("strength response did not disable caching")
	}

	if service.strengthCalls != 1 {
		t.Fatalf("CheckStrength() calls = %d, want 1", service.strengthCalls)
	}

	if service.lastStrength.Password != "synthetic-only-password" {
		t.Fatal("strength request password was not forwarded")
	}

	responseBody := recorder.Body.String()

	if strings.Contains(responseBody, "synthetic-only-password") {
		t.Fatal("strength response echoed the submitted password")
	}

	var body strengthResponse

	if err := json.NewDecoder(strings.NewReader(responseBody)).Decode(&body); err != nil {
		t.Fatalf("decode strength response: %v", err)
	}

	if body.Score != 4 {
		t.Fatalf("score = %d, want 4", body.Score)
	}

	if body.Label != "very strong" {
		t.Fatalf("label = %q, want very strong", body.Label)
	}

	if body.EntropyBits != 110.25 {
		t.Fatalf("entropy bits = %v, want 110.25", body.EntropyBits)
	}

	if body.CrackTimeEstimate != "centuries" {
		t.Fatalf("crack time estimate = %q, want centuries", body.CrackTimeEstimate)
	}

	if len(body.Suggestions) != 1 ||
		body.Suggestions[0] != "Use a password manager." {
		t.Fatalf("suggestions = %#v", body.Suggestions)
	}
}

func TestHandlerRejectsInvalidRequestBodies(t *testing.T) {
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
			body:        `{"length":`,
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_request",
		},
		{
			name:        "unknown field",
			contentType: "application/json",
			body: `{
				"length": 18,
				"includeUppercase": true,
				"includeLowercase": true,
				"includeDigits": true,
				"includeSymbols": true,
				"admin": true
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			router := newHandlerTestRouter(&handlerTestPasswordService{})

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/passwords/generate",
				strings.NewReader(test.body),
			)
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}

			var responseBody errorResponseBody

			if err := json.NewDecoder(recorder.Body).Decode(&responseBody); err != nil {
				t.Fatalf("decode error response: %v", err)
			}

			if responseBody.Error.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", responseBody.Error.Code, test.wantCode)
			}
		})
	}
}

func TestHandlerMapsPasswordServiceErrorsSafely(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid password request",
			serviceErr: passwordclient.ErrPasswordRequestInvalid,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_password_request",
		},
		{
			name:       "password service unavailable",
			serviceErr: passwordclient.ErrPasswordServiceUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "password_tools_unavailable",
		},
		{
			name:       "invalid password service response",
			serviceErr: passwordclient.ErrPasswordResponseInvalid,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "password_tools_unavailable",
		},
		{
			name:       "unexpected error",
			serviceErr: errors.New("sensitive internal password-service detail"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &handlerTestPasswordService{
				generateErr: test.serviceErr,
			}

			router := newHandlerTestRouter(service)

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/passwords/generate",
				strings.NewReader(`{
					"length": 18,
					"includeUppercase": true,
					"includeLowercase": true,
					"includeDigits": true,
					"includeSymbols": true
				}`),
			)
			request.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}

			responseBody := recorder.Body.String()

			if strings.Contains(responseBody, "sensitive internal password-service detail") {
				t.Fatal("error response exposed sensitive service details")
			}

			var body errorResponseBody

			if err := json.NewDecoder(strings.NewReader(responseBody)).Decode(&body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}

			if body.Error.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", body.Error.Code, test.wantCode)
			}

			if body.Error.RequestID == "" {
				t.Fatal("error response did not include a request ID")
			}
		})
	}
}

func newHandlerTestRouter(
	service Service,
) http.Handler {
	handler := New(
		service,
		zap.NewNop().Sugar(),
	)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)

	router.Post("/v1/passwords/generate", handler.Generate)
	router.Post("/v1/passwords/strength", handler.CheckStrength)

	return router
}

type errorResponseBody struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}
