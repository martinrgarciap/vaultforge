package passwordclient

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultAddress        = "127.0.0.1:50053"
	DefaultDialTimeout    = 2 * time.Second
	DefaultRequestTimeout = 5 * time.Second
)

type Config struct {
	address        string
	dialTimeout    time.Duration
	requestTimeout time.Duration
}

func NewConfig(
	address string,
	dialTimeout time.Duration,
	requestTimeout time.Duration,
) (Config, error) {
	address = strings.TrimSpace(address)

	if address == "" {
		return Config{}, errors.New("PASSWORD_SERVICE_ADDR is required")
	}

	if strings.Contains(address, "://") {
		return Config{}, errors.New("PASSWORD_SERVICE_ADDR must be host:port")
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return Config{}, errors.New("PASSWORD_SERVICE_ADDR must be host:port")
	}

	if strings.TrimSpace(host) == "" {
		return Config{}, errors.New("PASSWORD_SERVICE_ADDR host is required")
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, errors.New("PASSWORD_SERVICE_ADDR port must be between 1 and 65535")
	}

	if dialTimeout <= 0 {
		return Config{}, errors.New("PASSWORD_SERVICE_DIAL_TIMEOUT must be greater than zero")
	}

	if requestTimeout <= 0 {
		return Config{}, errors.New("PASSWORD_SERVICE_TIMEOUT must be greater than zero")
	}

	return Config{
		address:        address,
		dialTimeout:    dialTimeout,
		requestTimeout: requestTimeout,
	}, nil
}

func (config Config) Address() string {
	return config.address
}

func (config Config) DialTimeout() time.Duration {
	return config.dialTimeout
}

func (config Config) RequestTimeout() time.Duration {
	return config.requestTimeout
}
