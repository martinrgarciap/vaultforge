package passwordclient

import (
	"testing"
	"time"
)

func TestNewConfigAcceptsValidConfig(t *testing.T) {
	t.Parallel()

	config, err := NewConfig(
		"127.0.0.1:50053",
		2*time.Second,
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	if config.Address() != "127.0.0.1:50053" {
		t.Fatalf("address = %q, want 127.0.0.1:50053", config.Address())
	}

	if config.DialTimeout() != 2*time.Second {
		t.Fatalf("dial timeout = %v, want 2s", config.DialTimeout())
	}

	if config.RequestTimeout() != 5*time.Second {
		t.Fatalf("request timeout = %v, want 5s", config.RequestTimeout())
	}
}

func TestNewConfigRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		address        string
		dialTimeout    time.Duration
		requestTimeout time.Duration
	}{
		{
			name:           "empty address",
			address:        "",
			dialTimeout:    DefaultDialTimeout,
			requestTimeout: DefaultRequestTimeout,
		},
		{
			name:           "address with scheme",
			address:        "http://127.0.0.1:50053",
			dialTimeout:    DefaultDialTimeout,
			requestTimeout: DefaultRequestTimeout,
		},
		{
			name:           "address without port",
			address:        "127.0.0.1",
			dialTimeout:    DefaultDialTimeout,
			requestTimeout: DefaultRequestTimeout,
		},
		{
			name:           "non-numeric port",
			address:        "127.0.0.1:not-a-port",
			dialTimeout:    DefaultDialTimeout,
			requestTimeout: DefaultRequestTimeout,
		},
		{
			name:           "zero port",
			address:        "127.0.0.1:0",
			dialTimeout:    DefaultDialTimeout,
			requestTimeout: DefaultRequestTimeout,
		},
		{
			name:           "invalid high port",
			address:        "127.0.0.1:70000",
			dialTimeout:    DefaultDialTimeout,
			requestTimeout: DefaultRequestTimeout,
		},
		{
			name:           "zero dial timeout",
			address:        DefaultAddress,
			dialTimeout:    0,
			requestTimeout: DefaultRequestTimeout,
		},
		{
			name:           "zero request timeout",
			address:        DefaultAddress,
			dialTimeout:    DefaultDialTimeout,
			requestTimeout: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewConfig(
				test.address,
				test.dialTimeout,
				test.requestTimeout,
			)
			if err == nil {
				t.Fatal("NewConfig() returned nil error, want validation error")
			}
		})
	}
}
