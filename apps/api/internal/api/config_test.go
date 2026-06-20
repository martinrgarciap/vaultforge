package api

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
)

const testDatabaseURL = "postgres://vaultforge:test@localhost:5432/vaultforge_test?sslmode=disable"

func TestLoadConfigUsesDefaults(t *testing.T) {
	setValidTokenEnvironment(t)

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
	setValidTokenEnvironment(t)

	databaseURL := "postgres://vaultforge:test@localhost:5433/custom_test?sslmode=disable"

	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_URL", databaseURL)
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

func TestLoadConfigRejectsInvalidEnvironment(
	t *testing.T,
) {
	setValidTokenEnvironment(t)

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

func TestLoadConfigRejectsMissingDatabaseURL(
	t *testing.T,
) {
	setValidTokenEnvironment(t)

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
	setValidTokenEnvironment(t)

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

func TestLoadConfigRejectsMissingSigningSeed(
	t *testing.T,
) {
	setValidTokenEnvironment(t)

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

	setValidTokenEnvironment(t)

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
	setValidTokenEnvironment(t)

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
				setValidTokenEnvironment(t)

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
	setValidTokenEnvironment(t)

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

func setValidTokenEnvironment(t *testing.T) {
	t.Helper()

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
