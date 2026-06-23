package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"go.uber.org/zap"
)

type rateLimitTestLimiter struct {
	decision     ratelimit.Decision
	err          error
	calls        int
	lastScope    string
	lastIdentity []string
	lastPolicy   ratelimit.Policy
}

func (limiter *rateLimitTestLimiter) Allow(
	_ context.Context,
	scope string,
	policy ratelimit.Policy,
	identityParts ...string,
) (ratelimit.Decision, error) {
	limiter.calls++
	limiter.lastScope = scope
	limiter.lastPolicy = policy
	limiter.lastIdentity = append(
		[]string(nil),
		identityParts...,
	)

	return limiter.decision, limiter.err
}

func testRateLimitPolicy(
	t *testing.T,
) ratelimit.Policy {
	t.Helper()

	policy, err := ratelimit.NewPolicy(
		5,
		time.Minute,
	)
	if err != nil {
		t.Fatalf(
			"create rate-limit policy: %v",
			err,
		)
	}

	return policy
}

func TestRateLimitByPeerIPAllowsRequest(
	t *testing.T,
) {
	t.Parallel()

	limiter := &rateLimitTestLimiter{
		decision: ratelimit.Decision{
			Allowed:   true,
			Remaining: 4,
		},
	}

	handlerCalled := false

	handler := chimiddleware.RequestID(
		RateLimitByPeerIP(
			limiter,
			ratelimit.ScopeRegistration,
			testRateLimitPolicy(t),
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				handlerCalled = true
				w.WriteHeader(http.StatusNoContent)
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/register",
		nil,
	)
	request.RemoteAddr = "203.0.113.10:54321"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}

	if !handlerCalled {
		t.Fatal("allowed request did not reach handler")
	}

	if limiter.calls != 1 {
		t.Fatalf(
			"limiter calls = %d, want 1",
			limiter.calls,
		)
	}

	if limiter.lastScope !=
		ratelimit.ScopeRegistration {
		t.Fatalf(
			"scope = %q",
			limiter.lastScope,
		)
	}

	if len(limiter.lastIdentity) != 1 ||
		limiter.lastIdentity[0] !=
			"203.0.113.10" {
		t.Fatalf(
			"identity = %#v",
			limiter.lastIdentity,
		)
	}
}

