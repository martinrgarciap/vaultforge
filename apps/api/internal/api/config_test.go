package api

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/hashclient"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
)

const (
	testDatabaseURL = "postgres://vaultforge:test@localhost:5432/vaultforge_test?sslmode=disable"
	testRedisURL    = "redis://127.0.0.1:6379/1"
)

func TestLoadConfigUsesDefaults(t *testing.T) {
	setValidConfigEnvironment(t)

	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", testDatabaseURL)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Env != "development" {
		t.Errorf(
			"expected environment development, got %q",
			cfg.Env,
		)
	}

	if cfg.Addr != ":8080" {
		t.Errorf(
			"expected HTTP address :8080, got %q",
			cfg.Addr,
		)
	}

	if cfg.DatabaseURL != testDatabaseURL {
		t.Errorf(
			"expected database URL %q, got %q",
			testDatabaseURL,
			cfg.DatabaseURL,
		)
	}

	if cfg.Redis.DialTimeout() != 2*time.Second {
		t.Errorf(
			"Redis dial timeout = %v, want %v",
			cfg.Redis.DialTimeout(),
			2*time.Second,
		)
	}

	if cfg.Redis.ReadTimeout() != time.Second {
		t.Errorf(
			"Redis read timeout = %v, want %v",
			cfg.Redis.ReadTimeout(),
			time.Second,
		)
	}

	if cfg.Redis.WriteTimeout() != time.Second {
		t.Errorf(
			"Redis write timeout = %v, want %v",
			cfg.Redis.WriteTimeout(),
			time.Second,
		)
	}

	if cfg.Redis.PoolTimeout() != 2*time.Second {
		t.Errorf(
			"Redis pool timeout = %v, want %v",
			cfg.Redis.PoolTimeout(),
			2*time.Second,
		)
	}

	if cfg.HashService.Address() != hashclient.DefaultAddress {
		t.Errorf(
			"hash service address = %q, want %q",
			cfg.HashService.Address(),
			hashclient.DefaultAddress,
		)
	}

	if cfg.HashService.DialTimeout() != hashclient.DefaultDialTimeout {
		t.Errorf(
			"hash service dial timeout = %v, want %v",
			cfg.HashService.DialTimeout(),
			hashclient.DefaultDialTimeout,
		)
	}

	if cfg.HashService.RequestTimeout() != hashclient.DefaultRequestTimeout {
		t.Errorf(
			"hash service request timeout = %v, want %v",
			cfg.HashService.RequestTimeout(),
			hashclient.DefaultRequestTimeout,
		)
	}

	if cfg.Tokens.Issuer() !=
		defaultAccessTokenIssuer {
		t.Errorf(
			"token issuer = %q, want %q",
			cfg.Tokens.Issuer(),
			defaultAccessTokenIssuer,
		)
	}

	if cfg.Tokens.Audience() !=
		defaultAccessTokenAudience {
		t.Errorf(
			"token audience = %q, want %q",
			cfg.Tokens.Audience(),
			defaultAccessTokenAudience,
		)
	}

	if cfg.Tokens.KeyID() !=
		defaultAccessTokenKeyID {
		t.Errorf(
			"token key ID = %q, want %q",
			cfg.Tokens.KeyID(),
			defaultAccessTokenKeyID,
		)
	}

	if cfg.Tokens.Lifetimes().
		AccessTokenTTL() !=
		session.DefaultAccessTokenTTL {
		t.Errorf(
			"access-token TTL = %v, want %v",
			cfg.Tokens.Lifetimes().
				AccessTokenTTL(),
			session.DefaultAccessTokenTTL,
		)
	}

	if cfg.Tokens.Lifetimes().
		RefreshTokenTTL() !=
		session.DefaultRefreshTokenTTL {
		t.Errorf(
			"refresh-token TTL = %v, want %v",
			cfg.Tokens.Lifetimes().
				RefreshTokenTTL(),
			session.DefaultRefreshTokenTTL,
		)
	}

	if cfg.Tokens.Lifetimes().
		ClockLeeway() !=
		session.DefaultClockLeeway {
		t.Errorf(
			"clock leeway = %v, want %v",
			cfg.Tokens.Lifetimes().
				ClockLeeway(),
			session.DefaultClockLeeway,
		)
	}
}

