package store_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/db"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	authIntegrationPassword  = "correct horse battery staple"
	authIntegrationUserAgent = "VaultForge Authentication Integration Test"
	authIntegrationTimeout   = 5 * time.Second
)

var errAuthenticationIntegrationAccessTokenIssue = errors.New(
	"synthetic access-token issuance failure",
)

type authenticationErrorResponse struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

type authenticationAccountResponse struct {
	User auth.Account `json:"user"`
}

type authenticationLoginResponse struct {
	User                  auth.Account `json:"user"`
	TokenType             string       `json:"tokenType"`
	AccessToken           string       `json:"accessToken"`
	AccessTokenExpiresAt  time.Time    `json:"accessTokenExpiresAt"`
	RefreshToken          string       `json:"refreshToken"`
	RefreshTokenExpiresAt time.Time    `json:"refreshTokenExpiresAt"`
}

func TestAuthenticationHTTPFlowIntegration(
	t *testing.T,
) {
	app, databasePool, observedLogs :=
		newAuthenticationIntegrationApplication(t)

	router := app.Routes()

	registerRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/register",
		"  Integration.User@Example.COM  ",
		authIntegrationPassword,
	)

	if registerRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"registration status = %d, want %d; body = %s",
			registerRecorder.Code,
			http.StatusCreated,
			registerRecorder.Body.String(),
		)
	}

	var registerResponse authenticationAccountResponse

	if err := json.NewDecoder(
		registerRecorder.Body,
	).Decode(&registerResponse); err != nil {
		t.Fatalf(
			"decode registration response: %v",
			err,
		)
	}

	const normalizedEmail = "integration.user@example.com"

	if registerResponse.User.Email != normalizedEmail {
		t.Fatalf(
			"registered email = %q, want %q",
			registerResponse.User.Email,
			normalizedEmail,
		)
	}

	if registerResponse.User.ID == "" {
		t.Fatal(
			"registration response did not include a user ID",
		)
	}

	var (
		storedPasswordHash      string
		storedPasswordAlgorithm string
		storedStatus            string
	)

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		authIntegrationTimeout,
	)
	defer cancelQuery()

	err := databasePool.QueryRow(
		queryContext,
		`
			SELECT
				password_hash,
				password_algorithm,
				status
			FROM users
			WHERE email = $1
		`,
		normalizedEmail,
	).Scan(
		&storedPasswordHash,
		&storedPasswordAlgorithm,
		&storedStatus,
	)
	if err != nil {
		t.Fatalf(
			"read registered user: %v",
			err,
		)
	}

	if storedPasswordHash == authIntegrationPassword {
		t.Fatal(
			"database stored the plaintext password",
		)
	}

	if strings.Contains(
		storedPasswordHash,
		authIntegrationPassword,
	) {
		t.Fatal(
			"stored password hash contains the plaintext password",
		)
	}

	if !strings.HasPrefix(
		storedPasswordHash,
		"$argon2id$",
	) {
		t.Fatal(
			"database did not store an Argon2id encoded hash",
		)
	}

	if storedPasswordAlgorithm !=
		auth.AlgorithmArgon2id {
		t.Fatalf(
			"password algorithm = %q, want %q",
			storedPasswordAlgorithm,
			auth.AlgorithmArgon2id,
		)
	}

	if storedStatus != "active" {
		t.Fatalf(
			"user status = %q, want active",
			storedStatus,
		)
	}

	registerBody := registerRecorder.Body.String()

	if strings.Contains(
		registerBody,
		authIntegrationPassword,
	) ||
		strings.Contains(
			registerBody,
			storedPasswordHash,
		) ||
		strings.Contains(
			registerBody,
			"password_hash",
		) {
		t.Fatal(
			"registration response exposed password material",
		)
	}

	loginRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/login",
		"INTEGRATION.USER@EXAMPLE.COM",
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

	if loginResponse.User.ID !=
		registerResponse.User.ID {
		t.Fatalf(
			"login user ID = %q, want %q",
			loginResponse.User.ID,
			registerResponse.User.ID,
		)
	}

	if loginResponse.User.Email != normalizedEmail {
		t.Fatalf(
			"login email = %q, want %q",
			loginResponse.User.Email,
			normalizedEmail,
		)
	}

	if loginResponse.TokenType != "Bearer" {
		t.Fatalf(
			"token type = %q, want Bearer",
			loginResponse.TokenType,
		)
	}

	if loginResponse.AccessToken == "" {
		t.Fatal(
			"login response did not include an access token",
		)
	}

	if loginResponse.RefreshToken == "" {
		t.Fatal(
			"login response did not include a refresh token",
		)
	}

	if loginResponse.AccessTokenExpiresAt.IsZero() {
		t.Fatal(
			"login response did not include an access-token expiration",
		)
	}

	if loginResponse.RefreshTokenExpiresAt.IsZero() {
		t.Fatal(
			"login response did not include a refresh-token expiration",
		)
	}

	if !loginResponse.RefreshTokenExpiresAt.After(
		loginResponse.AccessTokenExpiresAt,
	) {
		t.Fatal(
			"refresh token should expire after the access token",
		)
	}

	parsedRefreshToken, err :=
		session.ParseRefreshToken(
			loginResponse.RefreshToken,
		)
	if err != nil {
		t.Fatalf(
			"parse login refresh token: %v",
			err,
		)
	}

	refreshTokenDigest, err :=
		parsedRefreshToken.Digest()
	if err != nil {
		t.Fatalf(
			"digest login refresh token: %v",
			err,
		)
	}

	sessionQueryContext, cancelSessionQuery :=
		context.WithTimeout(
			context.Background(),
			authIntegrationTimeout,
		)
	defer cancelSessionQuery()

	var (
		storedRefreshTokenHash []byte
		storedTokenFamilyID    string
		storedSessionExpiresAt time.Time
		storedSessionRevoked   bool
		storedUserAgent        string
	)

	err = databasePool.QueryRow(
		sessionQueryContext,
		`
		SELECT
			refresh_token_hash,
			token_family_id::text,
			expires_at,
			revoked_at IS NOT NULL,
			COALESCE(user_agent, '')
		FROM sessions
		WHERE user_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`,
		registerResponse.User.ID,
	).Scan(
		&storedRefreshTokenHash,
		&storedTokenFamilyID,
		&storedSessionExpiresAt,
		&storedSessionRevoked,
		&storedUserAgent,
	)
	if err != nil {
		t.Fatalf(
			"read login session: %v",
			err,
		)
	}

	if !bytes.Equal(
		storedRefreshTokenHash,
		refreshTokenDigest.Bytes(),
	) {
		t.Fatal(
			"database refresh-token digest did not match the returned token",
		)
	}

	if bytes.Equal(
		storedRefreshTokenHash,
		[]byte(loginResponse.RefreshToken),
	) {
		t.Fatal(
			"database stored the plaintext refresh token",
		)
	}

	if storedTokenFamilyID == "" {
		t.Fatal(
			"database session did not include a token-family ID",
		)
	}

	if storedSessionRevoked {
		t.Fatal(
			"new login session should be active",
		)
	}

	if storedUserAgent != authIntegrationUserAgent {
		t.Fatalf(
			"stored user agent = %q, want %q",
			storedUserAgent,
			authIntegrationUserAgent,
		)
	}

	if !storedSessionExpiresAt.Equal(
		loginResponse.RefreshTokenExpiresAt,
	) {
		t.Fatalf(
			"stored refresh expiration = %v, response expiration = %v",
			storedSessionExpiresAt,
			loginResponse.RefreshTokenExpiresAt,
		)
	}

	accessTokenVerifier :=
		newAuthenticationIntegrationAccessTokenManager(
			t,
		)

	principal, err := accessTokenVerifier.Verify(
		context.Background(),
		loginResponse.AccessToken,
	)
	if err != nil {
		t.Fatalf(
			"verify login access token: %v",
			err,
		)
	}

	if principal.UserID !=
		registerResponse.User.ID {
		t.Fatalf(
			"access-token user ID = %q, want %q",
			principal.UserID,
			registerResponse.User.ID,
		)
	}

	if principal.SessionID != storedTokenFamilyID {
		t.Fatalf(
			"access-token session ID = %q, want stored token-family ID",
			principal.SessionID,
		)
	}

	incorrectPasswordRecorder :=
		performAuthenticationRequest(
			t,
			router,
			"/v1/auth/login",
			normalizedEmail,
			"incorrect horse battery staple",
		)

	unknownEmailRecorder :=
		performAuthenticationRequest(
			t,
			router,
			"/v1/auth/login",
			"unknown-user@example.com",
			authIntegrationPassword,
		)

	incorrectPasswordError :=
		decodeAuthenticationError(
			t,
			incorrectPasswordRecorder,
			http.StatusUnauthorized,
		)

	unknownEmailError :=
		decodeAuthenticationError(
			t,
			unknownEmailRecorder,
			http.StatusUnauthorized,
		)

	if incorrectPasswordError.Error.Code !=
		"invalid_credentials" {
		t.Fatalf(
			"incorrect password code = %q, want invalid_credentials",
			incorrectPasswordError.Error.Code,
		)
	}

	if unknownEmailError.Error.Code !=
		incorrectPasswordError.Error.Code {
		t.Fatalf(
			"unknown email code = %q, incorrect password code = %q",
			unknownEmailError.Error.Code,
			incorrectPasswordError.Error.Code,
		)
	}

	sessionCountContext, cancelSessionCount :=
		context.WithTimeout(
			context.Background(),
			authIntegrationTimeout,
		)
	defer cancelSessionCount()

	var storedSessionCount int

	err = databasePool.QueryRow(
		sessionCountContext,
		`
		SELECT count(*)
		FROM sessions
		WHERE user_id = $1::uuid
	`,
		registerResponse.User.ID,
	).Scan(&storedSessionCount)
	if err != nil {
		t.Fatalf(
			"count authentication sessions: %v",
			err,
		)
	}

	if storedSessionCount != 1 {
		t.Fatalf(
			"stored session count = %d, want 1",
			storedSessionCount,
		)
	}

	if unknownEmailError.Error.Message !=
		incorrectPasswordError.Error.Message {
		t.Fatalf(
			"unknown email and incorrect password messages differ",
		)
	}

	assertAuthenticationLogsSafe(
		t,
		observedLogs,
		authIntegrationPassword,
		storedPasswordHash,
	)
}

