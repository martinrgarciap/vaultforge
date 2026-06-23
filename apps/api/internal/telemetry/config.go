package telemetry

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const DefaultEndpoint = "http://127.0.0.1:4318"

type Config struct {
	enabled  bool
	endpoint string
}

func LoadConfig() (Config, error) {
	enabledValue := strings.TrimSpace(os.Getenv("OTEL_TRACING_ENABLED"))
	if enabledValue == "" {
		enabledValue = "false"
	}

	enabled, err := strconv.ParseBool(enabledValue)
	if err != nil {
		return Config{}, errors.New("OTEL_TRACING_ENABLED must be true or false")
	}

	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	if !enabled {
		return Config{
			enabled:  false,
			endpoint: endpoint,
		}, nil
	}

	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil ||
		(parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") ||
		parsedEndpoint.Host == "" ||
		parsedEndpoint.User != nil ||
		parsedEndpoint.RawQuery != "" ||
		parsedEndpoint.Fragment != "" {
		return Config{}, errors.New("OTEL_EXPORTER_OTLP_ENDPOINT must be a safe HTTP or HTTPS base URL")
	}

	return Config{
		enabled:  true,
		endpoint: endpoint,
	}, nil
}

func (config Config) Enabled() bool {
	return config.enabled
}

func (config Config) Endpoint() string {
	if config.endpoint == "" {
		return DefaultEndpoint
	}

	return config.endpoint
}