func TestLoadConfigUsesEnvironmentVariables(
	t *testing.T,
) {
	setValidConfigEnvironment(t)

	databaseURL := "postgres://vaultforge:test@localhost:5433/custom_test?sslmode=disable"

	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6380/2")
	t.Setenv("REDIS_DIAL_TIMEOUT", "3s")
	t.Setenv("REDIS_READ_TIMEOUT", "1500ms")
	t.Setenv("REDIS_WRITE_TIMEOUT", "1250ms")
	t.Setenv("REDIS_POOL_TIMEOUT", "4s")
	t.Setenv("HASH_SERVICE_ADDR", "127.0.0.1:50052")
	t.Setenv("HASH_SERVICE_DIAL_TIMEOUT", "4s")
	t.Setenv("HASH_SERVICE_TIMEOUT", "2500ms")
	t.Setenv(
		"ACCESS_TOKEN_ISSUER",
		"custom-issuer",
	)
	t.Setenv(
		"ACCESS_TOKEN_AUDIENCE",
		"custom-audience",
	)
	t.Setenv(
		"ACCESS_TOKEN_KEY_ID",
		"custom-ed25519-v2",
	)
	t.Setenv(
		"ACCESS_TOKEN_TTL",
		"15m",
	)
	t.Setenv(
		"REFRESH_TOKEN_TTL",
		"336h",
	)
	t.Setenv(
		"ACCESS_TOKEN_CLOCK_LEEWAY",
		"45s",
	)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Env != "test" {
		t.Errorf(
			"expected environment test, got %q",
			cfg.Env,
		)
	}

	if cfg.Addr != ":9090" {
		t.Errorf(
			"expected HTTP address :9090, got %q",
			cfg.Addr,
		)
	}

	if cfg.DatabaseURL != databaseURL {
		t.Errorf(
			"expected database URL %q, got %q",
			databaseURL,
			cfg.DatabaseURL,
		)
	}

	if cfg.Redis.DialTimeout() != 3*time.Second {
		t.Errorf(
			"Redis dial timeout = %v",
			cfg.Redis.DialTimeout(),
		)
	}

	if cfg.Redis.ReadTimeout() != 1500*time.Millisecond {
		t.Errorf(
			"Redis read timeout = %v",
			cfg.Redis.ReadTimeout(),
		)
	}

	if cfg.Redis.WriteTimeout() != 1250*time.Millisecond {
		t.Errorf(
			"Redis write timeout = %v",
			cfg.Redis.WriteTimeout(),
		)
	}

	if cfg.Redis.PoolTimeout() != 4*time.Second {
		t.Errorf(
			"Redis pool timeout = %v",
			cfg.Redis.PoolTimeout(),
		)
	}

	if cfg.HashService.Address() != "127.0.0.1:50052" {
		t.Errorf(
			"hash service address = %q",
			cfg.HashService.Address(),
		)
	}

	if cfg.HashService.DialTimeout() != 4*time.Second {
		t.Errorf(
			"hash service dial timeout = %v",
			cfg.HashService.DialTimeout(),
		)
	}

	if cfg.HashService.RequestTimeout() != 2500*time.Millisecond {
		t.Errorf(
			"hash service request timeout = %v",
			cfg.HashService.RequestTimeout(),
		)
	}

	if cfg.Tokens.Issuer() !=
		"custom-issuer" {
		t.Errorf(
			"token issuer = %q",
			cfg.Tokens.Issuer(),
		)
	}

	if cfg.Tokens.Audience() !=
		"custom-audience" {
		t.Errorf(
			"token audience = %q",
			cfg.Tokens.Audience(),
		)
	}

	if cfg.Tokens.KeyID() !=
		"custom-ed25519-v2" {
		t.Errorf(
			"token key ID = %q",
			cfg.Tokens.KeyID(),
		)
	}

	if cfg.Tokens.Lifetimes().
		AccessTokenTTL() !=
		15*time.Minute {
		t.Errorf(
			"access-token TTL = %v",
			cfg.Tokens.Lifetimes().
				AccessTokenTTL(),
		)
	}

	if cfg.Tokens.Lifetimes().
		RefreshTokenTTL() !=
		336*time.Hour {
		t.Errorf(
			"refresh-token TTL = %v",
			cfg.Tokens.Lifetimes().
				RefreshTokenTTL(),
		)
	}

	if cfg.Tokens.Lifetimes().
		ClockLeeway() !=
		45*time.Second {
		t.Errorf(
			"clock leeway = %v",
			cfg.Tokens.Lifetimes().
				ClockLeeway(),
		)
	}
}

