package store_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest/observer"
)

const (
	firstSessionUserAgent = "VaultForge Session Integration First"

	secondSessionUserAgent = "VaultForge Session Integration Second"
)

type authenticationSessionsResponse struct {
	Sessions []struct {
		ID        string    `json:"id"`
		UserAgent string    `json:"userAgent"`
		CreatedAt time.Time `json:"createdAt"`
		ExpiresAt time.Time `json:"expiresAt"`
		Current   bool      `json:"current"`
	} `json:"sessions"`
}

func TestAuthenticationSessionLifecycleIntegration(
	t *testing.T,
) {
	app, _, observedLogs :=
		newAuthenticationIntegrationApplication(t)

	router := app.Routes()

	account := registerAuthenticationSessionUser(
		t,
		router,
		"session-lifecycle@example.com",
	)

	firstLogin := loginAuthenticationSessionUser(
		t,
		router,
		account.User.Email,
		firstSessionUserAgent,
	)

	secondLogin := loginAuthenticationSessionUser(
		t,
		router,
		account.User.Email,
		secondSessionUserAgent,
	)

	accessTokenVerifier :=
		newAuthenticationIntegrationAccessTokenManager(t)

	firstPrincipal, err := accessTokenVerifier.Verify(
		context.Background(),
		firstLogin.AccessToken,
	)
	if err != nil {
		t.Fatalf(
			"verify first access token: %v",
			err,
		)
	}

	secondPrincipal, err := accessTokenVerifier.Verify(
		context.Background(),
		secondLogin.AccessToken,
	)
	if err != nil {
		t.Fatalf(
			"verify second access token: %v",
			err,
		)
	}

	if firstPrincipal.UserID != account.User.ID ||
		secondPrincipal.UserID != account.User.ID {
		t.Fatal(
			"login access tokens did not belong to the registered user",
		)
	}

	if firstPrincipal.SessionID ==
		secondPrincipal.SessionID {
		t.Fatal(
			"separate logins shared a token-family ID",
		)
	}

	listRecorder := performProtectedSessionRequest(
		router,
		http.MethodGet,
		"/v1/sessions",
		secondLogin.AccessToken,
	)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf(
			"list status = %d, want %d",
			listRecorder.Code,
			http.StatusOK,
		)
	}

	listResponse :=
		decodeAuthenticationSessionsResponse(
			t,
			listRecorder,
		)

	if len(listResponse.Sessions) != 2 {
		t.Fatalf(
			"listed session count = %d, want 2",
			len(listResponse.Sessions),
		)
	}

	sessionsByID := make(
		map[string]struct {
			UserAgent string
			Current   bool
		},
		len(listResponse.Sessions),
	)

	for _, listedSession := range listResponse.Sessions {
		sessionsByID[listedSession.ID] = struct {
			UserAgent string
			Current   bool
		}{
			UserAgent: listedSession.UserAgent,
			Current:   listedSession.Current,
		}

		if listedSession.CreatedAt.IsZero() {
			t.Fatal(
				"listed session did not include a creation time",
			)
		}

		if listedSession.ExpiresAt.IsZero() {
			t.Fatal(
				"listed session did not include an expiration time",
			)
		}
	}

	firstListedSession, found :=
		sessionsByID[firstPrincipal.SessionID]
	if !found {
		t.Fatal(
			"first login session was not listed",
		)
	}

	if firstListedSession.UserAgent !=
		firstSessionUserAgent {
		t.Fatalf(
			"first user agent = %q, want %q",
			firstListedSession.UserAgent,
			firstSessionUserAgent,
		)
	}

	if firstListedSession.Current {
		t.Fatal(
			"first session was incorrectly marked current",
		)
	}

	secondListedSession, found :=
		sessionsByID[secondPrincipal.SessionID]
	if !found {
		t.Fatal(
			"second login session was not listed",
		)
	}

	if secondListedSession.UserAgent !=
		secondSessionUserAgent {
		t.Fatalf(
			"second user agent = %q, want %q",
			secondListedSession.UserAgent,
			secondSessionUserAgent,
		)
	}

	if !secondListedSession.Current {
		t.Fatal(
			"second session was not marked current",
		)
	}

	revokeRecorder := performProtectedSessionRequest(
		router,
		http.MethodDelete,
		"/v1/sessions/"+firstPrincipal.SessionID,
		secondLogin.AccessToken,
	)

	if revokeRecorder.Code !=
		http.StatusNoContent {
		t.Fatalf(
			"revoke status = %d, want %d",
			revokeRecorder.Code,
			http.StatusNoContent,
		)
	}

	if revokeRecorder.Body.Len() != 0 {
		t.Fatal(
			"revoke response included a body",
		)
	}

	firstAfterRevocation :=
		performProtectedSessionRequest(
			router,
			http.MethodGet,
			"/v1/sessions",
			firstLogin.AccessToken,
		)

	assertProtectedSessionUnauthorized(
		t,
		firstAfterRevocation,
	)

	secondAfterRevocation :=
		performProtectedSessionRequest(
			router,
			http.MethodGet,
			"/v1/sessions",
			secondLogin.AccessToken,
		)

	if secondAfterRevocation.Code !=
		http.StatusOK {
		t.Fatalf(
			"current session status = %d, want %d",
			secondAfterRevocation.Code,
			http.StatusOK,
		)
	}

	remainingSessions :=
		decodeAuthenticationSessionsResponse(
			t,
			secondAfterRevocation,
		)

	if len(remainingSessions.Sessions) != 1 {
		t.Fatalf(
			"remaining session count = %d, want 1",
			len(remainingSessions.Sessions),
		)
	}

	if remainingSessions.Sessions[0].ID !=
		secondPrincipal.SessionID {
		t.Fatalf(
			"remaining session ID = %q, want %q",
			remainingSessions.Sessions[0].ID,
			secondPrincipal.SessionID,
		)
	}

	if !remainingSessions.Sessions[0].Current {
		t.Fatal(
			"remaining session was not marked current",
		)
	}

	logoutRecorder := performProtectedSessionRequest(
		router,
		http.MethodDelete,
		"/v1/sessions/current",
		secondLogin.AccessToken,
	)

	if logoutRecorder.Code !=
		http.StatusNoContent {
		t.Fatalf(
			"logout status = %d, want %d",
			logoutRecorder.Code,
			http.StatusNoContent,
		)
	}

	secondAfterLogout :=
		performProtectedSessionRequest(
			router,
			http.MethodGet,
			"/v1/sessions",
			secondLogin.AccessToken,
		)

	assertProtectedSessionUnauthorized(
		t,
		secondAfterLogout,
	)

	assertAuthenticationSessionLogsSafe(
		t,
		observedLogs,
		firstLogin.AccessToken,
		secondLogin.AccessToken,
	)
}

