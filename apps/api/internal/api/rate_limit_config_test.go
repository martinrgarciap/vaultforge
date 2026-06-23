package api

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigUsesDefaultRateLimits(
	t *testing.T,
) {
	setValidConfigEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}

	if cfg.RateLimits.Registration.Limit() != 5 ||
		cfg.RateLimits.Registration.Window() !=
			10*time.Minute {
		t.Fatalf(
			"registration policy = %d per %v",
			cfg.RateLimits.Registration.Limit(),
			cfg.RateLimits.Registration.Window(),
		)
	}

	if cfg.RateLimits.Login.Limit() != 20 ||
		cfg.RateLimits.Login.Window() != time.Minute {
		t.Fatalf(
			"login policy = %d per %v",
			cfg.RateLimits.Login.Limit(),
			cfg.RateLimits.Login.Window(),
		)
	}

	if cfg.RateLimits.Refresh.Limit() != 30 ||
		cfg.RateLimits.Refresh.Window() != time.Minute {
		t.Fatalf(
			"refresh policy = %d per %v",
			cfg.RateLimits.Refresh.Limit(),
			cfg.RateLimits.Refresh.Window(),
		)
	}

	if cfg.RateLimits.Mutation.Limit() != 60 ||
		cfg.RateLimits.Mutation.Window() != time.Minute {
		t.Fatalf(
			"mutation policy = %d per %v",
			cfg.RateLimits.Mutation.Limit(),
			cfg.RateLimits.Mutation.Window(),
		)
	}

	if cfg.RateLimits.LoginProtection.FailureLimit() != 5 ||
		cfg.RateLimits.LoginProtection.FailureWindow() !=
			15*time.Minute ||
		cfg.RateLimits.LoginProtection.LockoutDuration() !=
			15*time.Minute {
		t.Fatalf(
			"unexpected login-protection defaults",
		)
	}

	if _, err := cfg.RateLimits.NewKeyBuilder(); err != nil {
		t.Fatalf("create key builder: %v", err)
	}
}

func TestLoadConfigUsesCustomRateLimits(
	t *testing.T,
) {
	setValidConfigEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)

	t.Setenv("RATE_LIMIT_REGISTRATION_REQUESTS", "7")
	t.Setenv("RATE_LIMIT_REGISTRATION_WINDOW", "12m")
	t.Setenv("RATE_LIMIT_LOGIN_REQUESTS", "25")
	t.Setenv("RATE_LIMIT_LOGIN_WINDOW", "90s")
	t.Setenv("RATE_LIMIT_REFRESH_REQUESTS", "40")
	t.Setenv("RATE_LIMIT_REFRESH_WINDOW", "2m")
	t.Setenv("RATE_LIMIT_MUTATION_REQUESTS", "75")
	t.Setenv("RATE_LIMIT_MUTATION_WINDOW", "45s")
	t.Setenv("LOGIN_FAILURE_LIMIT", "6")
	t.Setenv("LOGIN_FAILURE_WINDOW", "20m")
	t.Setenv("LOGIN_LOCKOUT_DURATION", "30m")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}

	if cfg.RateLimits.Registration.Limit() != 7 ||
		cfg.RateLimits.Registration.Window() !=
			12*time.Minute {
		t.Fatal("custom registration policy was not loaded")
	}

	if cfg.RateLimits.Login.Limit() != 25 ||
		cfg.RateLimits.Login.Window() !=
			90*time.Second {
		t.Fatal("custom login policy was not loaded")
	}

	if cfg.RateLimits.Refresh.Limit() != 40 ||
		cfg.RateLimits.Refresh.Window() !=
			2*time.Minute {
		t.Fatal("custom refresh policy was not loaded")
	}

	if cfg.RateLimits.Mutation.Limit() != 75 ||
		cfg.RateLimits.Mutation.Window() !=
			45*time.Second {
		t.Fatal("custom mutation policy was not loaded")
	}

	if cfg.RateLimits.LoginProtection.FailureLimit() != 6 ||
		cfg.RateLimits.LoginProtection.FailureWindow() !=
			20*time.Minute ||
		cfg.RateLimits.LoginProtection.LockoutDuration() !=
			30*time.Minute {
		t.Fatal("custom login protection was not loaded")
	}
}

func TestLoadConfigRejectsMissingRateLimitIdentityKey(
	t *testing.T,
) {
	setValidConfigEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected missing HMAC key to fail")
	}
}

func TestLoadConfigRejectsMalformedRateLimitIdentityKey(
	t *testing.T,
) {
	const marker = "synthetic-rate-limit-secret-marker"

	setValidConfigEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv(
		"RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64",
		marker,
	)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected malformed HMAC key to fail")
	}

	if strings.Contains(err.Error(), marker) {
		t.Fatal("configuration error exposed HMAC key material")
	}
}

func TestLoadConfigRejectsShortRateLimitIdentityKey(
	t *testing.T,
) {
	setValidConfigEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv(
		"RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64",
		base64.StdEncoding.EncodeToString(
			bytes.Repeat([]byte{0x42}, 31),
		),
	)

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected short HMAC key to fail")
	}
}

func TestLoadConfigRejectsInvalidRateLimitPolicies(
	t *testing.T,
) {
	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "zero registration requests",
			key:   "RATE_LIMIT_REGISTRATION_REQUESTS",
			value: "0",
		},
		{
			name:  "malformed login requests",
			key:   "RATE_LIMIT_LOGIN_REQUESTS",
			value: "many",
		},
		{
			name:  "negative refresh requests",
			key:   "RATE_LIMIT_REFRESH_REQUESTS",
			value: "-1",
		},
		{
			name:  "zero mutation window",
			key:   "RATE_LIMIT_MUTATION_WINDOW",
			value: "0s",
		},
		{
			name:  "malformed registration window",
			key:   "RATE_LIMIT_REGISTRATION_WINDOW",
			value: "later",
		},
		{
			name:  "zero login failure limit",
			key:   "LOGIN_FAILURE_LIMIT",
			value: "0",
		},
		{
			name:  "negative login failure window",
			key:   "LOGIN_FAILURE_WINDOW",
			value: "-1s",
		},
		{
			name:  "zero lockout duration",
			key:   "LOGIN_LOCKOUT_DURATION",
			value: "0s",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setValidConfigEnvironment(t)
			t.Setenv("DATABASE_URL", testDatabaseURL)
			t.Setenv(testCase.key, testCase.value)

			if _, err := LoadConfig(); err == nil {
				t.Fatal(
					"expected invalid policy to return an error",
				)
			}
		})
	}
}
