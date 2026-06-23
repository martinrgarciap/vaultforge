package api

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessioncookie"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/redisclient"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
)

const (
	defaultAccessTokenIssuer   = "vaultforge-api"
	defaultAccessTokenAudience = "vaultforge-api"
	defaultAccessTokenKeyID    = "local-ed25519-v1"
)

type Config struct {
	Env            string
	Addr           string
	DatabaseURL    string
	Redis          redisclient.Config
	Tokens         session.TokenConfig
	SessionCookies sessioncookie.Config
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Env: strings.ToLower(
			getEnv("APP_ENV", "development"),
		),
		Addr:        getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
	}

	switch cfg.Env {
	case "development", "test", "production":
	default:
		return Config{}, fmt.Errorf(
			"APP_ENV must be development, test, or production",
		)
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf(
			"DATABASE_URL is required",
		)
	}

	redisConfig, err := loadRedisConfig()
	if err != nil {
		return Config{}, err
	}

	tokenConfig, err := loadTokenConfig()
	if err != nil {
		return Config{}, err
	}

	cfg.Redis = redisConfig
	cfg.Tokens = tokenConfig
	cfg.SessionCookies = sessioncookie.NewConfig(
		cfg.Env == "production",
	)

	return cfg, nil
}

func loadRedisConfig() (redisclient.Config, error) {
	dialTimeout, err := loadDuration(
		"REDIS_DIAL_TIMEOUT",
		redisclient.DefaultDialTimeout,
	)
	if err != nil {
		return redisclient.Config{}, err
	}

	readTimeout, err := loadDuration(
		"REDIS_READ_TIMEOUT",
		redisclient.DefaultReadTimeout,
	)
	if err != nil {
		return redisclient.Config{}, err
	}

	writeTimeout, err := loadDuration(
		"REDIS_WRITE_TIMEOUT",
		redisclient.DefaultWriteTimeout,
	)
	if err != nil {
		return redisclient.Config{}, err
	}

	poolTimeout, err := loadDuration(
		"REDIS_POOL_TIMEOUT",
		redisclient.DefaultPoolTimeout,
	)
	if err != nil {
		return redisclient.Config{}, err
	}

	config, err := redisclient.NewConfig(
		os.Getenv("REDIS_URL"),
		dialTimeout,
		readTimeout,
		writeTimeout,
		poolTimeout,
	)
	if err != nil {
		return redisclient.Config{}, err
	}

	return config, nil
}

func loadTokenConfig() (
	session.TokenConfig,
	error,
) {
	accessTokenTTL, err := loadDuration(
		"ACCESS_TOKEN_TTL",
		session.DefaultAccessTokenTTL,
	)
	if err != nil {
		return session.TokenConfig{}, err
	}

	refreshTokenTTL, err := loadDuration(
		"REFRESH_TOKEN_TTL",
		session.DefaultRefreshTokenTTL,
	)
	if err != nil {
		return session.TokenConfig{}, err
	}

	clockLeeway, err := loadDuration(
		"ACCESS_TOKEN_CLOCK_LEEWAY",
		session.DefaultClockLeeway,
	)
	if err != nil {
		return session.TokenConfig{}, err
	}

	lifetimes, err := session.NewTokenLifetimes(
		accessTokenTTL,
		refreshTokenTTL,
		clockLeeway,
	)
	if err != nil {
		return session.TokenConfig{},
			fmt.Errorf(
				"token lifetime configuration is invalid: %w",
				err,
			)
	}

	encodedSeed := strings.TrimSpace(
		os.Getenv(
			"ACCESS_TOKEN_ED25519_SEED_BASE64",
		),
	)

	if encodedSeed == "" {
		return session.TokenConfig{},
			fmt.Errorf(
				"ACCESS_TOKEN_ED25519_SEED_BASE64 is required",
			)
	}

	if strings.ContainsAny(
		encodedSeed,
		" \t\r\n",
	) {
		return session.TokenConfig{},
			fmt.Errorf(
				"ACCESS_TOKEN_ED25519_SEED_BASE64 must be valid standard Base64",
			)
	}

	seed, err := base64.StdEncoding.
		Strict().
		DecodeString(encodedSeed)
	if err != nil {
		return session.TokenConfig{},
			fmt.Errorf(
				"ACCESS_TOKEN_ED25519_SEED_BASE64 must be valid standard Base64",
			)
	}

	tokenConfig, err := session.NewTokenConfig(
		getEnv(
			"ACCESS_TOKEN_ISSUER",
			defaultAccessTokenIssuer,
		),
		getEnv(
			"ACCESS_TOKEN_AUDIENCE",
			defaultAccessTokenAudience,
		),
		getEnv(
			"ACCESS_TOKEN_KEY_ID",
			defaultAccessTokenKeyID,
		),
		seed,
		lifetimes,
	)
	if err != nil {
		return session.TokenConfig{},
			fmt.Errorf(
				"access token configuration is invalid: %w",
				err,
			)
	}

	return tokenConfig, nil
}

func loadDuration(
	key string,
	fallback time.Duration,
) (time.Duration, error) {
	value := getEnv(
		key,
		fallback.String(),
	)

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf(
			"%s must be a valid duration",
			key,
		)
	}

	return duration, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(
		os.Getenv(key),
	)

	if value == "" {
		return fallback
	}

	return value
}