func TestAuthenticationSessionOwnershipAndLogoutAllIntegration(
	t *testing.T,
) {
	app, _, observedLogs :=
		newAuthenticationIntegrationApplication(t)

	router := app.Routes()

	owner := registerAuthenticationSessionUser(
		t,
		router,
		"session-ownership-owner@example.com",
	)

	otherUser := registerAuthenticationSessionUser(
		t,
		router,
		"session-ownership-other@example.com",
	)

	ownerFirstLogin :=
		loginAuthenticationSessionUser(
			t,
			router,
			owner.User.Email,
			"VaultForge Owner First",
		)

	ownerSecondLogin :=
		loginAuthenticationSessionUser(
			t,
			router,
			owner.User.Email,
			"VaultForge Owner Second",
		)

	otherLogin :=
		loginAuthenticationSessionUser(
			t,
			router,
			otherUser.User.Email,
			"VaultForge Other User",
		)

	accessTokenVerifier :=
		newAuthenticationIntegrationAccessTokenManager(t)

	ownerFirstPrincipal, err :=
		accessTokenVerifier.Verify(
			context.Background(),
			ownerFirstLogin.AccessToken,
		)
	if err != nil {
		t.Fatalf(
			"verify owner first access token: %v",
			err,
		)
	}

	ownerSecondPrincipal, err :=
		accessTokenVerifier.Verify(
			context.Background(),
			ownerSecondLogin.AccessToken,
		)
	if err != nil {
		t.Fatalf(
			"verify owner second access token: %v",
			err,
		)
	}

	otherPrincipal, err :=
		accessTokenVerifier.Verify(
			context.Background(),
			otherLogin.AccessToken,
		)
	if err != nil {
		t.Fatalf(
			"verify other-user access token: %v",
			err,
		)
	}

	if ownerFirstPrincipal.UserID !=
		owner.User.ID ||
		ownerSecondPrincipal.UserID !=
			owner.User.ID {
		t.Fatal(
			"owner access tokens did not belong to the owner",
		)
	}

	if otherPrincipal.UserID !=
		otherUser.User.ID {
		t.Fatal(
			"other-user access token did not belong to the other user",
		)
	}

	crossUserRecorder :=
		performProtectedSessionRequest(
			router,
			http.MethodDelete,
			"/v1/sessions/"+
				ownerFirstPrincipal.SessionID,
			otherLogin.AccessToken,
		)

	crossUserError := decodeAuthenticationError(
		t,
		crossUserRecorder,
		http.StatusNotFound,
	)

	if crossUserError.Error.Code !=
		"session_not_found" {
		t.Fatalf(
			"cross-user error code = %q, want %q",
			crossUserError.Error.Code,
			"session_not_found",
		)
	}

	if strings.Contains(
		crossUserRecorder.Body.String(),
		owner.User.ID,
	) {
		t.Fatal(
			"cross-user response exposed the owner ID",
		)
	}

	ownerAfterCrossUserAttempt :=
		performProtectedSessionRequest(
			router,
			http.MethodGet,
			"/v1/sessions",
			ownerFirstLogin.AccessToken,
		)

	if ownerAfterCrossUserAttempt.Code !=
		http.StatusOK {
		t.Fatalf(
			"owner session status after cross-user attempt = %d, want %d",
			ownerAfterCrossUserAttempt.Code,
			http.StatusOK,
		)
	}

	ownerSessions :=
		decodeAuthenticationSessionsResponse(
			t,
			ownerAfterCrossUserAttempt,
		)

	if len(ownerSessions.Sessions) != 2 {
		t.Fatalf(
			"owner session count = %d, want 2",
			len(ownerSessions.Sessions),
		)
	}

	unknownSessionRecorder :=
		performProtectedSessionRequest(
			router,
			http.MethodDelete,
			"/v1/sessions/00000000-0000-0000-0000-000000000999",
			ownerSecondLogin.AccessToken,
		)

	unknownSessionError :=
		decodeAuthenticationError(
			t,
			unknownSessionRecorder,
			http.StatusNotFound,
		)

	if unknownSessionError.Error.Code !=
		"session_not_found" {
		t.Fatalf(
			"unknown-session error code = %q, want %q",
			unknownSessionError.Error.Code,
			"session_not_found",
		)
	}

	logoutAllRecorder :=
		performProtectedSessionRequest(
			router,
			http.MethodDelete,
			"/v1/sessions",
			ownerSecondLogin.AccessToken,
		)

	if logoutAllRecorder.Code !=
		http.StatusNoContent {
		t.Fatalf(
			"logout-all status = %d, want %d",
			logoutAllRecorder.Code,
			http.StatusNoContent,
		)
	}

	if logoutAllRecorder.Body.Len() != 0 {
		t.Fatal(
			"logout-all response included a body",
		)
	}

	ownerFirstAfterLogoutAll :=
		performProtectedSessionRequest(
			router,
			http.MethodGet,
			"/v1/sessions",
			ownerFirstLogin.AccessToken,
		)

	assertProtectedSessionUnauthorized(
		t,
		ownerFirstAfterLogoutAll,
	)

	ownerSecondAfterLogoutAll :=
		performProtectedSessionRequest(
			router,
			http.MethodGet,
			"/v1/sessions",
			ownerSecondLogin.AccessToken,
		)

	assertProtectedSessionUnauthorized(
		t,
		ownerSecondAfterLogoutAll,
	)

	otherUserAfterOwnerLogoutAll :=
		performProtectedSessionRequest(
			router,
			http.MethodGet,
			"/v1/sessions",
			otherLogin.AccessToken,
		)

	if otherUserAfterOwnerLogoutAll.Code !=
		http.StatusOK {
		t.Fatalf(
			"other-user status after owner logout-all = %d, want %d",
			otherUserAfterOwnerLogoutAll.Code,
			http.StatusOK,
		)
	}

	otherSessions :=
		decodeAuthenticationSessionsResponse(
			t,
			otherUserAfterOwnerLogoutAll,
		)

	if len(otherSessions.Sessions) != 1 {
		t.Fatalf(
			"other-user session count = %d, want 1",
			len(otherSessions.Sessions),
		)
	}

	if otherSessions.Sessions[0].ID !=
		otherPrincipal.SessionID {
		t.Fatalf(
			"other-user session ID = %q, want %q",
			otherSessions.Sessions[0].ID,
			otherPrincipal.SessionID,
		)
	}

	if !otherSessions.Sessions[0].Current {
		t.Fatal(
			"other-user session was not marked current",
		)
	}

	assertAuthenticationSessionLogsSafe(
		t,
		observedLogs,
		ownerFirstLogin.AccessToken,
		ownerSecondLogin.AccessToken,
		otherLogin.AccessToken,
	)
}

