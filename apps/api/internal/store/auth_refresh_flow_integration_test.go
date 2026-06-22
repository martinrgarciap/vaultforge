package store_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessioncookie"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"go.uber.org/zap/zaptest/observer"
)

var errAuthenticationRefreshAccessTokenIssue = errors.New(
	"synthetic refresh access-token issuance failure",
)

type authenticationRefreshResponse struct {
	TokenType             string    `json:"tokenType"`
	AccessToken           string    `json:"accessToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshToken          string    `json:"-"`
	CSRFToken             string    `json:"-"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

func TestAuthenticationRefreshRotationAndReplayIntegration(t *testing.T) {
	app, databasePool, observedLogs :=
		newAuthenticationIntegrationApplication(t)

	router := app.Routes()

	accountResponse, loginResponse :=
		registerAndLoginAuthenticationRefreshUser(
			t,
			router,
			"refresh-flow@example.com",
		)

	currentRefreshToken, err :=
		session.ParseRefreshToken(
			loginResponse.RefreshToken,
		)
	if err != nil {
		t.Fatalf(
			"parse login refresh token: %v",
			err,
		)
	}

	currentDigest, err := currentRefreshToken.Digest()
	if err != nil {
		t.Fatalf(
			"digest login refresh token: %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		authIntegrationTimeout,
	)
	defer cancelQuery()

	var (
		tokenFamilyID         string
		originalExpiresAt     time.Time
		originalStoredHash    []byte
		originalStoredRevoked bool
	)

	err = databasePool.QueryRow(
		queryContext,
		`
			SELECT
				token_family_id::text,
				expires_at,
				refresh_token_hash,
				revoked_at IS NOT NULL
			FROM sessions
			WHERE refresh_token_hash = $1
		`,
		currentDigest.Bytes(),
	).Scan(
		&tokenFamilyID,
		&originalExpiresAt,
		&originalStoredHash,
		&originalStoredRevoked,
	)
	if err != nil {
		t.Fatalf(
			"read original refresh session: %v",
			err,
		)
	}

	if tokenFamilyID == "" {
		t.Fatal(
			"original session did not have a token-family ID",
		)
	}

	if originalStoredRevoked {
		t.Fatal(
			"original session was revoked before refresh",
		)
	}

	if !bytes.Equal(
		originalStoredHash,
		currentDigest.Bytes(),
	) {
		t.Fatal(
			"original stored digest did not match the login refresh token",
		)
	}

	refreshRecorder := performAuthenticationRefreshRequest(
		t,
		router,
		loginResponse.RefreshToken,
		loginResponse.CSRFToken,
	)

	if refreshRecorder.Code != http.StatusOK {
		t.Fatalf(
			"refresh status = %d, want %d",
			refreshRecorder.Code,
			http.StatusOK,
		)
	}

	var refreshResponse authenticationRefreshResponse

	if err := json.NewDecoder(
		refreshRecorder.Body,
	).Decode(&refreshResponse); err != nil {
		t.Fatalf(
			"decode refresh response: %v",
			err,
		)
	}

	cookieConfig := sessioncookie.NewConfig(false)

	refreshResponse.RefreshToken = authenticationResponseCookieValue(
		t,
		refreshRecorder,
		cookieConfig.RefreshCookieName(),
	)

	refreshResponse.CSRFToken = authenticationResponseCookieValue(
		t,
		refreshRecorder,
		cookieConfig.CSRFCookieName(),
	)

	if refreshResponse.TokenType != "Bearer" {
		t.Fatalf(
			"refresh token type = %q, want Bearer",
			refreshResponse.TokenType,
		)
	}

	if refreshResponse.AccessToken == "" {
		t.Fatal(
			"refresh response did not include an access token",
		)
	}

	if refreshResponse.RefreshToken == "" {
		t.Fatal(
			"refresh response did not include a replacement refresh token",
		)
	}

	if refreshResponse.RefreshToken ==
		loginResponse.RefreshToken {
		t.Fatal(
			"refresh returned the presented refresh token instead of a replacement",
		)
	}

	if refreshResponse.CSRFToken == "" {
		t.Fatal("refresh response did not include a replacement CSRF cookie")
	}

	if refreshResponse.CSRFToken == loginResponse.CSRFToken {
		t.Fatal("refresh did not rotate the CSRF cookie")
	}

	if refreshResponse.AccessTokenExpiresAt.IsZero() {
		t.Fatal(
			"refresh response did not include an access-token expiration",
		)
	}

	if !refreshResponse.RefreshTokenExpiresAt.Equal(
		loginResponse.RefreshTokenExpiresAt,
	) {
		t.Fatalf(
			"refresh expiration changed from %v to %v",
			loginResponse.RefreshTokenExpiresAt,
			refreshResponse.RefreshTokenExpiresAt,
		)
	}

	replacementRefreshToken, err :=
		session.ParseRefreshToken(
			refreshResponse.RefreshToken,
		)
	if err != nil {
		t.Fatalf(
			"parse replacement refresh token: %v",
			err,
		)
	}

	replacementDigest, err :=
		replacementRefreshToken.Digest()
	if err != nil {
		t.Fatalf(
			"digest replacement refresh token: %v",
			err,
		)
	}

	var (
		storedReplacementFamilyID string
		storedReplacementExpiry   time.Time
		storedReplacementHash     []byte
		storedReplacementRevoked  bool
		storedReplacementAgent    string
		storedOriginalRevoked     bool
	)

	err = databasePool.QueryRow(
		queryContext,
		`
			SELECT
				token_family_id::text,
				expires_at,
				refresh_token_hash,
				revoked_at IS NOT NULL,
				COALESCE(user_agent, '')
			FROM sessions
			WHERE refresh_token_hash = $1
		`,
		replacementDigest.Bytes(),
	).Scan(
		&storedReplacementFamilyID,
		&storedReplacementExpiry,
		&storedReplacementHash,
		&storedReplacementRevoked,
		&storedReplacementAgent,
	)
	if err != nil {
		t.Fatalf(
			"read replacement refresh session: %v",
			err,
		)
	}

	err = databasePool.QueryRow(
		queryContext,
		`
			SELECT revoked_at IS NOT NULL
			FROM sessions
			WHERE refresh_token_hash = $1
		`,
		currentDigest.Bytes(),
	).Scan(&storedOriginalRevoked)
	if err != nil {
		t.Fatalf(
			"read original revocation state: %v",
			err,
		)
	}

	if !storedOriginalRevoked {
		t.Fatal(
			"successful refresh did not revoke the original row",
		)
	}

	if storedReplacementRevoked {
		t.Fatal(
			"replacement refresh row should initially be active",
		)
	}

	if storedReplacementFamilyID != tokenFamilyID {
		t.Fatalf(
			"replacement family ID = %q, want %q",
			storedReplacementFamilyID,
			tokenFamilyID,
		)
	}

	if !storedReplacementExpiry.Equal(
		originalExpiresAt,
	) {
		t.Fatalf(
			"replacement expiry = %v, want original expiry %v",
			storedReplacementExpiry,
			originalExpiresAt,
		)
	}

	if !bytes.Equal(
		storedReplacementHash,
		replacementDigest.Bytes(),
	) {
		t.Fatal(
			"stored replacement digest did not match the returned token",
		)
	}

	if bytes.Equal(
		storedReplacementHash,
		[]byte(refreshResponse.RefreshToken),
	) {
		t.Fatal(
			"database stored the plaintext replacement refresh token",
		)
	}

	if storedReplacementAgent !=
		authIntegrationUserAgent {
		t.Fatalf(
			"replacement user agent = %q, want %q",
			storedReplacementAgent,
			authIntegrationUserAgent,
		)
	}

	accessTokenVerifier :=
		newAuthenticationIntegrationAccessTokenManager(t)

	principal, err := accessTokenVerifier.Verify(
		context.Background(),
		refreshResponse.AccessToken,
	)
	if err != nil {
		t.Fatalf(
			"verify refreshed access token: %v",
			err,
		)
	}

	if principal.UserID != accountResponse.User.ID {
		t.Fatalf(
			"refreshed access-token user ID = %q, want %q",
			principal.UserID,
			accountResponse.User.ID,
		)
	}

	if principal.SessionID != tokenFamilyID {
		t.Fatalf(
			"refreshed access-token session ID = %q, want %q",
			principal.SessionID,
			tokenFamilyID,
		)
	}

	var (
		familyRowCount       int
		activeFamilyRowCount int
	)

	err = databasePool.QueryRow(
		queryContext,
		`
			SELECT
				count(*),
				count(*) FILTER (
					WHERE revoked_at IS NULL
				)
			FROM sessions
			WHERE token_family_id = $1::uuid
		`,
		tokenFamilyID,
	).Scan(
		&familyRowCount,
		&activeFamilyRowCount,
	)
	if err != nil {
		t.Fatalf(
			"count refreshed family rows: %v",
			err,
		)
	}

	if familyRowCount != 2 {
		t.Fatalf(
			"family row count = %d, want 2",
			familyRowCount,
		)
	}

	if activeFamilyRowCount != 1 {
		t.Fatalf(
			"active family row count = %d, want 1",
			activeFamilyRowCount,
		)
	}

	replayRecorder := performAuthenticationRefreshRequest(
		t,
		router,
		loginResponse.RefreshToken,
		loginResponse.CSRFToken,
	)

	replayError := decodeAuthenticationRefreshError(
		t,
		replayRecorder,
		http.StatusUnauthorized,
	)

	if replayError.Error.Code !=
		"invalid_refresh_token" {
		t.Fatalf(
			"replay error code = %q, want invalid_refresh_token",
			replayError.Error.Code,
		)
	}

	replacementAfterReplayRecorder := performAuthenticationRefreshRequest(
		t,
		router,
		refreshResponse.RefreshToken,
		refreshResponse.CSRFToken,
	)

	replacementAfterReplayError :=
		decodeAuthenticationRefreshError(
			t,
			replacementAfterReplayRecorder,
			http.StatusUnauthorized,
		)

	if replacementAfterReplayError.Error.Code !=
		replayError.Error.Code {
		t.Fatalf(
			"replacement error code = %q, replay error code = %q",
			replacementAfterReplayError.Error.Code,
			replayError.Error.Code,
		)
	}

	if replacementAfterReplayError.Error.Message !=
		replayError.Error.Message {
		t.Fatal(
			"replacement and replay errors exposed different public messages",
		)
	}

	err = databasePool.QueryRow(
		queryContext,
		`
			SELECT
				count(*),
				count(*) FILTER (
					WHERE revoked_at IS NULL
				)
			FROM sessions
			WHERE token_family_id = $1::uuid
		`,
		tokenFamilyID,
	).Scan(
		&familyRowCount,
		&activeFamilyRowCount,
	)
	if err != nil {
		t.Fatalf(
			"count family rows after replay: %v",
			err,
		)
	}

	if familyRowCount != 2 {
		t.Fatalf(
			"family row count after replay = %d, want 2",
			familyRowCount,
		)
	}

	if activeFamilyRowCount != 0 {
		t.Fatalf(
			"active family rows after replay = %d, want 0",
			activeFamilyRowCount,
		)
	}

	assertAuthenticationRefreshLogsSafe(
		t,
		observedLogs,
		loginResponse.RefreshToken,
		refreshResponse.RefreshToken,
		currentDigest.Bytes(),
		replacementDigest.Bytes(),
	)
}

func TestAuthenticationRefreshRevokesFamilyWhenAccessTokenIssuanceFails(
	t *testing.T,
) {
	accessTokenManager :=
		newAuthenticationIntegrationAccessTokenManager(t)

	accessTokenProvider :=
		&failSecondAuthenticationAccessTokenProvider{
			delegate: accessTokenManager,
		}

	app, databasePool, observedLogs :=
		newAuthenticationIntegrationApplicationWithAccessTokenProvider(
			t,
			accessTokenProvider,
		)

	router := app.Routes()

	_, loginResponse :=
		registerAndLoginAuthenticationRefreshUser(
			t,
			router,
			"refresh-token-failure@example.com",
		)

	if accessTokenProvider.issueCalls != 1 {
		t.Fatalf(
			"access-token issue calls after login = %d, want 1",
			accessTokenProvider.issueCalls,
		)
	}

	currentRefreshToken, err :=
		session.ParseRefreshToken(
			loginResponse.RefreshToken,
		)
	if err != nil {
		t.Fatalf(
			"parse login refresh token: %v",
			err,
		)
	}

	currentDigest, err := currentRefreshToken.Digest()
	if err != nil {
		t.Fatalf(
			"digest login refresh token: %v",
			err,
		)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		authIntegrationTimeout,
	)
	defer cancelQuery()

	var tokenFamilyID string

	err = databasePool.QueryRow(
		queryContext,
		`
			SELECT token_family_id::text
			FROM sessions
			WHERE refresh_token_hash = $1
		`,
		currentDigest.Bytes(),
	).Scan(&tokenFamilyID)
	if err != nil {
		t.Fatalf(
			"read refresh token family: %v",
			err,
		)
	}

	refreshRecorder := performAuthenticationRefreshRequest(
		t,
		router,
		loginResponse.RefreshToken,
		loginResponse.CSRFToken,
	)

	errorResponse := decodeAuthenticationRefreshError(
		t,
		refreshRecorder,
		http.StatusServiceUnavailable,
	)

	if errorResponse.Error.Code !=
		"authentication_unavailable" {
		t.Fatalf(
			"refresh error code = %q, want authentication_unavailable",
			errorResponse.Error.Code,
		)
	}

	if accessTokenProvider.issueCalls != 2 {
		t.Fatalf(
			"access-token issue calls after refresh = %d, want 2",
			accessTokenProvider.issueCalls,
		)
	}

	responseBody := refreshRecorder.Body.String()

	if strings.Contains(
		responseBody,
		errAuthenticationRefreshAccessTokenIssue.Error(),
	) {
		t.Fatal(
			"refresh response exposed the access-token issuance error",
		)
	}

	if strings.Contains(responseBody, "accessToken") ||
		strings.Contains(responseBody, "refreshToken") {
		t.Fatal(
			"failed refresh response exposed token fields",
		)
	}

	var (
		familyRowCount       int
		activeFamilyRowCount int
		revokedFamilyRows    int
	)

	err = databasePool.QueryRow(
		queryContext,
		`
			SELECT
				count(*),
				count(*) FILTER (
					WHERE revoked_at IS NULL
				),
				count(*) FILTER (
					WHERE revoked_at IS NOT NULL
				)
			FROM sessions
			WHERE token_family_id = $1::uuid
		`,
		tokenFamilyID,
	).Scan(
		&familyRowCount,
		&activeFamilyRowCount,
		&revokedFamilyRows,
	)
	if err != nil {
		t.Fatalf(
			"count refresh family after issuance failure: %v",
			err,
		)
	}

	if familyRowCount != 2 {
		t.Fatalf(
			"family row count = %d, want 2",
			familyRowCount,
		)
	}

	if activeFamilyRowCount != 0 {
		t.Fatalf(
			"active family row count = %d, want 0",
			activeFamilyRowCount,
		)
	}

	if revokedFamilyRows != 2 {
		t.Fatalf(
			"revoked family row count = %d, want 2",
			revokedFamilyRows,
		)
	}

	var replacementDigest []byte

	err = databasePool.QueryRow(
		queryContext,
		`
			SELECT refresh_token_hash
			FROM sessions
			WHERE token_family_id = $1::uuid
			  AND refresh_token_hash <> $2
			ORDER BY created_at DESC
			LIMIT 1
		`,
		tokenFamilyID,
		currentDigest.Bytes(),
	).Scan(&replacementDigest)
	if err != nil {
		t.Fatalf(
			"read failed-refresh replacement digest: %v",
			err,
		)
	}

	assertAuthenticationRefreshLogsSafe(
		t,
		observedLogs,
		loginResponse.RefreshToken,
		"",
		currentDigest.Bytes(),
		replacementDigest,
	)

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
				"encode observed refresh failure log: %v",
				err,
			)
		}

		if strings.Contains(
			string(encodedEntry),
			errAuthenticationRefreshAccessTokenIssue.Error(),
		) {
			t.Fatal(
				"refresh logs exposed the access-token issuance error",
			)
		}
	}
}

