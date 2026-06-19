package api

import "testing"

const testDatabaseURL = "postgres://vaultforge:test@localhost:5432/vaultforge_test?sslmode=disable"

func TestLoadConfigUsesDefaults(t *testing.T) {
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
}

func TestLoadConfigUsesEnvironmentVariables(t *testing.T) {
	databaseURL := "postgres://vaultforge:test@localhost:5433/custom_test?sslmode=disable"

	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_URL", databaseURL)

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
}

func TestLoadConfigRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "invalid")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", testDatabaseURL)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected invalid environment to return an error")
	}
}

func TestLoadConfigRejectsMissingDatabaseURL(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected missing database URL to return an error")
	}
}

func TestLoadConfigRejectsWhitespaceDatabaseURL(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", "   ")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected whitespace database URL to return an error")
	}
}
