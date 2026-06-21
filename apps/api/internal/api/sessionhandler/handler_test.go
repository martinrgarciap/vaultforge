package sessionhandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"go.uber.org/zap"
)

const (
	handlerTestUserID = "00000000-0000-0000-0000-000000000401"

	handlerTestCurrentSessionID = "00000000-0000-0000-0000-000000000402"

	handlerTestOtherSessionID = "00000000-0000-0000-0000-000000000403"
)

func TestHandlerListsSessions(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		21,
		18,
		0,
		0,
		0,
		time.UTC,
	)

	service := &handlerTestSessionService{
		sessions: []session.SessionSummary{
			{
				ID:        handlerTestCurrentSessionID,
				UserAgent: "Thunder Client",
				CreatedAt: createdAt,
				ExpiresAt: createdAt.Add(24 * time.Hour),
				Current:   true,
			},
		},
	}

	router := newHandlerTestRouter(service)

	request := newAuthenticatedHandlerRequest(
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

	if service.listCalls != 1 {
		t.Fatalf(
			"ListSessions() calls = %d, want 1",
			service.listCalls,
		)
	}

	if service.lastPrincipal != handlerTestPrincipal() {
		t.Fatalf(
			"principal = %+v, want %+v",
			service.lastPrincipal,
			handlerTestPrincipal(),
		)
	}

	var body listSessionsResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode sessions response: %v",
			err,
		)
	}

	if len(body.Sessions) != 1 {
		t.Fatalf(
			"session count = %d, want 1",
			len(body.Sessions),
		)
	}

	if body.Sessions[0].ID !=
		handlerTestCurrentSessionID {
		t.Fatalf(
			"session ID = %q",
			body.Sessions[0].ID,
		)
	}

	if !body.Sessions[0].Current {
		t.Fatal(
			"current session was not marked current",
		)
	}
}

func TestHandlerLogoutCurrent(t *testing.T) {
	t.Parallel()

	service := &handlerTestSessionService{}
	router := newHandlerTestRouter(service)

	request := newAuthenticatedHandlerRequest(
		http.MethodDelete,
		"/v1/sessions/current",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}

	if service.logoutCurrentCalls != 1 {
		t.Fatalf(
			"LogoutCurrent() calls = %d, want 1",
			service.logoutCurrentCalls,
		)
	}

	if recorder.Body.Len() != 0 {
		t.Fatal(
			"logout response should not include a body",
		)
	}
}

func TestHandlerRevokesOwnedSession(t *testing.T) {
	t.Parallel()

	service := &handlerTestSessionService{}
	router := newHandlerTestRouter(service)

	request := newAuthenticatedHandlerRequest(
		http.MethodDelete,
		"/v1/sessions/"+handlerTestOtherSessionID,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}

	if service.revokeCalls != 1 {
		t.Fatalf(
			"RevokeSession() calls = %d, want 1",
			service.revokeCalls,
		)
	}

	if service.lastSessionID !=
		handlerTestOtherSessionID {
		t.Fatalf(
			"session ID = %q, want %q",
			service.lastSessionID,
			handlerTestOtherSessionID,
		)
	}
}

func TestHandlerLogoutAll(t *testing.T) {
	t.Parallel()

	service := &handlerTestSessionService{}
	router := newHandlerTestRouter(service)

	request := newAuthenticatedHandlerRequest(
		http.MethodDelete,
		"/v1/sessions",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}

	if service.logoutAllCalls != 1 {
		t.Fatalf(
			"LogoutAll() calls = %d, want 1",
			service.logoutAllCalls,
		)
	}
}

func TestHandlerMapsSessionErrorsSafely(
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
			name:       "session not found",
			serviceErr: session.ErrSessionNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "session_not_found",
		},
		{
			name:       "dependency unavailable",
			serviceErr: session.ErrSessionUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "authentication_unavailable",
		},
		{
			name:       "unexpected error",
			serviceErr: errors.New("sensitive internal detail"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &handlerTestSessionService{
				revokeErr: test.serviceErr,
			}

			router := newHandlerTestRouter(service)

			request := newAuthenticatedHandlerRequest(
				http.MethodDelete,
				"/v1/sessions/"+handlerTestOtherSessionID,
			)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf(
					"status = %d, want %d",
					recorder.Code,
					test.wantStatus,
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
					"decode error response: %v",
					err,
				)
			}

			if body.Error.Code != test.wantCode {
				t.Fatalf(
					"error code = %q, want %q",
					body.Error.Code,
					test.wantCode,
				)
			}

			if body.Error.RequestID == "" {
				t.Fatal(
					"error response did not include a request ID",
				)
			}
		})
	}
}

func newHandlerTestRouter(
	service *handlerTestSessionService,
) http.Handler {
	handler := New(
		service,
		zap.NewNop().Sugar(),
	)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)

	router.Group(func(router chi.Router) {
		router.Use(
			appmiddleware.RequireAuthentication(
				&handlerTestAuthenticator{
					principal: handlerTestPrincipal(),
				},
				zap.NewNop().Sugar(),
			),
		)

		router.Get("/v1/sessions", handler.List)
		router.Delete("/v1/sessions", handler.LogoutAll)
		router.Delete("/v1/sessions/current", handler.LogoutCurrent)
		router.Delete("/v1/sessions/{sessionID}", handler.Revoke)
	})

	return router
}

func newAuthenticatedHandlerRequest(
	method string,
	path string,
) *http.Request {
	request := httptest.NewRequest(
		method,
		path,
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
	)

	return request
}

func handlerTestPrincipal() session.Principal {
	return session.Principal{
		UserID:    handlerTestUserID,
		SessionID: handlerTestCurrentSessionID,
	}
}

type handlerTestAuthenticator struct {
	principal session.Principal
}

func (authenticator *handlerTestAuthenticator) AuthenticateAccessToken(
	context.Context,
	string,
) (session.Principal, error) {
	return authenticator.principal, nil
}

type handlerTestSessionService struct {
	sessions []session.SessionSummary

	listErr          error
	logoutCurrentErr error
	revokeErr        error
	logoutAllErr     error

	listCalls          int
	logoutCurrentCalls int
	revokeCalls        int
	logoutAllCalls     int

	lastPrincipal session.Principal
	lastSessionID string
}

func (service *handlerTestSessionService) ListSessions(
	_ context.Context,
	principal session.Principal,
) ([]session.SessionSummary, error) {
	service.listCalls++
	service.lastPrincipal = principal

	if service.listErr != nil {
		return nil, service.listErr
	}

	return append(
		[]session.SessionSummary(nil),
		service.sessions...,
	), nil
}

func (service *handlerTestSessionService) LogoutCurrent(
	_ context.Context,
	principal session.Principal,
) error {
	service.logoutCurrentCalls++
	service.lastPrincipal = principal

	return service.logoutCurrentErr
}

func (service *handlerTestSessionService) RevokeSession(
	_ context.Context,
	principal session.Principal,
	sessionID string,
) error {
	service.revokeCalls++
	service.lastPrincipal = principal
	service.lastSessionID = sessionID

	return service.revokeErr
}

func (service *handlerTestSessionService) LogoutAll(
	_ context.Context,
	principal session.Principal,
) error {
	service.logoutAllCalls++
	service.lastPrincipal = principal

	return service.logoutAllErr
}
