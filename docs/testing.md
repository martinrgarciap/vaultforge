# VaultForge Testing and QA

VaultForge uses layered automated testing to verify domain behavior, HTTP
contracts, real dependency integration, browser workflows, reliability, and
security regressions.

VaultForge is a portfolio and learning project. All tests must use synthetic
data. Tests must never print plaintext credentials, tokens, cookies, signing
keys, HMAC keys, database URLs, Redis URLs, vault passphrases, encryption keys,
or real vault values.

## Testing layers

### Go unit and service tests

Go unit tests cover:

- Authentication and password policy
- Access-token and refresh-token behavior
- Session orchestration
- Vault and item domain rules
- Pagination and cursor handling
- Idempotency
- Optimistic concurrency
- Rate-limit configuration
- Build metadata sanitization
- Request and identifier bounds

Service tests use fakes for repositories, clocks, password hashers, token
providers, and other replaceable dependencies.

### HTTP handler and middleware tests

HTTP tests cover:

- Request validation
- Authentication and authorization
- Public status codes and error envelopes
- Refresh cookies and CSRF validation
- Security headers
- Request IDs
- Request timeouts
- Panic recovery
- Rate-limit enforcement
- Loopback-only internal routes
- Normalized OpenTelemetry route spans and trace correlation
- Body, header, query, token, cursor, and identifier bounds

### PostgreSQL integration tests

PostgreSQL integration tests use the dedicated `vaultforge_test` database and
verify:

- Migrations and schema constraints
- Registration and login persistence
- Session creation, rotation, replay detection, and revocation
- Owner-scoped vault and item access
- Item lifecycle operations
- Durable idempotency
- Optimistic-concurrency conflicts
- Transactional outbox writes
- Dependency outage behavior and recovery

### Redis integration tests

Redis integration tests use an isolated Redis database and verify:

- Atomic fixed-window limits
- Concurrent request enforcement
- Counter and lockout expiration
- Failed-login lockouts
- Successful-login clearing
- Opaque HMAC-derived identities
- Absence of raw email and IP values in Redis keys
- Redis outage behavior and recovery

### API contract tests

The maintained OpenAPI document is:

```text
apps/api/openapi.yaml
```

The contract test is:

```text
apps/api/internal/api/openapi_contract_test.go
```

It verifies:

- The OpenAPI document is structurally valid
- Registered Chi routes and documented routes remain synchronized
- Critical authentication, vault, item, versioning, and error contracts satisfy
  the OpenAPI schemas
- Refresh cookies, CSRF headers, bearer authentication, idempotency keys,
  `ETag`, and `If-Match` remain documented

The contract test runs as part of the normal Go test suite and therefore also
runs in `make verify` and GitHub Actions.

### Fuzz tests

Focused Go fuzz tests protect parsers that receive untrusted input:

```text
apps/api/internal/api/itemhandler/cursor_fuzz_test.go
apps/api/internal/api/itemhandler/headers_fuzz_test.go
apps/api/internal/api/middleware/authentication_fuzz_test.go
```

Normal Go test execution runs their seed corpus.

Optional deeper local fuzzing can be run with:

```bash
cd apps/api

go test ./internal/api/itemhandler \
  -run=^$ \
  -fuzz=FuzzDecodeItemCursor \
  -fuzztime=10s

go test ./internal/api/itemhandler \
  -run=^$ \
  -fuzz=FuzzParseItemETag \
  -fuzztime=10s

go test ./internal/api/middleware \
  -run=^$ \
  -fuzz=FuzzBearerToken \
  -fuzztime=10s
```

Long-running fuzzing is optional and is not required for every commit.

### Telemetry tests

OpenTelemetry tests verify:

- Tracing is disabled by default
- Enabled endpoints are validated without exposing credentials
- HTTP spans use normalized Chi route patterns
- Raw paths, query strings, authorization values, and resource identifiers are
  excluded from spans
