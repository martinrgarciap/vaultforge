package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/passwordclient"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
	"go.uber.org/zap"
)

type routeTestPasswordService struct {
	generateResult passwordclient.GenerateResult
	generateCalls  int

	strengthResult passwordclient.CheckStrengthResult
	strengthCalls  int
}

func (service *routeTestPasswordService) Generate(
	_ context.Context,
	_ passwordclient.GenerateInput,
) (passwordclient.GenerateResult, error) {
	service.generateCalls++

	return service.generateResult, nil
}

func (service *routeTestPasswordService) CheckStrength(
	_ context.Context,
	_ passwordclient.CheckStrengthInput,
) (passwordclient.CheckStrengthResult, error) {
	service.strengthCalls++

	return service.strengthResult, nil
}

func TestPasswordGenerateRouteIsPublicAndRateLimited(t *testing.T) {
	t.Parallel()

	limiter := newAllowingTestRequestLimiter()

	service := &routeTestPasswordService{
		generateResult: passwordclient.GenerateResult{
			Password:    "A1!synthetic-demo",
			EntropyBits: 96.5,
		},
	}

	app := newPasswordRouteTestApplication(
		limiter,
		service,
	)

	router := app.Routes()

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

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if service.generateCalls != 1 {
		t.Fatalf("Generate() calls = %d, want 1", service.generateCalls)
	}

	if limiter.calls != 1 {
		t.Fatalf("rate-limit calls = %d, want 1", limiter.calls)
	}

	if limiter.lastScope != ratelimit.ScopePasswordTools {
		t.Fatalf("rate-limit scope = %q, want %q", limiter.lastScope, ratelimit.ScopePasswordTools)
	}

	if len(limiter.lastIdentity) != 1 ||
		limiter.lastIdentity[0] != "192.0.2.1" {
		t.Fatalf("rate-limit identity = %#v, want peer IP", limiter.lastIdentity)
	}

	var body struct {
		Password    string  `json:"password"`
		EntropyBits float64 `json:"entropyBits"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode generate response: %v", err)
	}

	if body.Password != "A1!synthetic-demo" {
		t.Fatalf("password = %q, want generated password", body.Password)
	}
}

func TestPasswordStrengthRouteIsPublicAndRateLimited(t *testing.T) {
	t.Parallel()

	limiter := newAllowingTestRequestLimiter()

	service := &routeTestPasswordService{
		strengthResult: passwordclient.CheckStrengthResult{
			Score:             3,
			Label:             "strong",
			EntropyBits:       85.25,
			CrackTimeEstimate: "years",
			Suggestions: []string{
				"Use more words.",
			},
		},
	}

	app := newPasswordRouteTestApplication(
		limiter,
		service,
	)

	router := app.Routes()

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

	if service.strengthCalls != 1 {
		t.Fatalf("CheckStrength() calls = %d, want 1", service.strengthCalls)
	}

	if limiter.calls != 1 {
		t.Fatalf("rate-limit calls = %d, want 1", limiter.calls)
	}

	if limiter.lastScope != ratelimit.ScopePasswordTools {
		t.Fatalf("rate-limit scope = %q, want %q", limiter.lastScope, ratelimit.ScopePasswordTools)
	}

	responseBody := recorder.Body.String()

	if strings.Contains(responseBody, "synthetic-only-password") {
		t.Fatal("strength route echoed the submitted password")
	}

	var body struct {
		Score             uint32   `json:"score"`
		Label             string   `json:"label"`
		EntropyBits       float64  `json:"entropyBits"`
		CrackTimeEstimate string   `json:"crackTimeEstimate"`
		Suggestions       []string `json:"suggestions"`
	}

	if err := json.NewDecoder(strings.NewReader(responseBody)).Decode(&body); err != nil {
		t.Fatalf("decode strength response: %v", err)
	}

	if body.Score != 3 {
		t.Fatalf("score = %d, want 3", body.Score)
	}

	if body.Label != "strong" {
		t.Fatalf("label = %q, want strong", body.Label)
	}
}

func TestPasswordRoutesReturnRateLimitResponse(t *testing.T) {
	t.Parallel()

	limiter := &testRequestLimiter{
		decision: ratelimit.Decision{
			Allowed:    false,
			RetryAfter: 30 * time.Second,
		},
	}

	service := &routeTestPasswordService{
		generateResult: passwordclient.GenerateResult{
			Password:    "A1!synthetic-demo",
			EntropyBits: 96.5,
		},
	}

	app := newPasswordRouteTestApplication(
		limiter,
		service,
	)

	router := app.Routes()

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

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}

	if recorder.Header().Get("Retry-After") != "30" {
		t.Fatalf("Retry-After = %q, want 30", recorder.Header().Get("Retry-After"))
	}

	if service.generateCalls != 0 {
		t.Fatalf("Generate() calls = %d, want 0 after rate limit denial", service.generateCalls)
	}

	var body errorResponseBody

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode rate-limit response: %v", err)
	}

	if body.Error.Code != "rate_limit_exceeded" {
		t.Fatalf("error code = %q, want rate_limit_exceeded", body.Error.Code)
	}
}

func newPasswordRouteTestApplication(
	limiter *testRequestLimiter,
	passwordService *routeTestPasswordService,
) *Application {
	cfg := Config{
		Env:         "test",
		Addr:        ":8080",
		DatabaseURL: "postgres://test",
	}

	return NewApplication(
		cfg,
		zap.NewNop().Sugar(),
		&testDatabasePinger{},
		limiter,
		&routeTestAuthService{},
		newTestSessionService(),
		passwordService,
		nil,
		nil,
	)
}
