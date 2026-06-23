package api

import (
	"fmt"
	"time"
)

const (
	defaultHTTPRequestTimeout = 15 * time.Second
	defaultReadHeaderTimeout  = 5 * time.Second
	defaultReadTimeout        = 10 * time.Second
	defaultWriteTimeout       = 30 * time.Second
	defaultIdleTimeout        = time.Minute
	defaultShutdownTimeout    = 5 * time.Second
)

type HTTPConfig struct {
	requestTimeout    time.Duration
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	shutdownTimeout   time.Duration
}

func loadHTTPConfig() (HTTPConfig, error) {
	requestTimeout, err := loadDuration(
		"HTTP_REQUEST_TIMEOUT",
		defaultHTTPRequestTimeout,
	)
	if err != nil {
		return HTTPConfig{}, err
	}

	readHeaderTimeout, err := loadDuration(
		"HTTP_READ_HEADER_TIMEOUT",
		defaultReadHeaderTimeout,
	)
	if err != nil {
		return HTTPConfig{}, err
	}

	readTimeout, err := loadDuration(
		"HTTP_READ_TIMEOUT",
		defaultReadTimeout,
	)
	if err != nil {
		return HTTPConfig{}, err
	}

	writeTimeout, err := loadDuration(
		"HTTP_WRITE_TIMEOUT",
		defaultWriteTimeout,
	)
	if err != nil {
		return HTTPConfig{}, err
	}

	idleTimeout, err := loadDuration(
		"HTTP_IDLE_TIMEOUT",
		defaultIdleTimeout,
	)
	if err != nil {
		return HTTPConfig{}, err
	}

	shutdownTimeout, err := loadDuration(
		"HTTP_SHUTDOWN_TIMEOUT",
		defaultShutdownTimeout,
	)
	if err != nil {
		return HTTPConfig{}, err
	}

	return newHTTPConfig(
		requestTimeout,
		readHeaderTimeout,
		readTimeout,
		writeTimeout,
		idleTimeout,
		shutdownTimeout,
	)
}

func newHTTPConfig(
	requestTimeout time.Duration,
	readHeaderTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	idleTimeout time.Duration,
	shutdownTimeout time.Duration,
) (HTTPConfig, error) {
	values := []struct {
		name  string
		value time.Duration
	}{
		{
			name:  "HTTP_REQUEST_TIMEOUT",
			value: requestTimeout,
		},
		{
			name:  "HTTP_READ_HEADER_TIMEOUT",
			value: readHeaderTimeout,
		},
		{
			name:  "HTTP_READ_TIMEOUT",
			value: readTimeout,
		},
		{
			name:  "HTTP_WRITE_TIMEOUT",
			value: writeTimeout,
		},
		{
			name:  "HTTP_IDLE_TIMEOUT",
			value: idleTimeout,
		},
		{
			name:  "HTTP_SHUTDOWN_TIMEOUT",
			value: shutdownTimeout,
		},
	}

	for _, value := range values {
		if value.value <= 0 {
			return HTTPConfig{}, fmt.Errorf(
				"%s must be greater than zero",
				value.name,
			)
		}
	}

	if writeTimeout <= requestTimeout {
		return HTTPConfig{}, fmt.Errorf(
			"HTTP_WRITE_TIMEOUT must be greater than HTTP_REQUEST_TIMEOUT",
		)
	}

	return HTTPConfig{
		requestTimeout:    requestTimeout,
		readHeaderTimeout: readHeaderTimeout,
		readTimeout:       readTimeout,
		writeTimeout:      writeTimeout,
		idleTimeout:       idleTimeout,
		shutdownTimeout:   shutdownTimeout,
	}, nil
}

func (config HTTPConfig) RequestTimeout() time.Duration {
	if config.requestTimeout <= 0 {
		return defaultHTTPRequestTimeout
	}

	return config.requestTimeout
}

func (config HTTPConfig) ReadHeaderTimeout() time.Duration {
	if config.readHeaderTimeout <= 0 {
		return defaultReadHeaderTimeout
	}

	return config.readHeaderTimeout
}

func (config HTTPConfig) ReadTimeout() time.Duration {
	if config.readTimeout <= 0 {
		return defaultReadTimeout
	}

	return config.readTimeout
}

func (config HTTPConfig) WriteTimeout() time.Duration {
	if config.writeTimeout <= 0 {
		return defaultWriteTimeout
	}

	return config.writeTimeout
}

func (config HTTPConfig) IdleTimeout() time.Duration {
	if config.idleTimeout <= 0 {
		return defaultIdleTimeout
	}

	return config.idleTimeout
}

func (config HTTPConfig) ShutdownTimeout() time.Duration {
	if config.shutdownTimeout <= 0 {
		return defaultShutdownTimeout
	}

	return config.shutdownTimeout
}