func TestLoadConfigRejectsInvalidHashServiceConfig(t *testing.T) {
	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "malformed address",
			key:   "HASH_SERVICE_ADDR",
			value: "http://127.0.0.1:50051",
		},
		{
			name:  "address without port",
			key:   "HASH_SERVICE_ADDR",
			value: "127.0.0.1",
		},
		{
			name:  "malformed dial timeout",
			key:   "HASH_SERVICE_DIAL_TIMEOUT",
			value: "invalid",
		},
		{
			name:  "zero dial timeout",
			key:   "HASH_SERVICE_DIAL_TIMEOUT",
			value: "0s",
		},
		{
			name:  "malformed request timeout",
			key:   "HASH_SERVICE_TIMEOUT",
			value: "invalid",
		},
		{
			name:  "zero request timeout",
			key:   "HASH_SERVICE_TIMEOUT",
			value: "0s",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setValidConfigEnvironment(t)

			t.Setenv("DATABASE_URL", testDatabaseURL)
			t.Setenv(testCase.key, testCase.value)

			_, err := LoadConfig()
			if err == nil {
				t.Fatal(
					"expected invalid hash service config to return an error",
				)
			}
		})
	}
}

func TestLoadConfigAcceptsCustomPasswordServiceConfig(t *testing.T) {
	setValidConfigEnvironment(t)

	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("PASSWORD_SERVICE_ADDR", "password-service:50053")
	t.Setenv("PASSWORD_SERVICE_DIAL_TIMEOUT", "4s")
	t.Setenv("PASSWORD_SERVICE_TIMEOUT", "2500ms")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.PasswordService.Address() != "password-service:50053" {
		t.Errorf(
			"password service address = %q",
			cfg.PasswordService.Address(),
		)
	}

	if cfg.PasswordService.DialTimeout() != 4*time.Second {
		t.Errorf(
			"password service dial timeout = %v",
			cfg.PasswordService.DialTimeout(),
		)
	}

	if cfg.PasswordService.RequestTimeout() != 2500*time.Millisecond {
		t.Errorf(
			"password service request timeout = %v",
			cfg.PasswordService.RequestTimeout(),
		)
	}
}

func TestLoadConfigRejectsInvalidPasswordServiceConfig(t *testing.T) {
	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "malformed address",
			key:   "PASSWORD_SERVICE_ADDR",
			value: "http://127.0.0.1:50053",
		},
		{
			name:  "address without port",
			key:   "PASSWORD_SERVICE_ADDR",
			value: "127.0.0.1",
		},
		{
			name:  "malformed dial timeout",
			key:   "PASSWORD_SERVICE_DIAL_TIMEOUT",
			value: "invalid",
		},
		{
			name:  "zero dial timeout",
			key:   "PASSWORD_SERVICE_DIAL_TIMEOUT",
			value: "0s",
		},
		{
			name:  "malformed request timeout",
			key:   "PASSWORD_SERVICE_TIMEOUT",
			value: "invalid",
		},
		{
			name:  "zero request timeout",
			key:   "PASSWORD_SERVICE_TIMEOUT",
			value: "0s",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setValidConfigEnvironment(t)

			t.Setenv("DATABASE_URL", testDatabaseURL)
			t.Setenv(testCase.key, testCase.value)

			_, err := LoadConfig()
			if err == nil {
				t.Fatal(
					"expected invalid password service config to return an error",
				)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidEnvironment(
	t *testing.T,
) {
	setValidConfigEnvironment(t)

	t.Setenv("APP_ENV", "invalid")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", testDatabaseURL)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal(
			"expected invalid environment to return an error",
		)
	}
}

