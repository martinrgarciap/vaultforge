package store_test

import (
	"context"
	"encoding/json"
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
	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	authIntegrationPassword = "correct horse battery staple"
	authIntegrationTimeout  = 5 * time.Second
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
			"login status = %d, want %d; body = %s",
			loginRecorder.Code,
			http.StatusOK,
			loginRecorder.Body.String(),
		)
	}

	var loginResponse authenticationAccountResponse

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
	passwordHasher := auth.NewArgon2idHasher()
	authService := auth.NewService(
		userStore,
		passwordHasher,
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
	)

	return app, databasePool, observedLogs
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