func TestAuthenticationLoginRevokesSessionWhenAccessTokenIssuanceFails(
	t *testing.T,
) {
	app, databasePool, _ :=
		newAuthenticationIntegrationApplicationWithAccessTokenProvider(
			t,
			&failingAuthenticationAccessTokenProvider{},
		)

	router := app.Routes()

	registerRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/register",
		"token-failure@example.com",
		authIntegrationPassword,
	)

	if registerRecorder.Code !=
		http.StatusCreated {
		t.Fatalf(
			"registration status = %d, want %d",
			registerRecorder.Code,
			http.StatusCreated,
		)
	}

	var registerResponse authenticationAccountResponse

	if err := json.NewDecoder(
		registerRecorder.Body,
	).Decode(&registerResponse); err != nil {
		t.Fatalf(
			"decode registration response: %v",
			err,
		)
	}

	loginRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/login",
		"token-failure@example.com",
		authIntegrationPassword,
	)

	errorResponse := decodeAuthenticationError(
		t,
		loginRecorder,
		http.StatusServiceUnavailable,
	)

	if errorResponse.Error.Code !=
		"authentication_unavailable" {
		t.Fatalf(
			"login error code = %q, want %q",
			errorResponse.Error.Code,
			"authentication_unavailable",
		)
	}

	responseBody := loginRecorder.Body.String()

	if strings.Contains(
		responseBody,
		errAuthenticationIntegrationAccessTokenIssue.
			Error(),
	) {
		t.Fatal(
			"login response exposed the token issuance error",
		)
	}

	if strings.Contains(
		responseBody,
		"accessToken",
	) ||
		strings.Contains(
			responseBody,
			"refreshToken",
		) {
		t.Fatal(
			"failed login response exposed token fields",
		)
	}

	queryContext, cancelQuery :=
		context.WithTimeout(
			context.Background(),
			authIntegrationTimeout,
		)
	defer cancelQuery()

	var (
		activeSessionCount  int64
		revokedSessionCount int64
	)

	err := databasePool.QueryRow(
		queryContext,
		`
			SELECT
				count(*) FILTER (
					WHERE revoked_at IS NULL
				),
				count(*) FILTER (
					WHERE revoked_at IS NOT NULL
				)
			FROM sessions
			WHERE user_id = $1::uuid
		`,
		registerResponse.User.ID,
	).Scan(
		&activeSessionCount,
		&revokedSessionCount,
	)
	if err != nil {
		t.Fatalf(
			"count sessions after token issuance failure: %v",
			err,
		)
	}

	if activeSessionCount != 0 {
		t.Fatalf(
			"active session count = %d, want 0",
			activeSessionCount,
		)
	}

	if revokedSessionCount != 1 {
		t.Fatalf(
			"revoked session count = %d, want 1",
			revokedSessionCount,
		)
	}
}

