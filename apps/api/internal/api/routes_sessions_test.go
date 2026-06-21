package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"go.uber.org/zap"
)

const (
	routeSessionTestUserID = "user-123"

	routeSessionTestCurrentID = "00000000-0000-0000-0000-000000000102"

	routeSessionTestOtherID = "00000000-0000-0000-0000-000000000199"
)

func TestRoutesSessionsRequireAuthentication(
	t *testing.T,
) {
	t.Parallel()

	app := newApplicationWithAuthService(
		&routeTestAuthService{},
		zap.NewNop().Sugar(),
	)

	router := app.Routes()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "list sessions",
			method: http.MethodGet,
			path:   "/v1/sessions",
		},
		{
			name:   "logout all",
			method: http.MethodDelete,
			path:   "/v1/sessions",
		},
		{
			name:   "logout current",
			method: http.MethodDelete,
			path:   "/v1/sessions/current",
		},
		{
			name:   "revoke session",
			method: http.MethodDelete,
			path: "/v1/sessions/" +
				routeSessionTestOtherID,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(
				test.method,
				test.path,
				nil,
			)

			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code !=
				http.StatusUnauthorized {
				t.Fatalf(
					"status = %d, want %d",
					recorder.Code,
					http.StatusUnauthorized,
				)
			}

			if recorder.Header().Get(
				"WWW-Authenticate",
			) != "Bearer" {
				t.Fatal(
					"unauthorized response did not include a Bearer challenge",
				)
			}

			var body errorResponseBody

			if err := json.NewDecoder(
				recorder.Body,
			).Decode(&body); err != nil {
				t.Fatalf(
					"decode unauthorized response: %v",
					err,
				)
			}

			if body.Error.Code != "unauthorized" {
				t.Fatalf(
					"error code = %q, want unauthorized",
					body.Error.Code,
				)
			}

			if body.Error.RequestID == "" {
				t.Fatal(
					"unauthorized response did not include a request ID",
				)
			}
		})
	}
}

func TestRoutesListSessionsAuthenticated(
	t *testing.T,
) {
	t.Parallel()

	app := newApplicationWithAuthService(
		&routeTestAuthService{},
		zap.NewNop().Sugar(),
	)

	router := app.Routes()

	request := newAuthenticatedSessionRouteRequest(
		t,
		http.MethodGet,
		"/v1/sessions",
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
		Sessions []struct {
			ID        string `json:"id"`
			UserAgent string `json:"userAgent"`
			Current   bool   `json:"current"`
		} `json:"sessions"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode sessions response: %v",
			err,
		)
	}

	if body.Sessions == nil {
		t.Fatal(
			"sessions response contained null instead of an empty array",
		)
	}

	if len(body.Sessions) != 0 {
		t.Fatalf(
			"session count = %d, want 0",
			len(body.Sessions),
		)
	}
}

func TestRoutesSessionRevocationActions(
	t *testing.T,
) {
	t.Parallel()

	app := newApplicationWithAuthService(
		&routeTestAuthService{},
		zap.NewNop().Sugar(),
	)

	router := app.Routes()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "logout all",
			path: "/v1/sessions",
		},
		{
			name: "logout current",
			path: "/v1/sessions/current",
		},
		{
			name: "revoke other session",
			path: "/v1/sessions/" +
				routeSessionTestOtherID,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			request :=
				newAuthenticatedSessionRouteRequest(
					t,
					http.MethodDelete,
					test.path,
				)

			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code !=
				http.StatusNoContent {
				t.Fatalf(
					"status = %d, want %d",
					recorder.Code,
					http.StatusNoContent,
				)
			}

			if recorder.Body.Len() != 0 {
				t.Fatal(
					"successful revocation response included a body",
				)
			}
		})
	}
}

func TestRoutesSessionsRejectInvalidAccessToken(
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
		"/v1/sessions",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer malformed-access-token",
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
			"decode invalid-token response: %v",
			err,
		)
	}

	if body.Error.Code != "unauthorized" {
		t.Fatalf(
			"error code = %q, want unauthorized",
			body.Error.Code,
		)
	}
}

func newAuthenticatedSessionRouteRequest(
	t *testing.T,
	method string,
	path string,
) *http.Request {
	t.Helper()

	accessToken, err :=
		newTestAccessTokenManager().Issue(
			context.Background(),
			session.Principal{
				UserID:    routeSessionTestUserID,
				SessionID: routeSessionTestCurrentID,
			},
		)
	if err != nil {
		t.Fatalf(
			"issue session route access token: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		method,
		path,
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer "+accessToken.Value(),
	)

	return request
}