func registerAndLoginAuthenticationRefreshUser(
	t *testing.T,
	router http.Handler,
	email string,
) (
	authenticationAccountResponse,
	authenticationLoginResponse,
) {
	t.Helper()

	registerRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/register",
		email,
		authIntegrationPassword,
	)

	if registerRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"registration status = %d, want %d",
			registerRecorder.Code,
			http.StatusCreated,
		)
	}

	var accountResponse authenticationAccountResponse

	if err := json.NewDecoder(
		registerRecorder.Body,
	).Decode(&accountResponse); err != nil {
		t.Fatalf(
			"decode registration response: %v",
			err,
		)
	}

	loginRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/login",
		email,
		authIntegrationPassword,
	)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf(
			"login status = %d, want %d",
			loginRecorder.Code,
			http.StatusOK,
		)
	}

	var loginResponse authenticationLoginResponse

	if err := json.NewDecoder(
		loginRecorder.Body,
	).Decode(&loginResponse); err != nil {
		t.Fatalf(
			"decode login response: %v",
			err,
		)
	}

	cookieConfig := sessioncookie.NewConfig(false)

	loginResponse.RefreshToken = authenticationResponseCookieValue(
		t,
		loginRecorder,
		cookieConfig.RefreshCookieName(),
	)

	loginResponse.CSRFToken = authenticationResponseCookieValue(
		t,
		loginRecorder,
		cookieConfig.CSRFCookieName(),
	)

	return accountResponse, loginResponse
}