func TestAuthenticationDuplicateRegistrationIntegration(
	t *testing.T,
) {
	app, _, _ :=
		newAuthenticationIntegrationApplication(t)

	router := app.Routes()

	firstRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/register",
		"duplicate-auth@example.com",
		authIntegrationPassword,
	)

	if firstRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"first registration status = %d, want %d",
			firstRecorder.Code,
			http.StatusCreated,
		)
	}

	secondRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/register",
		"  DUPLICATE-AUTH@EXAMPLE.COM  ",
		authIntegrationPassword,
	)

	errorResponse := decodeAuthenticationError(
		t,
		secondRecorder,
		http.StatusConflict,
	)

	if errorResponse.Error.Code !=
		"email_unavailable" {
		t.Fatalf(
			"duplicate registration code = %q, want email_unavailable",
			errorResponse.Error.Code,
		)
	}

	responseBody := secondRecorder.Body.String()

	if strings.Contains(
		responseBody,
		"users_email_unique",
	) ||
		strings.Contains(
			responseBody,
			"duplicate key",
		) {
		t.Fatal(
			"duplicate registration exposed PostgreSQL details",
		)
	}
}

func TestAuthenticationDisabledUserIntegration(
	t *testing.T,
) {
	app, databasePool, _ :=
		newAuthenticationIntegrationApplication(t)

	router := app.Routes()

	const email = "disabled-auth@example.com"

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

	updateContext, cancelUpdate := context.WithTimeout(
		context.Background(),
		authIntegrationTimeout,
	)
	defer cancelUpdate()

	commandTag, err := databasePool.Exec(
		updateContext,
		`
			UPDATE users
			SET
				status = 'disabled',
				updated_at = now()
			WHERE email = $1
		`,
		email,
	)
	if err != nil {
		t.Fatalf(
			"disable user: %v",
			err,
		)
	}

	if commandTag.RowsAffected() != 1 {
		t.Fatalf(
			"disabled rows = %d, want 1",
			commandTag.RowsAffected(),
		)
	}

	loginRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/login",
		email,
		authIntegrationPassword,
	)

	errorResponse := decodeAuthenticationError(
		t,
		loginRecorder,
		http.StatusUnauthorized,
	)

	if errorResponse.Error.Code !=
		"invalid_credentials" {
		t.Fatalf(
			"disabled login code = %q, want invalid_credentials",
			errorResponse.Error.Code,
		)
	}

	if strings.Contains(
		loginRecorder.Body.String(),
		"disabled",
	) {
		t.Fatal(
			"login response revealed that the account is disabled",
		)
	}
}

