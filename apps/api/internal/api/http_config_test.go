package api

import (
	"testing"
	"time"
)

func TestLoadConfigUsesDefaultHTTPTimeouts(
	t *testing.T,
) {
	setValidConfigEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}

	if config.HTTP.RequestTimeout() !=
		15*time.Second {
		t.Fatalf(
			"request timeout = %v, want %v",
			config.HTTP.RequestTimeout(),
			15*time.Second,
		)
	}

	if config.HTTP.ReadHeaderTimeout() !=
		5*time.Second {
		t.Fatalf(
			"read-header timeout = %v, want %v",
			config.HTTP.ReadHeaderTimeout(),
			5*time.Second,
		)
	}

	if config.HTTP.ReadTimeout() !=
		10*time.Second {
		t.Fatalf(
			"read timeout = %v, want %v",
			config.HTTP.ReadTimeout(),
			10*time.Second,
		)
	}

	if config.HTTP.WriteTimeout() !=
		30*time.Second {
		t.Fatalf(
			"write timeout = %v, want %v",
			config.HTTP.WriteTimeout(),
			30*time.Second,
		)
	}

	if config.HTTP.IdleTimeout() != time.Minute {
		t.Fatalf(
			"idle timeout = %v, want %v",
			config.HTTP.IdleTimeout(),
			time.Minute,
		)
	}

	if config.HTTP.ShutdownTimeout() !=
		5*time.Second {
		t.Fatalf(
			"shutdown timeout = %v, want %v",
			config.HTTP.ShutdownTimeout(),
			5*time.Second,
		)
	}
}

func TestLoadConfigUsesCustomHTTPTimeouts(
	t *testing.T,
) {
	setValidConfigEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)

	t.Setenv("HTTP_REQUEST_TIMEOUT", "20s")
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "4s")
	t.Setenv("HTTP_READ_TIMEOUT", "12s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "45s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "90s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "8s")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}

	if config.HTTP.RequestTimeout() !=
		20*time.Second {
		t.Fatal("custom request timeout was not loaded")
	}

	if config.HTTP.ReadHeaderTimeout() !=
		4*time.Second {
		t.Fatal("custom read-header timeout was not loaded")
	}

	if config.HTTP.ReadTimeout() !=
		12*time.Second {
		t.Fatal("custom read timeout was not loaded")
	}

	if config.HTTP.WriteTimeout() !=
		45*time.Second {
		t.Fatal("custom write timeout was not loaded")
	}

	if config.HTTP.IdleTimeout() !=
		90*time.Second {
		t.Fatal("custom idle timeout was not loaded")
	}

	if config.HTTP.ShutdownTimeout() !=
		8*time.Second {
		t.Fatal("custom shutdown timeout was not loaded")
	}
}

func TestHTTPConfigZeroValueUsesSafeDefaults(
	t *testing.T,
) {
	t.Parallel()

	var config HTTPConfig

	if config.RequestTimeout() !=
		defaultHTTPRequestTimeout {
		t.Fatal("zero-value request timeout was unsafe")
	}

	if config.ReadHeaderTimeout() !=
		defaultReadHeaderTimeout {
		t.Fatal("zero-value read-header timeout was unsafe")
	}

	if config.ReadTimeout() !=
		defaultReadTimeout {
		t.Fatal("zero-value read timeout was unsafe")
	}

	if config.WriteTimeout() !=
		defaultWriteTimeout {
		t.Fatal("zero-value write timeout was unsafe")
	}

	if config.IdleTimeout() !=
		defaultIdleTimeout {
		t.Fatal("zero-value idle timeout was unsafe")
	}

	if config.ShutdownTimeout() !=
		defaultShutdownTimeout {
		t.Fatal("zero-value shutdown timeout was unsafe")
	}
}

func TestLoadConfigRejectsInvalidHTTPTimeouts(
	t *testing.T,
) {
	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "malformed request timeout",
			key:   "HTTP_REQUEST_TIMEOUT",
			value: "eventually",
		},
		{
			name:  "zero read-header timeout",
			key:   "HTTP_READ_HEADER_TIMEOUT",
			value: "0s",
		},
		{
			name:  "negative read timeout",
			key:   "HTTP_READ_TIMEOUT",
			value: "-1s",
		},
		{
			name:  "zero write timeout",
			key:   "HTTP_WRITE_TIMEOUT",
			value: "0s",
		},
		{
			name:  "zero idle timeout",
			key:   "HTTP_IDLE_TIMEOUT",
			value: "0s",
		},
		{
			name:  "negative shutdown timeout",
			key:   "HTTP_SHUTDOWN_TIMEOUT",
			value: "-1s",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setValidConfigEnvironment(t)
			t.Setenv("DATABASE_URL", testDatabaseURL)
			t.Setenv(testCase.key, testCase.value)

			if _, err := LoadConfig(); err == nil {
				t.Fatal(
					"expected invalid HTTP timeout to fail",
				)
			}
		})
	}
}

func TestLoadConfigRejectsWriteTimeoutNotGreaterThanRequestTimeout(
	t *testing.T,
) {
	setValidConfigEnvironment(t)
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("HTTP_REQUEST_TIMEOUT", "30s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "30s")

	if _, err := LoadConfig(); err == nil {
		t.Fatal(
			"expected incompatible request and write timeouts to fail",
		)
	}
}