- PostgreSQL span names expose only high-level operation types
- Redis tracing disables command statements, keys, and values
- Request logs include trace IDs only when a valid span context exists
- Export failures use generic warnings

The tests use in-memory span exporters and do not require the local Collector or
Jaeger containers.

### Frontend unit and component tests

Vitest and React Testing Library cover:

- API helpers and runtime response parsing
- Authentication restoration
- Registration and login
- Route guards
- Vault workflows
- Item workflows
- Version-conflict handling
- Session management
- Loading, empty, retryable error, unauthorized, and not-found states

### Official system smoke test

The official VaultForge full-stack smoke test is:

```text
apps/web/e2e/vaultforge.spec.ts
```

It runs through Playwright against the real:

- React application
- Go API
- PostgreSQL database
- Redis instance

The workflow verifies:

- Registration and login
- In-memory access-token handling
- Empty `localStorage`, `sessionStorage`, and IndexedDB
- Sensitive-value masking before explicit reveal
- Clipboard copy feedback without browser-persistence or URL leakage
- Absence of synthetic passwords in browser console messages and page errors
- `HttpOnly` refresh-cookie behavior
- Authentication restoration after reload
- Vault creation
- Item creation and editing
- A real stale-version conflict between browser sessions
- Soft deletion
- Restoration
- Permanent deletion
- Session listing
- Current-session logout
- Protected-route rejection after logout
- Accessibility checks
- Phone, tablet, and desktop overflow checks

This Playwright workflow is the maintained complete account-to-item smoke path.
A duplicate command-line or Postman workflow is intentionally not maintained.

## Security regression coverage

Automated tests include regression coverage for:

- Cross-user session, vault, and item access
- Malformed and invalid access tokens
- Replayed refresh tokens
- Duplicate and conflicting idempotency keys
- Stale item versions
- Oversized request bodies
- Oversized headers and bearer tokens
- Oversized or malformed queries and cursors
- Redis identity leakage
- Outbox metadata leakage
- Metrics, diagnostics, and telemetry leakage
- Raw route, SQL, Redis, token, cookie, or resource-identifier leakage into spans
- PostgreSQL and Redis failures
- Concurrent rate-limit enforcement
- Clipboard clearing success, failure, replacement, and cleanup behavior
- Temporary copy confirmation and timed reveal behavior
- Inactivity and hidden-tab sensitive-value resets
- Modal focus containment and restoration
- Browser storage, URL, console, and page-error leakage

Security regressions must fail the build.

## Quality policy

VaultForge prioritizes critical-path and security coverage over a single total
coverage percentage.

Every bug fix must include a regression test that fails without the fix and
passes with it.

Test fixtures must:

- Use synthetic identities and values
- Avoid real credentials
- Avoid committed signing or HMAC keys
- Avoid printing request bodies or authentication material on failure
- Avoid placing tokens, cookies, or secrets in test names
- Avoid storing access tokens in browser persistence

Tests may report safe structural information such as a route, status code,
field name, expected type, or synthetic resource identifier.

## Commands

Run backend and frontend tests:

```bash
make test
```

Run the API suite with the race detector:

```bash
make test-api
```

Run frontend unit and component tests:

```bash
make test-web
```

Run the official full-stack smoke test:

```bash
make test-e2e
```

Run formatting, static analysis, builds, integration tests, contract tests,
security regressions, and the full-stack smoke test:

```bash
make verify
```

## Continuous integration

GitHub Actions runs:

- Go formatting, module verification, Vet, Staticcheck, race-enabled tests,
  OpenAPI contract validation, PostgreSQL integration tests, and Redis
  integration tests
- Frontend formatting, ESLint, TypeScript, Vitest, and production build
- The real-stack Playwright smoke test with PostgreSQL and Redis
- In-memory OpenTelemetry safety and configuration tests without requiring Collector or Jaeger
- Gitleaks against repository history

CI recreates its databases, applies migrations, starts required dependencies,
and tests a complete user workflow.
