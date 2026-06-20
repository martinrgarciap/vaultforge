package api

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
)

const (
	defaultAccessTokenIssuer   = "vaultforge-api"
	defaultAccessTokenAudience = "vaultforge-api"
	defaultAccessTokenKeyID    = "local-ed25519-v1"
)

type Config struct {
	Env         string
	Addr        string
	DatabaseURL string
	Tokens      session.TokenConfig
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

	tokenConfig, err := loadTokenConfig()
	if err != nil {
		return Config{}, err
	}

	cfg.Tokens = tokenConfig

	return cfg, nil
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