func TestRateLimitByPeerIPIgnoresForwardedHeaders(
	t *testing.T,
) {
	t.Parallel()

	limiter := &rateLimitTestLimiter{
		decision: ratelimit.Decision{
			Allowed: true,
		},
	}

	handler := chimiddleware.RequestID(
		RateLimitByPeerIP(
			limiter,
			ratelimit.ScopeLogin,
			testRateLimitPolicy(t),
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				w.WriteHeader(http.StatusNoContent)
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/login",
		nil,
	)
	request.RemoteAddr = "203.0.113.10:12345"
	request.Header.Set(
		"X-Forwarded-For",
		"198.51.100.200",
	)
	request.Header.Set(
		"X-Real-IP",
		"198.51.100.201",
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}

	if len(limiter.lastIdentity) != 1 ||
		limiter.lastIdentity[0] !=
			"203.0.113.10" {
		t.Fatalf(
			"identity = %#v",
			limiter.lastIdentity,
		)
	}
}

func TestRateLimitByPeerIPReturnsTooManyRequests(
	t *testing.T,
) {
	t.Parallel()

	limiter := &rateLimitTestLimiter{
		decision: ratelimit.Decision{
			Allowed:    false,
			RetryAfter: 1500 * time.Millisecond,
		},
	}

	handlerCalled := false

	handler := chimiddleware.RequestID(
		RateLimitByPeerIP(
			limiter,
			ratelimit.ScopeRefresh,
			testRateLimitPolicy(t),
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				http.ResponseWriter,
				*http.Request,
			) {
				handlerCalled = true
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/refresh",
		nil,
	)
	request.RemoteAddr = "203.0.113.10:45678"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assertRateLimitResponse(
		t,
		recorder,
		http.StatusTooManyRequests,
		"rate_limit_exceeded",
	)

	if recorder.Header().Get("Retry-After") != "2" {
		t.Fatalf(
			"Retry-After = %q, want %q",
			recorder.Header().Get(
				"Retry-After",
			),
			"2",
		)
	}

	if handlerCalled {
		t.Fatal("denied request reached handler")
	}
}

func TestRateLimitByPeerIPFailsClosed(
	t *testing.T,
) {
	t.Parallel()

	const internalMarker = "synthetic Redis connection details"

	limiter := &rateLimitTestLimiter{
		err: errors.New(internalMarker),
	}

	handler := chimiddleware.RequestID(
		RateLimitByPeerIP(
			limiter,
			ratelimit.ScopeLogin,
			testRateLimitPolicy(t),
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				http.ResponseWriter,
				*http.Request,
			) {
				t.Fatal(
					"failed limiter reached handler",
				)
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/login",
		nil,
	)
	request.RemoteAddr = "203.0.113.10:45678"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assertRateLimitResponse(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"authentication_unavailable",
	)

	if strings.Contains(
		recorder.Body.String(),
		internalMarker,
	) {
		t.Fatal(
			"response exposed limiter details",
		)
	}
}

func TestRateLimitByPeerIPRejectsInvalidRemoteAddress(
	t *testing.T,
) {
	t.Parallel()

	limiter := &rateLimitTestLimiter{
		decision: ratelimit.Decision{
			Allowed: true,
		},
	}

	handler := chimiddleware.RequestID(
		RateLimitByPeerIP(
			limiter,
			ratelimit.ScopeLogin,
			testRateLimitPolicy(t),
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				http.ResponseWriter,
				*http.Request,
			) {
				t.Fatal(
					"invalid identity reached handler",
				)
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/login",
		nil,
	)
	request.RemoteAddr = "not-an-ip-address"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assertRateLimitResponse(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"authentication_unavailable",
	)

	if limiter.calls != 0 {
		t.Fatalf(
			"limiter calls = %d, want 0",
			limiter.calls,
		)
	}
}

func TestRateLimitByPrincipalUsesAuthenticatedUser(
	t *testing.T,
) {
	t.Parallel()

	limiter := &rateLimitTestLimiter{
		decision: ratelimit.Decision{
			Allowed: true,
		},
	}

	handler := chimiddleware.RequestID(
		RateLimitByPrincipal(
			limiter,
			ratelimit.ScopeMutation,
			testRateLimitPolicy(t),
			"service_unavailable",
			"The service is temporarily unavailable.",
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				w.WriteHeader(http.StatusNoContent)
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/vaults",
		nil,
	)

	requestContext := context.WithValue(
		request.Context(),
		principalContextKey{},
		session.Principal{
			UserID:    "user-123",
			SessionID: "session-123",
		},
	)

	request = request.WithContext(
		requestContext,
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}

	if len(limiter.lastIdentity) != 1 ||
		limiter.lastIdentity[0] != "user-123" {
		t.Fatalf(
			"identity = %#v",
			limiter.lastIdentity,
		)
	}
}

func TestRateLimitByPrincipalRejectsMissingPrincipal(
	t *testing.T,
) {
	t.Parallel()

	limiter := &rateLimitTestLimiter{
		decision: ratelimit.Decision{
			Allowed: true,
		},
	}

	handler := chimiddleware.RequestID(
		RateLimitByPrincipal(
			limiter,
			ratelimit.ScopeMutation,
			testRateLimitPolicy(t),
			"service_unavailable",
			"The service is temporarily unavailable.",
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				http.ResponseWriter,
				*http.Request,
			) {
				t.Fatal(
					"missing principal reached handler",
				)
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/vaults",
		nil,
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assertRateLimitResponse(
		t,
		recorder,
		http.StatusUnauthorized,
		"unauthorized",
	)

	if limiter.calls != 0 {
		t.Fatalf(
			"limiter calls = %d, want 0",
			limiter.calls,
		)
	}

	if recorder.Header().Get(
		"WWW-Authenticate",
	) != "Bearer" {
		t.Fatal(
			"unauthorized response omitted Bearer challenge",
		)
	}
}

func TestPeerIPCanonicalizesIPv6(
	t *testing.T,
) {
	t.Parallel()

	request := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)
	request.RemoteAddr =
		"[2001:0db8:0000:0000:0000:0000:0000:0001]:443"

	address, ok := PeerIP(request)
	if !ok {
		t.Fatal("expected valid IPv6 peer")
	}

	if address != "2001:db8::1" {
		t.Fatalf(
			"address = %q, want %q",
			address,
			"2001:db8::1",
		)
	}
}

func assertRateLimitResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			wantStatus,
			recorder.Body.String(),
		)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode response: %v",
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
		t.Fatal("response omitted safe message")
	}

	if body.Error.RequestID == "" {
		t.Fatal("response omitted request ID")
	}
}