func TestLoadConfigRejectsInvalidPasswordToolsRateLimitConfig(t *testing.T) {
	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "malformed request count",
			key:   "RATE_LIMIT_PASSWORD_TOOLS_REQUESTS",
			value: "invalid",
		},
		{
			name:  "zero request count",
			key:   "RATE_LIMIT_PASSWORD_TOOLS_REQUESTS",
			value: "0",
		},
		{
			name:  "malformed window",
			key:   "RATE_LIMIT_PASSWORD_TOOLS_WINDOW",
			value: "invalid",
		},
		{
			name:  "zero window",
			key:   "RATE_LIMIT_PASSWORD_TOOLS_WINDOW",
			value: "0s",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setValidConfigEnvironment(t)

			t.Setenv("DATABASE_URL", testDatabaseURL)
			t.Setenv(testCase.key, testCase.value)

			_, err := LoadConfig()
			if err == nil {
				t.Fatal("expected invalid password-tools rate-limit config to return an error")
			}
		})
	}
}

func TestLoadConfigRejectsMissingDatabaseURL(
	t *testing.T,
) {
	setValidConfigEnvironment(t)

	t.Setenv("APP_ENV", "development")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal(
			"expected missing database URL to return an error",
		)
	}
}

func TestLoadConfigRejectsWhitespaceDatabaseURL(
	t *testing.T,
) {
	setValidConfigEnvironment(t)

	t.Setenv("APP_ENV", "development")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", "   ")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal(
			"expected whitespace database URL to return an error",
		)
	}
}

func TestLoadConfigRejectsMissingRedisURL(t *testing.T) {
	setValidConfigEnvironment(t)

	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("REDIS_URL", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected missing Redis URL to return an error")
	}
}

func TestLoadConfigRejectsMalformedRedisURL(t *testing.T) {
	const secretMarker = "synthetic-redis-password-marker"

	setValidConfigEnvironment(t)

	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv(
		"REDIS_URL",
		"not-a-redis-url://:"+secretMarker,
	)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected malformed Redis URL to return an error")
	}

	if strings.Contains(err.Error(), secretMarker) {
		t.Fatal("configuration error exposed the Redis URL")
	}
}

func TestLoadConfigRejectsInvalidRedisTimeouts(t *testing.T) {
	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "malformed dial timeout",
			key:   "REDIS_DIAL_TIMEOUT",
			value: "invalid",
		},
		{
			name:  "zero read timeout",
			key:   "REDIS_READ_TIMEOUT",
			value: "0s",
		},
		{
			name:  "negative write timeout",
			key:   "REDIS_WRITE_TIMEOUT",
			value: "-1s",
		},
		{
			name:  "zero pool timeout",
			key:   "REDIS_POOL_TIMEOUT",
			value: "0s",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setValidConfigEnvironment(t)

			t.Setenv("DATABASE_URL", testDatabaseURL)
			t.Setenv(testCase.key, testCase.value)

			_, err := LoadConfig()
			if err == nil {
				t.Fatal(
					"expected invalid Redis timeout to return an error",
				)
			}
		})
	}
}

func TestLoadConfigRejectsMissingSigningSeed(
	t *testing.T,
) {
	setValidConfigEnvironment(t)

	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv(
		"ACCESS_TOKEN_ED25519_SEED_BASE64",
		"",
	)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal(
			"expected missing signing seed to return an error",
		)
	}
}

func TestLoadConfigRejectsMalformedSigningSeed(
	t *testing.T,
) {
	const secretMarker = "synthetic-invalid-signing-seed-marker"

	setValidConfigEnvironment(t)

	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv(
		"ACCESS_TOKEN_ED25519_SEED_BASE64",
		secretMarker,
	)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal(
			"expected malformed signing seed to return an error",
		)
	}

	if strings.Contains(
		err.Error(),
		secretMarker,
	) {
		t.Fatal(
			"configuration error exposed the signing seed",
		)
	}
}

func TestLoadConfigRejectsIncorrectSigningSeedLength(
	t *testing.T,
) {
	setValidConfigEnvironment(t)

	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv(
		"ACCESS_TOKEN_ED25519_SEED_BASE64",
		base64.StdEncoding.EncodeToString(
			bytes.Repeat(
				[]byte{0x31},
				ed25519.SeedSize-1,
			),
		),
	)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal(
			"expected incorrect signing seed length to return an error",
		)
	}
}

