# VaultForge Operational Runbook

This runbook covers the currently implemented VaultForge components:

- Go API
- React client
- PostgreSQL
- Redis
- Rust hash-service
- Rust password-service
- OpenTelemetry Collector
- Jaeger

VaultForge is a portfolio and learning project. Use synthetic data only. Do not
paste credentials, tokens, cookies, signing keys, HMAC keys, database URLs,
Redis URLs, request bodies, vault payloads, or decrypted data into logs, issues,
screenshots, or support messages.

## Normal local startup

Start PostgreSQL and Redis:

```bash
make compose-up
```

Start the API:

```bash
make dev-api
```

Start the frontend in another terminal:

```bash
make dev-web
```

Check dependency readiness:

```bash
curl -s http://localhost:8080/health/ready
```

Expected healthy response:

```json
{ "status": "ok", "environment": "development" }
```

## Optional local tracing

Tracing is disabled by default and is not required for normal application
operation.

Start Jaeger and the OpenTelemetry Collector:

```bash
make observability-up
```

Run the API with tracing enabled:

```bash
OTEL_TRACING_ENABLED=true \
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 \
make dev-api
```

Generate a request:

```bash
curl -s http://localhost:8080/health/ready
```

Open the Jaeger UI:

```text
http://localhost:16686
```

Select `vaultforge-api`, find traces, and open the
`GET /health/ready` trace.

The local tracing pipeline is:

```text
VaultForge API
    |
    | OTLP over HTTP
    v
OpenTelemetry Collector
    |
    | OTLP over gRPC
    v
Jaeger
```

Jaeger uses temporary local storage in this development setup. Traces disappear
when the Jaeger container is recreated.

Stop the optional tracing stack:

```bash
make observability-stop
```

## Safe operational signals

Safe information to inspect or share includes:

- HTTP method
- Normalized Chi route pattern
- HTTP status code or status class
- Request duration
- Bounded request ID
- Trace ID
- Sanitized service version and commit
- Dependency name
- High-level PostgreSQL operation such as `postgres.select`
- Whether PostgreSQL, Redis, the Collector, or Jaeger is reachable

Do not share:

- Raw URLs containing identifiers
- Query strings
- Request or response bodies
- SQL statements or parameters
- Redis commands, scripts, keys, or values
- Authorization headers
- Cookies
- Access or refresh tokens
- CSRF values
- Passwords or password hashes
- Email addresses or IP addresses used for abuse protection
- Database or Redis connection URLs
- Signing seeds or HMAC keys
- Vault item payloads
- Encryption keys or passphrases

## API will not start

### Symptoms

- `bind: address already in use`
- Database initialization failed
- Redis initialization failed
- Invalid environment configuration

### Checks

