package api

import "testing"

func TestLoadConfigUsesDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")

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
}

func TestLoadConfigUsesEnvironmentVariables(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", ":9090")

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
}

func TestLoadConfigRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "invalid")
	t.Setenv("HTTP_ADDR", ":8080")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected invalid environment to return an error")
	}
}