func registerAuthenticationSessionUser(
	t *testing.T,
	router http.Handler,
	email string,
) authenticationAccountResponse {
	t.Helper()

	recorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/register",
		email,
		authIntegrationPassword,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"registration status = %d, want %d",
			recorder.Code,
			http.StatusCreated,
		)
	}

	var response authenticationAccountResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&response); err != nil {
		t.Fatalf(
			"decode registration response: %v",
			err,
		)
	}

	return response
}

func loginAuthenticationSessionUser(
	t *testing.T,
	router http.Handler,
	email string,
	userAgent string,
) authenticationLoginResponse {
	t.Helper()

	requestBody, err := json.Marshal(
		struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}{
			Email:    email,
			Password: authIntegrationPassword,
		},
	)
	if err != nil {
		t.Fatalf(
			"encode login request: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/login",
		strings.NewReader(string(requestBody)),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	request.Header.Set(
		"User-Agent",
		userAgent,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"login status = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	var response authenticationLoginResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&response); err != nil {
		t.Fatalf(
			"decode login response: %v",
			err,
		)
	}

	return response
}

func performProtectedSessionRequest(
	router http.Handler,
	method string,
	path string,
	accessToken string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		method,
		path,
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func decodeAuthenticationSessionsResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
) authenticationSessionsResponse {
	t.Helper()

	var response authenticationSessionsResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&response); err != nil {
		t.Fatalf(
			"decode sessions response: %v",
			err,
		)
	}

	if response.Sessions == nil {
		t.Fatal(
			"sessions response contained null",
		)
	}

	return response
}

