package api

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env         string
	Addr        string
	DatabaseURL string
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
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))

	if value == "" {
		return fallback
	}

	return value
}