func performAuthenticationRefreshRequest(
	t *testing.T,
	router http.Handler,
	refreshToken string,
	csrfToken string,
) *httptest.ResponseRecorder {
	t.Helper()

	cookieConfig := sessioncookie.NewConfig(false)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{
		Name:  cookieConfig.RefreshCookieName(),
		Value: refreshToken,
	})
	request.AddCookie(&http.Cookie{
		Name:  cookieConfig.CSRFCookieName(),
		Value: csrfToken,
	})
	request.Header.Set(cookieConfig.CSRFHeaderName(), csrfToken)
	request.Header.Set("User-Agent", authIntegrationUserAgent)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func decodeAuthenticationRefreshError(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
) authenticationErrorResponse {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf(
			"refresh status = %d, want %d",
			recorder.Code,
			wantStatus,
		)
	}

	var errorResponse authenticationErrorResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&errorResponse); err != nil {
		t.Fatalf(
			"decode refresh error: %v",
			err,
		)
	}

	if errorResponse.Error.RequestID == "" {
		t.Fatal(
			"refresh error did not include a request ID",
		)
	}

	return errorResponse
}

func assertAuthenticationRefreshLogsSafe(
	t *testing.T,
	observedLogs *observer.ObservedLogs,
	currentToken string,
	replacementToken string,
	currentDigest []byte,
	replacementDigest []byte,
) {
	t.Helper()

	sensitiveMarkers := []string{
		currentToken,
		replacementToken,
		hex.EncodeToString(currentDigest),
		hex.EncodeToString(replacementDigest),
		base64.StdEncoding.EncodeToString(
			currentDigest,
		),
		base64.StdEncoding.EncodeToString(
			replacementDigest,
		),
	}

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
				"encode observed refresh log: %v",
				err,
			)
		}

		logText := string(encodedEntry)

		for _, marker := range sensitiveMarkers {
			if marker != "" &&
				strings.Contains(logText, marker) {
				t.Fatal(
					"authentication refresh log exposed token material",
				)
			}
		}
	}
}

type failSecondAuthenticationAccessTokenProvider struct {
	delegate   session.AccessTokenProvider
	issueCalls int
}

func (provider *failSecondAuthenticationAccessTokenProvider) Issue(
	ctx context.Context,
	principal session.Principal,
) (session.AccessToken, error) {
	provider.issueCalls++

	if provider.issueCalls == 2 {
		return session.AccessToken{},
			errAuthenticationRefreshAccessTokenIssue
	}

	return provider.delegate.Issue(
		ctx,
		principal,
	)
}

func (provider *failSecondAuthenticationAccessTokenProvider) Verify(
	ctx context.Context,
	tokenValue string,
) (session.Principal, error) {
	return provider.delegate.Verify(
		ctx,
		tokenValue,
	)
}
