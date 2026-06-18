package api

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env  string
	Addr string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Env: strings.ToLower(
			getEnv("APP_ENV", "development"),
		),
		Addr: getEnv("HTTP_ADDR", ":8080"),
	}

	switch cfg.Env {
	case "development", "test", "production":
	default:
		return Config{}, fmt.Errorf(
			"APP_ENV must be development, test, or production",
		)
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
