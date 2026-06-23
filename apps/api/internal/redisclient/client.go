package redisclient

import (
	"context"
	"errors"
	"sync"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	redislogging "github.com/redis/go-redis/v9/logging"
)

var disableInternalLogging sync.Once

type Client struct {
	client *redis.Client
}

func New(ctx context.Context, config Config) (*Client, error) {
	disableInternalLogging.Do(redislogging.Disable)

	options, err := config.options()
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(options)

	if err := redisotel.InstrumentTracing(
		client,
		redisotel.WithCallerEnabled(false),
		redisotel.WithDBStatement(false),
		redisotel.WithCommandFilter(redisotel.DefaultCommandFilter),
	); err != nil {
		_ = client.Close()

		return nil, errors.New("unable to initialize Redis tracing")
	}

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()

		return nil, errors.New("redis unavailable")
	}

	return &Client{client: client}, nil
}

func (client *Client) Ping(ctx context.Context) error {
	if client == nil || client.client == nil {
		return errors.New("redis unavailable")
	}

	return client.client.Ping(ctx).Err()
}

func (client *Client) RunScript(
	ctx context.Context,
	source string,
	keys []string,
	args ...any,
) (any, error) {
	if client == nil || client.client == nil {
		return nil, errors.New("redis unavailable")
	}

	script := redis.NewScript(source)

	return script.Run(
		ctx,
		client.client,
		keys,
		args...,
	).Result()
}

func (client *Client) Close() error {
	if client == nil || client.client == nil {
		return nil
	}

	return client.client.Close()
}