func TestLoadConfigRejectsInvalidTokenDurations(
	t *testing.T,
) {
	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "malformed access-token lifetime",
			key:   "ACCESS_TOKEN_TTL",
			value: "not-a-duration",
		},
		{
			name:  "zero access-token lifetime",
			key:   "ACCESS_TOKEN_TTL",
			value: "0s",
		},
		{
			name:  "refresh lifetime too short",
			key:   "REFRESH_TOKEN_TTL",
			value: "5m",
		},
		{
			name:  "negative clock leeway",
			key:   "ACCESS_TOKEN_CLOCK_LEEWAY",
			value: "-1s",
		},
		{
			name:  "clock leeway too long",
			key:   "ACCESS_TOKEN_CLOCK_LEEWAY",
			value: "10m",
		},
	}

	for _, testCase := range testCases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				setValidConfigEnvironment(t)

				t.Setenv(
					"DATABASE_URL",
					testDatabaseURL,
				)
				t.Setenv(
					testCase.key,
					testCase.value,
				)

				_, err := LoadConfig()
				if err == nil {
					t.Fatal(
						"expected invalid token duration to return an error",
					)
				}
			},
		)
	}
}

func TestLoadConfigRejectsInvalidTokenIdentity(
	t *testing.T,
) {
	setValidConfigEnvironment(t)

	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv(
		"ACCESS_TOKEN_ISSUER",
		"invalid issuer",
	)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal(
			"expected invalid token identity to return an error",
		)
	}
}

func setValidConfigEnvironment(t *testing.T) {
	t.Helper()

	t.Setenv("HTTP_REQUEST_TIMEOUT", "")
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "")
	t.Setenv("HTTP_READ_TIMEOUT", "")
	t.Setenv("HTTP_WRITE_TIMEOUT", "")
	t.Setenv("HTTP_IDLE_TIMEOUT", "")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "")

	t.Setenv(
		"RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64",
		base64.StdEncoding.EncodeToString(
			bytes.Repeat(
				[]byte{0x24},
				32,
			),
		),
	)

	t.Setenv("RATE_LIMIT_REGISTRATION_REQUESTS", "")
	t.Setenv("RATE_LIMIT_REGISTRATION_WINDOW", "")
	t.Setenv("RATE_LIMIT_LOGIN_REQUESTS", "")
	t.Setenv("RATE_LIMIT_LOGIN_WINDOW", "")
	t.Setenv("RATE_LIMIT_REFRESH_REQUESTS", "")
	t.Setenv("RATE_LIMIT_REFRESH_WINDOW", "")
	t.Setenv("RATE_LIMIT_MUTATION_REQUESTS", "")
	t.Setenv("RATE_LIMIT_MUTATION_WINDOW", "")
	t.Setenv("RATE_LIMIT_PASSWORD_TOOLS_REQUESTS", "")
	t.Setenv("RATE_LIMIT_PASSWORD_TOOLS_WINDOW", "")
	t.Setenv("LOGIN_FAILURE_LIMIT", "")
	t.Setenv("LOGIN_FAILURE_WINDOW", "")
	t.Setenv("LOGIN_LOCKOUT_DURATION", "")

	t.Setenv("REDIS_URL", testRedisURL)
	t.Setenv("REDIS_DIAL_TIMEOUT", "")
	t.Setenv("REDIS_READ_TIMEOUT", "")
	t.Setenv("REDIS_WRITE_TIMEOUT", "")
	t.Setenv("REDIS_POOL_TIMEOUT", "")

	t.Setenv("HASH_SERVICE_ADDR", "")
	t.Setenv("HASH_SERVICE_DIAL_TIMEOUT", "")
	t.Setenv("HASH_SERVICE_TIMEOUT", "")

	t.Setenv("PASSWORD_SERVICE_ADDR", "")
	t.Setenv("PASSWORD_SERVICE_DIAL_TIMEOUT", "")
	t.Setenv("PASSWORD_SERVICE_TIMEOUT", "")

	t.Setenv("ACCESS_TOKEN_ISSUER", "")
	t.Setenv("ACCESS_TOKEN_AUDIENCE", "")
	t.Setenv("ACCESS_TOKEN_KEY_ID", "")
	t.Setenv("ACCESS_TOKEN_TTL", "")
	t.Setenv("REFRESH_TOKEN_TTL", "")
	t.Setenv(
		"ACCESS_TOKEN_CLOCK_LEEWAY",
		"",
	)

	t.Setenv(
		"ACCESS_TOKEN_ED25519_SEED_BASE64",
		base64.StdEncoding.EncodeToString(
			bytes.Repeat(
				[]byte{0x42},
				ed25519.SeedSize,
			),
		),
	)
}
