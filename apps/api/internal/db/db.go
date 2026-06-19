package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

func New(
	ctx context.Context,
	databaseURL string,
) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New(
			"invalid database configuration",
		)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, errors.New(
			"unable to create database connection pool",
		)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, errors.New(
			"database unavailable",
		)
	}

	return pool, nil
}