func TestAuthenticationAndHealthRoutesIntegration(
	t *testing.T,
) {
	app, _, _ :=
		newAuthenticationIntegrationApplication(t)

	router := app.Routes()

	for _, path := range []string{
		"/health/live",
		"/health/ready",
	} {
		request := httptest.NewRequest(
			http.MethodGet,
			path,
			nil,
		)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf(
				"%s status = %d, want %d",
				path,
				recorder.Code,
				http.StatusOK,
			)
		}
	}

	registerRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/register",
		"health-auth@example.com",
		authIntegrationPassword,
	)

	if registerRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"auth route status = %d, want %d",
			registerRecorder.Code,
			http.StatusCreated,
		)
	}
}

func newAuthenticationIntegrationApplication(
	t *testing.T,
) (
	*api.Application,
	*pgxpool.Pool,
	*observer.ObservedLogs,
) {
	t.Helper()

	return newAuthenticationIntegrationApplicationWithAccessTokenProvider(
		t,
		newAuthenticationIntegrationAccessTokenManager(
			t,
		),
	)
}

func newAuthenticationIntegrationApplicationWithAccessTokenProvider(
	t *testing.T,
	accessTokenProvider session.AccessTokenProvider,
) (
	*api.Application,
	*pgxpool.Pool,
	*observer.ObservedLogs,
) {
	t.Helper()

	testDatabaseURL := strings.TrimSpace(
		os.Getenv("TEST_DATABASE_URL"),
	)
	if testDatabaseURL == "" {
		t.Skip(
			"TEST_DATABASE_URL is not configured",
		)
	}

	databaseContext, cancelDatabase :=
		context.WithTimeout(
			context.Background(),
			authIntegrationTimeout,
		)
	defer cancelDatabase()

	databasePool, err := db.New(
		databaseContext,
		testDatabaseURL,
	)
	if err != nil {
		t.Fatalf(
			"open authentication integration database: %v",
			err,
		)
	}

	t.Cleanup(databasePool.Close)

	resetAuthenticationIntegrationTables(
		t,
		databasePool,
	)

	logCore, observedLogs := observer.New(
		zap.DebugLevel,
	)
	logger := zap.New(logCore).Sugar()

	userStore := store.NewUserStore(
		databasePool,
	)

	passwordHasher :=
		auth.NewArgon2idHasher()

	authService := auth.NewService(
		userStore,
		passwordHasher,
	)

	sessionStore := store.NewSessionStore(
		databasePool,
	)

	refreshTokenGenerator :=
		session.NewRefreshTokenGenerator()

	sessionService := session.NewService(
		authService,
		sessionStore,
		refreshTokenGenerator,
		accessTokenProvider,
		session.DefaultTokenLifetimes(),
	)

	app := api.NewApplication(
		api.Config{
			Env:         "test",
			Addr:        ":8080",
			DatabaseURL: "postgres://test",
		},
		logger,
		databasePool,
		authService,
		sessionService,
	)

	return app, databasePool, observedLogs
}