Check whether another process owns the configured HTTP port:

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
```

Inspect Compose services:

```bash
docker compose -f deployments/compose.yaml ps
```

Check PostgreSQL and Redis logs:

```bash
make compose-logs
```

Validate environment variables without printing their values:

```bash
test -n "$DATABASE_URL" && echo "DATABASE_URL is set"
test -n "$REDIS_URL" && echo "REDIS_URL is set"
test -n "$ACCESS_TOKEN_ED25519_SEED_BASE64" && echo "signing seed is set"
test -n "$RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64" && echo "rate-limit HMAC key is set"
```

Never print the actual values.

### Recovery

Start dependencies:

```bash
make compose-up
```

Stop an old local API process only after confirming it is VaultForge:

```bash
kill "$(lsof -tiTCP:8080 -sTCP:LISTEN)"
```

Then restart the API.

## PostgreSQL unavailable

### Expected behavior

- API startup fails if PostgreSQL is unavailable.
- `/health/live` remains process-only when the API is already running.
- `/health/ready` returns `503 Service Unavailable`.
- Data-dependent routes return safe service-unavailable responses.
- Raw driver errors and connection details are not returned to clients.

### Checks

```bash
docker compose -f deployments/compose.yaml ps postgres
docker compose -f deployments/compose.yaml logs postgres
```

Verify readiness after recovery:

```bash
curl -s -i http://localhost:8080/health/ready
```

### Recovery

```bash
docker compose -f deployments/compose.yaml up -d --wait postgres
```

If migrations are missing:

```bash
make migrate-up
```

Do not delete the PostgreSQL volume unless intentionally resetting local data.

## Redis unavailable

### Expected behavior

- API startup fails if Redis is unavailable.
- `/health/ready` returns `503 Service Unavailable`.
- Registration, login, refresh, and authenticated mutations fail closed with
  safe dependency responses.
- Read-only vault and item requests do not require a Redis call during request
  handling.

### Checks

```bash
docker compose -f deployments/compose.yaml ps redis
docker compose -f deployments/compose.yaml logs redis
```

### Recovery

```bash
docker compose -f deployments/compose.yaml up -d --wait redis
```

Confirm readiness:

```bash
curl -s http://localhost:8080/health/ready
```

Do not inspect or publish Redis keys because they represent security-control
state, even though identities are HMAC-protected.

## Login lockout or rate limit

### Expected behavior

- Excess requests return `429 Too Many Requests`.
- Redis failure returns a safe `503` rather than bypassing security controls.
- Repeated invalid credentials can temporarily lock the normalized email and
  direct peer combination.
- Public responses do not reveal whether an account exists.

### Checks

Review only safe request metadata:

- Normalized route
- Status code
- Request ID
- Trace ID
- Timestamp

Do not log or inspect submitted credentials.

### Recovery

For normal local testing, wait for the configured window to expire.

A manual Redis reset should be used only against local synthetic data:

```bash
make redis-shell
```

Avoid broad key deletion in shared or production-like environments.

## Refresh-token replay detected

### Expected behavior

A replayed rotated refresh token revokes its entire session family. Existing
access tokens for that family stop working after the server-side session check.

### Checks

Use safe session metadata only:

- Session or family identifier when already allowed by the internal debugging
  context
- Revocation timestamp
- Request ID
- Trace ID
- Public status code

Never print the raw refresh token, its digest, cookies, or authorization header.

### Recovery

The user must log in again to create a new session family. Do not restore the
revoked family.

## Traces are missing

### Checks

Confirm tracing is enabled for the API process:

```bash
echo "${OTEL_TRACING_ENABLED:-false}"
```

Confirm the local services are running:

```bash
docker compose -f deployments/compose.yaml ps jaeger otel-collector
```

Inspect Collector and Jaeger logs:

```bash
make observability-logs
```

Confirm the Collector endpoint is reachable:

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:4318
```

An HTTP error response still confirms that the port is reachable.

Generate a fresh request after the API starts with tracing enabled:

```bash
curl -s http://localhost:8080/health/ready
```

Batch export can take a short moment before the trace appears.

### Expected failure behavior

Collector or Jaeger failure must not expose the configured endpoint or any
telemetry payload in application logs. The API records only the generic warning:

```text
OpenTelemetry export failed
```

Tracing is optional. Collector failure must not make normal API requests fail.

## Suspected telemetry leakage

Stop tracing immediately:

```bash
make observability-stop
```

Restart the API without `OTEL_TRACING_ENABLED=true`.

Preserve only the minimum safe evidence needed to reproduce the issue. Do not
attach the raw trace if it contains passwords, tokens, cookies, SQL parameters,
Redis keys, resource identifiers, or vault data.

Treat any exposed authentication or encryption material as compromised and
rotate it according to the applicable local or deployment procedure.

## Verification commands

Run the complete repository checks:

```bash
make verify
git diff --check
```

Validate Compose configuration:

```bash
docker compose -f deployments/compose.yaml config
```

The observability containers are intentionally excluded from the normal
`compose-up` dependency path and from required CI services.
