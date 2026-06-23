package db

import (
	"context"
	"errors"
	"strings"

	"github.com/exaring/otelpgx"
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

	config.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithSpanNameFunc(postgresSpanName),
		otelpgx.WithDisableAcquireTracer(),
		otelpgx.WithDisableConnectionDetailsInAttributes(),
		otelpgx.WithDisableSQLStatementInAttributes(),
	)

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

func postgresSpanName(statement string) string {
	fields := strings.Fields(statement)
	if len(fields) == 0 {
		return "postgres.query"
	}

	switch strings.ToUpper(fields[0]) {
	case "SELECT":
		return "postgres.select"
	case "INSERT":
		return "postgres.insert"
	case "UPDATE":
		return "postgres.update"
	case "DELETE":
		return "postgres.delete"
	case "BEGIN":
		return "postgres.begin"
	case "COMMIT":
		return "postgres.commit"
	case "ROLLBACK":
		return "postgres.rollback"
	default:
		return "postgres.query"
	}
}
