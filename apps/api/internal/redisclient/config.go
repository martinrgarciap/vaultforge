package redisclient

import (
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultDialTimeout  = 2 * time.Second
	DefaultReadTimeout  = time.Second
	DefaultWriteTimeout = time.Second
	DefaultPoolTimeout  = 2 * time.Second
)

type Config struct {
	url          string
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	poolTimeout  time.Duration
}

func NewConfig(
	url string,
	dialTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	poolTimeout time.Duration,
) (Config, error) {
	url = strings.TrimSpace(url)

	if url == "" {
		return Config{}, errors.New("REDIS_URL is required")
	}

	if _, err := redis.ParseURL(url); err != nil {
		return Config{}, errors.New("REDIS_URL must be a valid Redis URL")
	}

	timeouts := []struct {
		name  string
		value time.Duration
	}{
		{name: "REDIS_DIAL_TIMEOUT", value: dialTimeout},
		{name: "REDIS_READ_TIMEOUT", value: readTimeout},
		{name: "REDIS_WRITE_TIMEOUT", value: writeTimeout},
		{name: "REDIS_POOL_TIMEOUT", value: poolTimeout},
	}

	for _, timeout := range timeouts {
		if timeout.value <= 0 {
			return Config{}, errors.New(timeout.name + " must be greater than zero")
		}
	}

	return Config{
		url:          url,
		dialTimeout:  dialTimeout,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
		poolTimeout:  poolTimeout,
	}, nil
}

func (config Config) DialTimeout() time.Duration {
	return config.dialTimeout
}

func (config Config) ReadTimeout() time.Duration {
	return config.readTimeout
}

func (config Config) WriteTimeout() time.Duration {
	return config.writeTimeout
}

func (config Config) PoolTimeout() time.Duration {
	return config.poolTimeout
}

func (config Config) options() (*redis.Options, error) {
	options, err := redis.ParseURL(config.url)
	if err != nil {
		return nil, errors.New("invalid Redis configuration")
	}

	options.DialTimeout = config.dialTimeout
	options.ReadTimeout = config.readTimeout
	options.WriteTimeout = config.writeTimeout
	options.PoolTimeout = config.poolTimeout
	options.MaxRetries = 1
	options.DialerRetries = 1
	options.ContextTimeoutEnabled = true

	return options, nil
}
