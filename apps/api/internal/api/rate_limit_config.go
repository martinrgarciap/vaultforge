package api

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
)

const (
	defaultRegistrationRequests int64 = 5
	defaultRegistrationWindow         = 10 * time.Minute

	defaultLoginRequests int64 = 20
	defaultLoginWindow         = time.Minute

	defaultRefreshRequests int64 = 30
	defaultRefreshWindow         = time.Minute

	defaultMutationRequests int64 = 60
	defaultMutationWindow         = time.Minute

	defaultPasswordToolsRequests int64 = 60
	defaultPasswordToolsWindow         = time.Minute

	defaultLoginFailureLimit    int64 = 5
	defaultLoginFailureWindow         = 15 * time.Minute
	defaultLoginLockoutDuration       = 15 * time.Minute
)

type RateLimitConfig struct {
	Registration    ratelimit.Policy
	Login           ratelimit.Policy
	Refresh         ratelimit.Policy
	Mutation        ratelimit.Policy
	PasswordTools   ratelimit.Policy
	LoginProtection ratelimit.LoginPolicy

	keyPrefix       string
	identityHMACKey []byte
}

func loadRateLimitConfig(
	environment string,
) (RateLimitConfig, error) {
	identityHMACKey, err := loadRateLimitIdentityHMACKey()
	if err != nil {
		return RateLimitConfig{}, err
	}

	registration, err := loadRateLimitPolicy(
		"RATE_LIMIT_REGISTRATION_REQUESTS",
		defaultRegistrationRequests,
		"RATE_LIMIT_REGISTRATION_WINDOW",
		defaultRegistrationWindow,
	)
	if err != nil {
		return RateLimitConfig{}, err
	}

	login, err := loadRateLimitPolicy(
		"RATE_LIMIT_LOGIN_REQUESTS",
		defaultLoginRequests,
		"RATE_LIMIT_LOGIN_WINDOW",
		defaultLoginWindow,
	)
	if err != nil {
		return RateLimitConfig{}, err
	}

	refresh, err := loadRateLimitPolicy(
		"RATE_LIMIT_REFRESH_REQUESTS",
		defaultRefreshRequests,
		"RATE_LIMIT_REFRESH_WINDOW",
		defaultRefreshWindow,
	)
	if err != nil {
		return RateLimitConfig{}, err
	}

	mutation, err := loadRateLimitPolicy(
		"RATE_LIMIT_MUTATION_REQUESTS",
		defaultMutationRequests,
		"RATE_LIMIT_MUTATION_WINDOW",
		defaultMutationWindow,
	)
	if err != nil {
		return RateLimitConfig{}, err
	}

	passwordTools, err := loadRateLimitPolicy(
		"RATE_LIMIT_PASSWORD_TOOLS_REQUESTS",
		defaultPasswordToolsRequests,
		"RATE_LIMIT_PASSWORD_TOOLS_WINDOW",
		defaultPasswordToolsWindow,
	)
	if err != nil {
		return RateLimitConfig{}, err
	}

	failureLimit, err := loadPositiveInt64(
		"LOGIN_FAILURE_LIMIT",
		defaultLoginFailureLimit,
	)
	if err != nil {
		return RateLimitConfig{}, err
	}

	failureWindow, err := loadDuration(
		"LOGIN_FAILURE_WINDOW",
		defaultLoginFailureWindow,
	)
	if err != nil {
		return RateLimitConfig{}, err
	}

	lockoutDuration, err := loadDuration(
		"LOGIN_LOCKOUT_DURATION",
		defaultLoginLockoutDuration,
	)
	if err != nil {
		return RateLimitConfig{}, err
	}

	loginProtection, err := ratelimit.NewLoginPolicy(
		failureLimit,
		failureWindow,
		lockoutDuration,
	)
	if err != nil {
		return RateLimitConfig{}, fmt.Errorf(
			"login protection configuration is invalid: %w",
			err,
		)
	}

	return RateLimitConfig{
		Registration:    registration,
		Login:           login,
		Refresh:         refresh,
		Mutation:        mutation,
		PasswordTools:   passwordTools,
		LoginProtection: loginProtection,
		keyPrefix:       "vaultforge:" + environment + ":v1",
		identityHMACKey: append(
			[]byte(nil),
			identityHMACKey...,
		),
	}, nil
}

func (config RateLimitConfig) NewKeyBuilder() (
	*ratelimit.KeyBuilder,
	error,
) {
	return ratelimit.NewKeyBuilder(
		config.keyPrefix,
		config.identityHMACKey,
	)
}

func loadRateLimitPolicy(
	requestsKey string,
	defaultRequests int64,
	windowKey string,
	defaultWindow time.Duration,
) (ratelimit.Policy, error) {
	requests, err := loadPositiveInt64(
		requestsKey,
		defaultRequests,
	)
	if err != nil {
		return ratelimit.Policy{}, err
	}

	window, err := loadDuration(
		windowKey,
		defaultWindow,
	)
	if err != nil {
		return ratelimit.Policy{}, err
	}

	policy, err := ratelimit.NewPolicy(
		requests,
		window,
	)
	if err != nil {
		return ratelimit.Policy{}, fmt.Errorf(
			"%s and %s are invalid: %w",
			requestsKey,
			windowKey,
			err,
		)
	}

	return policy, nil
}

func loadPositiveInt64(
	key string,
	fallback int64,
) (int64, error) {
	value := getEnv(
		key,
		strconv.FormatInt(fallback, 10),
	)

	parsedValue, err := strconv.ParseInt(
		value,
		10,
		64,
	)
	if err != nil || parsedValue <= 0 {
		return 0, fmt.Errorf(
			"%s must be a positive integer",
			key,
		)
	}

	return parsedValue, nil
}

func loadRateLimitIdentityHMACKey() (
	[]byte,
	error,
) {
	encodedKey := strings.TrimSpace(
		os.Getenv(
			"RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64",
		),
	)

	if encodedKey == "" {
		return nil, fmt.Errorf(
			"RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64 is required",
		)
	}

	if strings.ContainsAny(
		encodedKey,
		" \t\r\n",
	) {
		return nil, fmt.Errorf(
			"RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64 must be valid standard Base64",
		)
	}

	key, err := base64.StdEncoding.
		Strict().
		DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf(
			"RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64 must be valid standard Base64",
		)
	}

	if len(key) < ratelimit.MinIdentityHMACKeyBytes {
		return nil, fmt.Errorf(
			"RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64 must decode to at least %d bytes",
			ratelimit.MinIdentityHMACKeyBytes,
		)
	}

	return key, nil
}
