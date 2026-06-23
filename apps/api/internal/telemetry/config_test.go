package telemetry

import (
	"strings"
	"testing"
)

func TestLoadConfigUsesDisabledDefaults(t *testing.T) {
	t.Setenv("OTEL_TRACING_ENABLED", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("load telemetry configuration: %v", err)
	}

	if config.Enabled() {
		t.Fatal("tracing should be disabled by default")
	}

	if config.Endpoint() != DefaultEndpoint {
		t.Fatalf("endpoint = %q, want %q", config.Endpoint(), DefaultEndpoint)
	}
}

func TestLoadConfigAcceptsEnabledEndpoint(t *testing.T) {
	t.Setenv("OTEL_TRACING_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("load telemetry configuration: %v", err)
	}

	if !config.Enabled() {
		t.Fatal("tracing was not enabled")
	}

	if config.Endpoint() != "http://127.0.0.1:4318" {
		t.Fatalf("endpoint = %q", config.Endpoint())
	}
}

func TestLoadConfigRejectsUnsafeEndpointWithoutExposure(t *testing.T) {
	const secretMarker = "synthetic-otel-secret-marker"

	t.Setenv("OTEL_TRACING_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://user:"+secretMarker+"@127.0.0.1:4318")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected unsafe endpoint to fail")
	}

	if strings.Contains(err.Error(), secretMarker) {
		t.Fatal("configuration error exposed endpoint credentials")
	}
}

func TestLoadConfigRejectsInvalidEnabledValue(t *testing.T) {
	t.Setenv("OTEL_TRACING_ENABLED", "sometimes")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected invalid enabled value to fail")
	}
}