func newAuthenticationIntegrationAccessTokenManager(
	t *testing.T,
) *session.AccessTokenManager {
	t.Helper()

	seed := bytes.Repeat(
		[]byte{0x53},
		ed25519.SeedSize,
	)

	privateKey := ed25519.NewKeyFromSeed(seed)

	manager, err := session.NewAccessTokenManager(
		"vaultforge-auth-integration",
		"vaultforge-auth-integration",
		"auth-integration-ed25519-v1",
		privateKey,
		session.DefaultTokenLifetimes(),
	)
	if err != nil {
		t.Fatalf(
			"create authentication integration access-token manager: %v",
			err,
		)
	}

	return manager
}

type failingAuthenticationAccessTokenProvider struct{}

func (*failingAuthenticationAccessTokenProvider) Issue(
	_ context.Context,
	_ session.Principal,
) (session.AccessToken, error) {
	return session.AccessToken{},
		errAuthenticationIntegrationAccessTokenIssue
}

func (*failingAuthenticationAccessTokenProvider) Verify(
	_ context.Context,
	_ string,
) (session.Principal, error) {
	return session.Principal{},
		session.ErrAccessTokenUnavailable
}

func resetAuthenticationIntegrationTables(
	t *testing.T,
	databasePool *pgxpool.Pool,
) {
	t.Helper()

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		authIntegrationTimeout,
	)
	defer cancelQuery()

	_, err := databasePool.Exec(
		queryContext,
		`
			TRUNCATE TABLE
				item_versions,
				vault_items,
				vaults,
				sessions,
				users
			CASCADE
		`,
	)
	if err != nil {
		t.Fatalf(
			"reset authentication integration tables: %v",
			err,
		)
	}
}

func performAuthenticationRequest(
	t *testing.T,
	router http.Handler,
	path string,
	email string,
	password string,
) *httptest.ResponseRecorder {
	t.Helper()

	requestBody, err := json.Marshal(
		struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}{
			Email:    email,
			Password: password,
		},
	)
	if err != nil {
		t.Fatalf(
			"encode authentication request: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		path,
		strings.NewReader(
			string(requestBody),
		),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	request.Header.Set(
		"User-Agent",
		authIntegrationUserAgent,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func decodeAuthenticationError(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
) authenticationErrorResponse {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			wantStatus,
			recorder.Body.String(),
		)
	}

	var errorResponse authenticationErrorResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&errorResponse); err != nil {
		t.Fatalf(
			"decode authentication error: %v",
			err,
		)
	}

	if errorResponse.Error.RequestID == "" {
		t.Fatal(
			"authentication error did not include a request ID",
		)
	}

	return errorResponse
}

func assertAuthenticationLogsSafe(
	t *testing.T,
	observedLogs *observer.ObservedLogs,
	password string,
	passwordHash string,
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
				"encode observed log entry: %v",
				err,
			)
		}

		logText := string(encodedEntry)

		if strings.Contains(
			logText,
			password,
		) {
			t.Fatal(
				"authentication log exposed a plaintext password",
			)
		}

		if strings.Contains(
			logText,
			passwordHash,
		) {
			t.Fatal(
				"authentication log exposed an encoded password hash",
			)
		}
	}
}
