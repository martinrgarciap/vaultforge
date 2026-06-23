package redisclient

import (
	"strings"
	"testing"
	"time"
)

const testRedisURL = "redis://127.0.0.1:6379/1"

func TestNewConfigAcceptsValidConfiguration(t *testing.T) {
	config, err := NewConfig(
		testRedisURL,
		3*time.Second,
		2*time.Second,
		2*time.Second,
		4*time.Second,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if config.DialTimeout() != 3*time.Second {
		t.Errorf("dial timeout = %v, want %v", config.DialTimeout(), 3*time.Second)
	}

	if config.ReadTimeout() != 2*time.Second {
		t.Errorf("read timeout = %v, want %v", config.ReadTimeout(), 2*time.Second)
	}

	if config.WriteTimeout() != 2*time.Second {
		t.Errorf("write timeout = %v, want %v", config.WriteTimeout(), 2*time.Second)
	}

	if config.PoolTimeout() != 4*time.Second {
		t.Errorf("pool timeout = %v, want %v", config.PoolTimeout(), 4*time.Second)
	}
}

func TestNewConfigRejectsMissingURL(t *testing.T) {
	_, err := NewConfig(
		"   ",
		DefaultDialTimeout,
		DefaultReadTimeout,
		DefaultWriteTimeout,
		DefaultPoolTimeout,
	)
	if err == nil {
		t.Fatal("expected missing Redis URL to return an error")
	}
}

func TestNewConfigRejectsMalformedURLWithoutExposingIt(t *testing.T) {
	const secretMarker = "synthetic-redis-password-marker"

	_, err := NewConfig(
		"not-a-redis-url://:"+secretMarker,
		DefaultDialTimeout,
		DefaultReadTimeout,
		DefaultWriteTimeout,
		DefaultPoolTimeout,
	)
	if err == nil {
		t.Fatal("expected malformed Redis URL to return an error")
	}

	if strings.Contains(err.Error(), secretMarker) {
		t.Fatal("configuration error exposed the Redis URL")
	}
}

func TestNewConfigRejectsNonPositiveTimeouts(t *testing.T) {
	testCases := []struct {
		name         string
		dialTimeout  time.Duration
		readTimeout  time.Duration
		writeTimeout time.Duration
		poolTimeout  time.Duration
	}{
		{
			name:         "zero dial timeout",
			dialTimeout:  0,
			readTimeout:  DefaultReadTimeout,
			writeTimeout: DefaultWriteTimeout,
			poolTimeout:  DefaultPoolTimeout,
		},
		{
			name:         "negative read timeout",
			dialTimeout:  DefaultDialTimeout,
			readTimeout:  -time.Second,
			writeTimeout: DefaultWriteTimeout,
			poolTimeout:  DefaultPoolTimeout,
		},
		{
			name:         "zero write timeout",
			dialTimeout:  DefaultDialTimeout,
			readTimeout:  DefaultReadTimeout,
			writeTimeout: 0,
			poolTimeout:  DefaultPoolTimeout,
		},
		{
			name:         "negative pool timeout",
			dialTimeout:  DefaultDialTimeout,
			readTimeout:  DefaultReadTimeout,
			writeTimeout: DefaultWriteTimeout,
			poolTimeout:  -time.Second,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewConfig(
				testRedisURL,
				testCase.dialTimeout,
				testCase.readTimeout,
				testCase.writeTimeout,
				testCase.poolTimeout,
			)
			if err == nil {
				t.Fatal("expected invalid timeout to return an error")
			}
		})
	}
}