func assertProtectedSessionUnauthorized(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
) {
	t.Helper()

	errorResponse := decodeAuthenticationError(
		t,
		recorder,
		http.StatusUnauthorized,
	)

	if errorResponse.Error.Code != "unauthorized" {
		t.Fatalf(
			"error code = %q, want unauthorized",
			errorResponse.Error.Code,
		)
	}

	if recorder.Header().Get(
		"WWW-Authenticate",
	) != "Bearer" {
		t.Fatal(
			"unauthorized response did not include a Bearer challenge",
		)
	}
}

func assertAuthenticationSessionLogsSafe(
	t *testing.T,
	observedLogs interface {
		All() []observer.LoggedEntry
	},
	accessTokens ...string,
) {
	t.Helper()

	for _, entry := range observedLogs.All() {
		encodedEntry, err := json.Marshal(
			struct {
				Message string         `json:"message"`
				Context map[string]any `json:"context"`
			}{
				Message: entry.Message,
				Context: entry.ContextMap(),
			},
		)
		if err != nil {
			t.Fatalf(
				"encode session log entry: %v",
				err,
			)
		}

		logText := string(encodedEntry)

		for _, accessToken := range accessTokens {
			if accessToken != "" &&
				strings.Contains(
					logText,
					accessToken,
				) {
				t.Fatal(
					"session log exposed an access token",
				)
			}
		}
	}
}
