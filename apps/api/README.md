# VaultForge API

Go HTTP API for VaultForge.

## Current capabilities

- Chi HTTP routing
- Structured Zap logging
- Request IDs
- Safe request logging
- Panic recovery
- Security headers
- Graceful shutdown
- PostgreSQL connection pooling with pgx
- Versioned database migrations
- Liveness and readiness endpoints
- Initial user persistence
- Real PostgreSQL integration tests

## Run locally

From the repository root:

```bash
make db-setup
make dev
```

The default API address is:

```text
http://localhost:8080
```

## Health endpoints

```text
GET /health
GET /health/live
GET /health/ready
```

`/health/ready` returns `503 Service Unavailable` when PostgreSQL is not
available.

## Test

From the repository root:

```bash
make test
```

Or from this directory:

```bash
go test -race -count=1 ./...
```

`TEST_DATABASE_URL` must be configured to run the PostgreSQL integration tests.

## Security

Never log or return:

- Passwords
- Authorization headers
- Cookies
- Refresh tokens
- Database URLs
- Vault values
- Decrypted vault contents

Use synthetic data during development and testing.
