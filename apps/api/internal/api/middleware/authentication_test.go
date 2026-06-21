package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"go.uber.org/zap"
)

func TestRequireAuthenticationAddsPrincipalToContext(
	t *testing.T,
) {
	t.Parallel()

	expectedPrincipal := session.Principal{
		UserID:    "user-123",
		SessionID: "session-456",
	}

	authenticator :=
		&middlewareTestAuthenticator{
			principal: expectedPrincipal,
		}

	nextCalled := false

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		nextCalled = true

		principal, ok := PrincipalFromContext(
			r.Context(),
		)
		if !ok {
			t.Fatal(
				"authenticated principal was missing from context",
			)
		}

		if principal != expectedPrincipal {
			t.Fatalf(
				"principal = %+v, want %+v",
				principal,
				expectedPrincipal,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := chimiddleware.RequestID(
		RequireAuthentication(
			authenticator,
			zap.NewNop().Sugar(),
		)(next),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/sessions",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
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

	if !nextCalled {
		t.Fatal(
			"authenticated request did not reach the next handler",
		)
	}

	if authenticator.calls != 1 {
		t.Fatalf(
			"AuthenticateAccessToken() calls = %d, want 1",
			authenticator.calls,
		)
	}

	if authenticator.lastToken !=
		"synthetic-access-token" {
		t.Fatal(
			"authenticator received the wrong token",
		)
	}
}

func TestRequireAuthenticationRejectsMissingOrMalformedHeader(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name   string
		values []string
	}{
		{
			name: "missing header",
		},
		{
			name: "wrong scheme",
			values: []string{
				"Basic synthetic-token",
			},
		},
		{
			name: "missing token",
			values: []string{
				"Bearer",
			},
		},
		{
			name: "empty token",
			values: []string{
				"Bearer ",
			},
		},
		{
			name: "multiple spaces",
			values: []string{
				"Bearer  synthetic-token",
			},
		},
		{
			name: "token contains whitespace",
			values: []string{
				"Bearer synthetic token",
			},
		},
		{
			name: "duplicate headers",
			values: []string{
				"Bearer first-token",
				"Bearer second-token",
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			authenticator :=
				&middlewareTestAuthenticator{}

			nextCalled := false

			next := http.HandlerFunc(func(
				http.ResponseWriter,
				*http.Request,
			) {
				nextCalled = true
			})

			handler := chimiddleware.RequestID(
				RequireAuthentication(
					authenticator,
					zap.NewNop().Sugar(),
				)(next),
			)

			request := httptest.NewRequest(
				http.MethodGet,
				"/v1/sessions",
				nil,
			)

			for _, value := range test.values {
				request.Header.Add(
					"Authorization",
					value,
				)
			}

			recorder := httptest.NewRecorder()

			handler.ServeHTTP(
				recorder,
				request,
			)

			assertMiddlewareAuthenticationError(
				t,
				recorder,
				http.StatusUnauthorized,
				"unauthorized",
			)

			if recorder.Header().Get(
				"WWW-Authenticate",
			) != "Bearer" {
				t.Fatal(
					"unauthorized response did not include a Bearer challenge",
				)
			}

			if authenticator.calls != 0 {
				t.Fatal(
					"authenticator was called for a malformed header",
				)
			}

			if nextCalled {
				t.Fatal(
					"malformed request reached the next handler",
				)
			}
		})
	}
}

func TestRequireAuthenticationMapsInvalidTokenGenerically(
	t *testing.T,
) {
	t.Parallel()

	const submittedToken = "synthetic-invalid-access-token"

	authenticator :=
		&middlewareTestAuthenticator{
			err: session.ErrAccessTokenInvalid,
		}

	handler := chimiddleware.RequestID(
		RequireAuthentication(
			authenticator,
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				http.ResponseWriter,
				*http.Request,
			) {
				t.Fatal(
					"invalid token reached the protected handler",
				)
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/sessions",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer "+submittedToken,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assertMiddlewareAuthenticationError(
		t,
		recorder,
		http.StatusUnauthorized,
		"unauthorized",
	)

	if strings.Contains(
		recorder.Body.String(),
		submittedToken,
	) {
		t.Fatal(
			"unauthorized response exposed the submitted token",
		)
	}
}

func TestRequireAuthenticationMapsDependencyFailureSafely(
	t *testing.T,
) {
	t.Parallel()

	authenticator :=
		&middlewareTestAuthenticator{
			err: session.ErrSessionUnavailable,
		}

	handler := chimiddleware.RequestID(
		RequireAuthentication(
			authenticator,
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				http.ResponseWriter,
				*http.Request,
			) {
				t.Fatal(
					"failed authentication reached the protected handler",
				)
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/sessions",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assertMiddlewareAuthenticationError(
		t,
		recorder,
		http.StatusServiceUnavailable,
		"authentication_unavailable",
	)
}

func TestRequireAuthenticationMapsUnexpectedFailureSafely(
	t *testing.T,
) {
	t.Parallel()

	const internalMarker = "synthetic-sensitive-authentication-error"

	authenticator :=
		&middlewareTestAuthenticator{
			err: errors.New(internalMarker),
		}

	handler := chimiddleware.RequestID(
		RequireAuthentication(
			authenticator,
			zap.NewNop().Sugar(),
		)(
			http.HandlerFunc(func(
				http.ResponseWriter,
				*http.Request,
			) {
				t.Fatal(
					"failed authentication reached the protected handler",
				)
			}),
		),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/sessions",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assertMiddlewareAuthenticationError(
		t,
		recorder,
		http.StatusInternalServerError,
		"internal_error",
	)

	if strings.Contains(
		recorder.Body.String(),
		internalMarker,
	) {
		t.Fatal(
			"authentication response exposed an internal error",
		)
	}
}

func TestPrincipalFromContextRejectsMissingPrincipal(
	t *testing.T,
) {
	t.Parallel()

	_, ok := PrincipalFromContext(
		context.Background(),
	)

	if ok {
		t.Fatal(
			"empty context unexpectedly contained a principal",
		)
	}
}

func assertMiddlewareAuthenticationError(
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
			"decode authentication error: %v",
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
			"authentication error did not include a safe message",
		)
	}

	if body.Error.RequestID == "" {
		t.Fatal(
			"authentication error did not include a request ID",
		)
	}
}

type middlewareTestAuthenticator struct {
	principal session.Principal
	err       error
	calls     int
	lastToken string
}

func (authenticator *middlewareTestAuthenticator) AuthenticateAccessToken(
	_ context.Context,
	tokenValue string,
) (session.Principal, error) {
	authenticator.calls++
	authenticator.lastToken = tokenValue

	if authenticator.err != nil {
		return session.Principal{},
			authenticator.err
	}

	return authenticator.principal, nil
}
