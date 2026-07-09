# VaultForge API

The VaultForge API is a Go HTTP service for account authentication, secure session management, authorization, PostgreSQL persistence, and browser-side encrypted vault workflows.

The API validates encrypted item envelope shape, stores ciphertext and nonce bytes, and returns encrypted payload envelopes. It does not receive vault passphrases, key-encryption keys, unwrapped vault data keys, or decrypted item payloads.

The current implementation includes:

- Account registration and login
- Argon2id password hashing behind a replaceable interface
- Ed25519-signed access tokens
- Opaque refresh tokens stored only as SHA-256 digests
- Refresh-token rotation with replay detection
- Host-only `HttpOnly`, `SameSite=Strict` refresh cookies
- Double-submit CSRF protection for refresh requests
- Refresh and CSRF cookie rotation and logout clearing
- Stateful access-token validation against active PostgreSQL sessions
- Session listing and revocation
- Owner-scoped vault workflows
- Complete encrypted vault-item lifecycle workflows
- Public, unauthenticated password generation and password-strength endpoints backed by the Rust password-service
- Keyset pagination
- Idempotency keys for item creation. Encrypted create retries must resend the exact same encrypted payload bytes for the same idempotency key; re-encrypting the same plaintext with a new nonce is treated as conflicting idempotency-key reuse.
- Strong `ETag` and `If-Match` optimistic concurrency
- Sanitized transactional outbox writes
- Optional OpenTelemetry tracing for HTTP, PostgreSQL, and Redis
- Trace IDs correlated with safe request logs
- Strict JSON decoding, safe public errors, request logging, panic recovery, and security headers

## Architecture

```text
HTTP request
    ↓
Chi router
    ↓
Global middleware
    ├── Bounded request ID
    ├── Safe OpenTelemetry request span
    ├── Safe request logging with trace correlation
    ├── Low-cardinality HTTP metrics
    ├── Panic recovery
    ├── Security headers
    └── Configurable request deadline
    ↓
Public authentication routes
    ├── Redis fixed-window request limit by direct peer IP
    ├── Login lockout check by normalized email plus direct peer IP
    └── Authentication or session handler
          ↓
      Authentication and session services
          ├── PasswordHasher
          ├── PostgreSQL stores
          └── Redis login-protection state

Protected routes
    ↓
Bearer authentication middleware
    ├── Verify the Ed25519 access token
    ├── Confirm the PostgreSQL session is active
    └── Add the authenticated principal to request context
    ↓
Read route or authenticated mutation
    ├── Read route: no Redis call during request handling
    └── Mutation: Redis fixed-window request limit by authenticated user ID
          ↓
      Session, vault, or item handler
          ↓
      Session or vault service
          ↓
      PostgreSQL transaction
          ├── Domain mutation
          └── Sanitized transactional audit intent
```

PostgreSQL is the durable source of truth for users, sessions, vaults, items, idempotency records, and sanitized transactional audit intent records.

Redis stores only bounded, temporary operational security state. It never stores vault payloads, passwords, tokens, keys, or durable idempotency records.

Handlers receive ownership from the authenticated principal. Clients cannot select an owner ID in request bodies or query parameters.

The HTTP handlers do not know which algorithm or language performs password hashing. The current `PasswordHasher` implementation uses the Rust gRPC hash service through the shared password-hashing boundary.

Vault-item payloads are encrypted browser-side and sent to the API as opaque encrypted envelopes.

## Package layout

```text
apps/api/
├── cmd/api/                  # Application entry point and dependency wiring
├── internal/
│   ├── api/
│   │   ├── authhandler/      # Registration, login, refresh, and login protection
│   │   ├── diagnostics/      # Sanitized build diagnostics
│   │   ├── health/           # Liveness and PostgreSQL-plus-Redis readiness
│   │   ├── itemhandler/      # Item lifecycle, pagination, ETag, and idempotency HTTP contracts
│   │   ├── metrics/          # Race-safe low-cardinality HTTP metrics
│   │   ├── middleware/       # Logging, limits, recovery, security, timeouts, and authentication
│   │   ├── passwordhandler/  # Public password generation and strength-check HTTP contracts
│   │   ├── request/          # Strict JSON and bodyless-request validation
│   │   ├── response/         # Shared JSON response contracts
│   │   ├── sessioncookie/    # Refresh-cookie and CSRF transport
│   │   ├── sessionhandler/   # Session listing and revocation handlers
│   │   └── vaulthandler/     # Vault lifecycle handlers
│   ├── auth/                 # Password policy, Argon2id, and account authentication
│   ├── buildinfo/            # Sanitized build version and commit metadata
│   ├── db/                   # PostgreSQL connection setup
│   ├── passwordclient/       # gRPC client boundary to the Rust password-service
│   ├── passwordpb/           # Generated password-service protobuf and gRPC code
│   ├── ratelimit/            # Redis scripts, opaque keys, request limits, and lockouts
│   ├── redisclient/          # Redis configuration, lifecycle, ping, and script execution
│   ├── session/              # Tokens, login, refresh, authentication, and sessions
│   ├── store/                # PostgreSQL stores and transactional operations
│   ├── telemetry/            # Optional OpenTelemetry configuration and provider
│   └── vault/                # Vault and item domain services and contracts
└── migrations/               # Versioned SQL migrations
```

## Requirements

From the repository root, install or configure:

- Go
- Docker with Docker Compose
- Make
- Staticcheck
- golang-migrate
- direnv

Create the local environment file:

```bash
cp .env.example .env
```

Generate separate local keys:

```bash
openssl rand -base64 32
openssl rand -base64 32
```

Place one generated value in `ACCESS_TOKEN_ED25519_SEED_BASE64` and the other in `RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64`. Do not reuse the same value.

Then load the environment:

```bash
direnv allow
```

## Run locally

From the repository root:

```bash
make db-setup
make dev-api
```

`make db-setup` starts PostgreSQL and Redis, creates the integration-test database when needed, and applies development migrations.

Default API address:

```text
http://localhost:8080
```

## Optional OpenTelemetry tracing

Tracing is disabled by default.

Start the local trace pipeline from the repository root:

```bash
make observability-up
```

Run the API with tracing enabled:

```bash
OTEL_TRACING_ENABLED=true \
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 \
make dev-api
```

Open Jaeger at:

```text
http://localhost:16686
```

The API exports:

- One server span per HTTP request
- Normalized Chi route patterns instead of raw request paths
- Sanitized PostgreSQL operation spans such as `postgres.select`
- Redis operation spans with command statements, keys, and values disabled
- Trace IDs in safe request logs

The API does not export request bodies, query strings, authorization headers,
cookies, tokens, SQL statements, SQL parameters, Redis commands, Redis keys,
Redis values, vault payloads, or raw resource identifiers.

Collector or Jaeger failure does not fail normal API requests. Export failures
produce only a generic warning without endpoint or payload details.

See [`../../docs/runbook.md`](../../docs/runbook.md) for the local trace workflow
and troubleshooting procedures.

## Health and operational routes

```text
GET /health
GET /health/live
GET /health/ready
GET /health/diagnostics
GET /internal/metrics
```

`/health` and `/health/live` verify that the process can respond. They do not query dependencies.

`/health/ready` checks PostgreSQL and Redis with a two-second deadline. It returns `503 Service Unavailable` if either required dependency is unavailable.

`/health/diagnostics` returns only:

```json
{
  "service": "vaultforge-api",
  "version": "sanitized-build-version",
  "commit": "sanitized-commit"
}
```

The Makefile injects version and commit metadata into development and verification builds. Invalid metadata falls back to safe defaults.

`/internal/metrics` emits Prometheus text for sanitized build information, process uptime, in-flight requests, completed-request counts, and request-duration sums and counts. Labels are limited to normalized method, Chi route pattern, and status class.

The metrics route accepts only direct loopback peers. It ignores forwarded-IP headers and returns `404 Not Found` to non-loopback callers. Diagnostics and metrics responses use `Cache-Control: no-store`.

## Redis rate limiting and login protection

Redis is required at startup and for security-sensitive request handling.

Default fixed-window policies:

| Scope                    | Identity              | Default policy            |
| ------------------------ | --------------------- | ------------------------- |
| Registration             | Direct peer IP        | 5 requests per 10 minutes |
| Login                    | Direct peer IP        | 20 requests per minute    |
| Refresh                  | Direct peer IP        | 30 requests per minute    |
| Password tools           | Direct peer IP        | 60 requests per minute    |
| Vault and item mutations | Authenticated user ID | 60 requests per minute    |

Login protection uses normalized email plus direct peer IP:

- The fifth invalid-credential failure within 15 minutes activates a 15-minute lockout.
- Active lockouts return `429 Too Many Requests` with a rounded-up `Retry-After` header.
- Successful login clears failed-login and lockout state on a best-effort basis after the session is created.
- Invalid email syntax uses a fixed sentinel identity rather than placing attacker-controlled input into key construction.

Redis keys use HMAC-SHA-256 over length-prefixed identity parts. Raw email addresses, IP addresses, user IDs, tokens, and other identity material are not stored in Redis keys or values.

The application ignores `X-Forwarded-For`, `X-Real-IP`, and similar headers until a trusted-proxy configuration exists.

Redis failure behavior:

- Registration, login, refresh, and authenticated mutations fail closed with safe `503` responses.
- Read-only vault and item routes do not call Redis during request handling.
- Readiness fails while Redis is unavailable.
- Liveness remains available.
- The Redis client and existing application process recover after Redis returns.

## Public password tools

```text
POST /v1/passwords/generate
POST /v1/passwords/strength
```

Both routes are public, require no `Authorization` header, and are backed by the Rust password-service over gRPC through the internal `passwordclient` boundary. They are rate-limited by direct peer IP using the same fixed-window policy (60 requests per minute) and always respond with `Cache-Control: no-store`.

These endpoints generate or rate synthetic password strings for the public password generator page and the registration form's live strength feedback. They never receive account credentials, vault passphrases, or vault data, and they are unrelated to the account-password `PasswordHasher` boundary or the Rust hash-service.

### Generate a password

```text
POST http://localhost:8080/v1/passwords/generate
Content-Type: application/json
```

```json
{
  "length": 20,
  "includeUppercase": true,
  "includeLowercase": true,
  "includeDigits": true,
  "includeSymbols": true,
  "excludeChars": ""
}
```

Successful response:

```text
200 OK
Cache-Control: no-store
```

```json
{
  "password": "generated-password-value",
  "entropyBits": 129.8
}
```

### Check password strength

```text
POST http://localhost:8080/v1/passwords/strength
Content-Type: application/json
```

```json
{
  "password": "candidate-password-to-rate"
}
```

Successful response:

```text
200 OK
Cache-Control: no-store
```

```json
{
  "score": 3,
  "label": "strong",
  "entropyBits": 61.2,
  "crackTimeEstimate": "3 years",
  "suggestions": ["Add another word or two. Uncommon words are better."]
}
```

An invalid request (for example, an unsatisfiable character-class combination) returns `422 Unprocessable Entity` with code `invalid_password_request`. Password-service unavailability, timeouts, and malformed upstream responses return `503 Service Unavailable` with code `password_tools_unavailable`. Neither endpoint logs or stores the submitted or generated password.

## Authentication routes

### Register

```text
POST http://localhost:8080/v1/auth/register
```

Thunder Client configuration:

```text
Method:  POST
Header:  Content-Type: application/json
Body:    JSON
```

```json
{
  "email": "developer@example.com",
  "password": "correct horse battery staple"
}
```

Successful response:

```text
201 Created
```

```json
{
  "user": {
    "id": "generated-uuid",
    "email": "developer@example.com",
    "status": "active",
    "createdAt": "timestamp",
    "updatedAt": "timestamp"
  }
}
```

### Login

```text
POST http://localhost:8080/v1/auth/login
```

Thunder Client configuration:

```text
Method:  POST
Header:  Content-Type: application/json
Header:  User-Agent: VaultForge-Thunder
Body:    JSON
```

```json
{
  "email": "developer@example.com",
  "password": "correct horse battery staple"
}
```

Successful response:

```text
200 OK
Cache-Control: no-store
```

```json
{
  "user": {
    "id": "generated-uuid",
    "email": "developer@example.com",
    "status": "active",
    "createdAt": "timestamp",
    "updatedAt": "timestamp"
  },
  "tokenType": "Bearer",
  "accessToken": "signed-access-token",
  "accessTokenExpiresAt": "timestamp",
  "refreshTokenExpiresAt": "timestamp"
}
```

The response does not expose the refresh token in JSON.

Login also sets two host-only cookies:

```text
vaultforge_refresh=[REDACTED]; Path=/v1/auth/refresh; HttpOnly; SameSite=Strict
vaultforge_csrf=[REDACTED]; Path=/; SameSite=Strict
```

Both cookies use the same absolute refresh-family expiration. Production adds the `Secure` flag. Local development omits it so the API can run over HTTP.

Each login creates a new session family. The supplied user agent is stored as session metadata.

### Refresh

```text
POST http://localhost:8080/v1/auth/refresh
```

Thunder Client must retain the cookies issued during login.

Configuration:

```text
Method:  POST
Header:  X-CSRF-Token: <current vaultforge_csrf cookie value>
Body:    None
```

Do not send JSON and do not set `Content-Type`. The request must be bodyless.

The browser or HTTP client sends:

- The `vaultforge_refresh` cookie automatically
- The readable `vaultforge_csrf` cookie automatically
- The same CSRF value in `X-CSRF-Token`

Successful response:

```text
200 OK
Cache-Control: no-store
```

```json
{
  "tokenType": "Bearer",
  "accessToken": "new-signed-access-token",
  "accessTokenExpiresAt": "timestamp",
  "refreshTokenExpiresAt": "timestamp"
}
```

A successful refresh:

1. Validates exactly one refresh cookie.
2. Validates exactly one CSRF cookie and one `X-CSRF-Token` header.
3. Requires the CSRF cookie and header to match.
4. Revokes the submitted refresh-token row.
5. Creates a replacement row in the same token family.
6. Preserves the family’s absolute expiration time.
7. Issues a new access token.
8. Rotates both the refresh and CSRF cookies.

The replacement refresh token is never included in JSON. After a successful refresh, clients must use the newly issued CSRF cookie value for the next refresh request.

A missing, malformed, duplicated, or mismatched CSRF value returns:

```text
403 Forbidden
```

```json
{
  "error": {
    "code": "csrf_validation_failed",
    "message": "The CSRF token is missing or invalid.",
    "request_id": "generated-request-id"
  }
}
```

A non-empty refresh request body returns:

```text
400 Bad Request
```

```json
{
  "error": {
    "code": "invalid_request",
    "message": "The refresh request must not contain a body.",
    "request_id": "generated-request-id"
  }
}
```

Refresh tokens are single-use. Reusing an already-rotated refresh token is treated as replay, and the entire token family is revoked.

Invalid, expired, revoked, replayed, malformed, and disabled-user refresh states return the same public response:

```text
401 Unauthorized
```

```json
{
  "error": {
    "code": "invalid_refresh_token",
    "message": "The refresh token is invalid or expired.",
    "request_id": "generated-request-id"
  }
}
```

Invalid refresh credentials also clear stale refresh and CSRF cookies.

## Protected session routes

Protected routes require exactly one bearer authorization header:

```text
Authorization: Bearer <access-token>
```

The API verifies both:

1. The access token’s Ed25519 signature and claims.
2. The matching session family is still active, unexpired, owned by the user, and associated with an active account.

Revoking a session therefore invalidates already-issued access tokens immediately instead of waiting for JWT expiration.

### List active sessions

```text
GET http://localhost:8080/v1/sessions
Authorization: Bearer <access-token>
```

Successful response:

```text
200 OK
```

```json
{
  "sessions": [
    {
      "id": "token-family-uuid",
      "userAgent": "VaultForge-Thunder",
      "createdAt": "timestamp",
      "expiresAt": "timestamp",
      "current": true
    }
  ]
}
```

A session ID is the token-family ID. Refresh-token rotation does not create a second logical session in this response.

### Logout the current session

```text
DELETE http://localhost:8080/v1/sessions/current
Authorization: Bearer <access-token>
```

Successful response:

```text
204 No Content
```

This revokes the authenticated token family. Its existing access and refresh tokens can no longer be used. The response also clears the current browser’s refresh and CSRF cookies.

### Revoke one owned session

```text
DELETE http://localhost:8080/v1/sessions/<session-id>
Authorization: Bearer <access-token>
```

Successful response:

```text
204 No Content
```

Revoking the current session by ID also clears the browser’s refresh and CSRF cookies. Revoking a different owned session does not clear the current browser cookies.

Unknown, already-revoked, and other users’ session IDs return the same public result:

```text
404 Not Found
```

```json
{
  "error": {
    "code": "session_not_found",
    "message": "The session was not found.",
    "request_id": "generated-request-id"
  }
}
```

### Logout all sessions

```text
DELETE http://localhost:8080/v1/sessions
Authorization: Bearer <access-token>
```

Successful response:

```text
204 No Content
```

This revokes every active session belonging to the authenticated user, including the session used to make the request, and clears the current browser’s refresh and CSRF cookies.

## Vault and item routes

All vault and item routes require:

```text
Authorization: Bearer <access-token>
```

Ownership comes exclusively from the authenticated principal. Unknown, malformed, deleted, and unowned resources use safe public not-found responses.

> Vault item payloads are encrypted browser-side before they reach the API; use synthetic data only. See [`../../README.md`](../../README.md) and [`../../docs/threat-model.md`](../../docs/threat-model.md) for the full encryption model.

### Vault routes

```text
POST   /v1/vaults
GET    /v1/vaults
GET    /v1/vaults/{vaultID}
PATCH  /v1/vaults/{vaultID}
DELETE /v1/vaults/{vaultID}
```

Vault creation and rename requests use:

```json
{
  "name": "Development"
}
```

Vault responses contain only safe metadata such as ID, name, and timestamps.

### Supported item types

```text
login
api_key
environment_variable
database_connection
secure_note
```

### Create an item

```text
POST /v1/vaults/{vaultID}/items
Authorization: Bearer <access-token>
Content-Type: application/json
Idempotency-Key: client-generated-key
```

```json
{
  "type": "api_key",
  "encryptedPayload": {
    "version": 1,
    "algorithm": "AES-256-GCM",
    "blob": "base64-encoded-nonce-plus-ciphertext"
  }
}
```

The browser encrypts the item payload with the Rust WASM crypto module before this request is sent; the API never receives plaintext. See [`../../README.md`](../../README.md) for the encryption model.

A successful creation returns:

```text
201 Created
ETag: "1"
Location: /v1/vaults/{vaultID}/items/{itemID}
```

Replaying the same idempotency key with the same normalized request returns the same item. Reusing it with different request data returns:

```text
409 Conflict
```

```json
{
  "error": {
    "code": "idempotency_conflict",
    "message": "The Idempotency-Key was already used with different request data.",
    "request_id": "generated-request-id"
  }
}
```

### List items

```text
GET /v1/vaults/{vaultID}/items
GET /v1/vaults/{vaultID}/items?state=deleted&limit=20&after=<opaque-cursor>
```

Query parameters:

| Parameter | Behavior                                                     |
| --------- | ------------------------------------------------------------ |
| `state`   | `active` by default; `deleted` selects soft-deleted items    |
| `limit`   | Defaults to `20`; allowed range is `1` through `100`         |
| `after`   | Opaque URL-safe keyset cursor returned by the preceding page |

Items are ordered by `updated_at DESC, id DESC`.

```json
{
  "items": [
    {
      "id": "generated-item-id",
      "type": "secure_note",
      "encryptedPayload": {
        "version": 1,
        "algorithm": "AES-256-GCM",
        "blob": "base64-encoded-nonce-plus-ciphertext"
      },
      "version": 1,
      "createdAt": "timestamp",
      "updatedAt": "timestamp"
    }
  ],
  "nextCursor": "opaque-cursor-when-another-page-exists"
}
```

The parent vault ID and owner ID are not repeated in public item resources.

### Retrieve an item

```text
GET /v1/vaults/{vaultID}/items/{itemID}
GET /v1/vaults/{vaultID}/items/{itemID}?state=deleted
```

Successful retrieval returns the current strong version:

```text
200 OK
ETag: "2"
```

### Update an item

```text
PUT /v1/vaults/{vaultID}/items/{itemID}
Authorization: Bearer <access-token>
Content-Type: application/json
If-Match: "2"
```

```json
{
  "type": "secure_note",
  "encryptedPayload": {
    "version": 1,
    "algorithm": "AES-256-GCM",
    "blob": "base64-encoded-nonce-plus-ciphertext"
  }
}
```

A successful update increments the version and returns the replacement `ETag`.

Missing `If-Match` returns `428 Precondition Required`. A malformed header returns `400 Bad Request`. A stale version returns:

```text
412 Precondition Failed
```

```json
{
  "error": {
    "code": "item_version_conflict",
    "message": "The item changed after the supplied version was retrieved.",
    "request_id": "generated-request-id"
  }
}
```

### Soft-delete, restore, and permanently delete

```text
DELETE /v1/vaults/{vaultID}/items/{itemID}
POST   /v1/vaults/{vaultID}/items/{itemID}/restore
DELETE /v1/vaults/{vaultID}/items/{itemID}/permanent
```

Each request requires:

```text
If-Match: "<current-version>"
```

Soft deletion returns the deleted resource and keeps the current version. Restoration clears `deletedAt` and increments the version. Permanent deletion is allowed only for a currently deleted item and returns:

```text
204 No Content
```

### Item response example

```json
{
  "item": {
    "id": "generated-item-id",
    "type": "api_key",
    "encryptedPayload": {
      "version": 1,
      "algorithm": "AES-256-GCM",
      "blob": "base64-encoded-nonce-plus-ciphertext"
    },
    "version": 1,
    "createdAt": "timestamp",
    "updatedAt": "timestamp",
    "deletedAt": "timestamp only when deleted"
  }
}
```

## Authentication behavior

### Email handling

Registration and login:

- Trim surrounding whitespace
- Convert email addresses to lowercase
- Preserve dots and plus aliases
- Reject malformed and oversized addresses
- Avoid provider-specific normalization

### Password handling

Account passwords:

- Must contain valid UTF-8
- Are normalized using Unicode NFC
- Must contain between 15 and 128 Unicode code points
- Are never trimmed or silently truncated
- Do not require arbitrary uppercase, number, or symbol rules

The current local adapter hashes passwords with Argon2id using a fresh cryptographically secure random salt for each account.

PostgreSQL stores:

```text
password_hash
password_algorithm
```

The plaintext password is never stored.

### Login failures

Unknown email addresses, incorrect passwords, invalid login input, and disabled accounts return the same public response:

```text
401 Unauthorized
```

```json
{
  "error": {
    "code": "invalid_credentials",
    "message": "The email or password is incorrect.",
    "request_id": "generated-request-id"
  }
}
```

This reduces account-enumeration clues.

Each invalid-credential failure increments the Redis login-protection state for normalized email plus direct peer IP. Reaching the configured threshold activates a temporary lockout:

```text
429 Too Many Requests
Retry-After: <seconds>
```

```json
{
  "error": {
    "code": "login_locked",
    "message": "Too many failed login attempts. Try again later.",
    "request_id": "generated-request-id"
  }
}
```

Redis unavailability blocks login with a safe `503 Service Unavailable` response rather than allowing unlimited password attempts.

### Access-token failures

Missing, malformed, invalid, expired, revoked-session, and disabled-account access tokens return:

```text
401 Unauthorized
WWW-Authenticate: Bearer
```

```json
{
  "error": {
    "code": "unauthorized",
    "message": "A valid access token is required.",
    "request_id": "generated-request-id"
  }
}
```

Dependency failures return a safe `503 Service Unavailable` response instead of exposing internal details.

## Token storage and session security

Access tokens:

- Are signed with Ed25519
- Include the user ID as the subject
- Include the token-family ID in the `sid` claim
- Include issuer, audience, issued-at, not-before, and expiration claims
- Are returned in login and refresh JSON responses
- Must be held only in client memory
- Must not be stored in `localStorage`, `sessionStorage`, IndexedDB, URLs, logs, or error reports
- Are never stored in PostgreSQL

Refresh tokens:

- Are cryptographically random opaque values
- Are never returned in JSON
- Are delivered through a host-only `HttpOnly` cookie
- Use `Path=/v1/auth/refresh`
- Use `SameSite=Strict`
- Use `Secure` in production
- Are stored in PostgreSQL only as SHA-256 digests
- Are rotated after each successful use
- Preserve the token family’s absolute expiration
- Use family-wide revocation after replay detection

CSRF protection:

- Uses a separate readable `vaultforge_csrf` cookie
- Requires exactly one `X-CSRF-Token` header
- Requires the cookie and header values to match
- Rotates the CSRF token with every successful refresh
- Rejects missing, duplicated, malformed, or mismatched values

Login and refresh responses use `Cache-Control: no-store`.

## Request protection

Registration and login JSON requests must:

- Use `Content-Type: application/json`
- Contain exactly one JSON object
- Use only documented fields
- Remain within the authentication body limit
- Remain within validated email and password bounds

Refresh requests must contain no body and must use the cookie and CSRF-header contract described above.

Global and route-specific bounds include:

- 32 KiB aggregate request-header limit
- 4 KiB bearer-token limit
- 4 KiB authentication body limit
- 4 KiB item raw-query limit
- Item page limits from 1 through 100
- Bounded opaque item cursors
- 32-byte `If-Match` limit
- 128-byte sanitized incoming request IDs
- 256-byte session, vault, item, principal, and correlation identifiers
- Existing vault names, item titles, payloads, idempotency keys, and user-agent bounds

Examples:

| Condition                                   |                       Status |
| ------------------------------------------- | ---------------------------: |
| Malformed JSON                              |            `400 Bad Request` |
| Malformed or oversized query                |            `400 Bad Request` |
| Missing or invalid access token             |           `401 Unauthorized` |
| Incorrect credentials                       |           `401 Unauthorized` |
| Invalid refresh token                       |           `401 Unauthorized` |
| Missing or invalid refresh CSRF token       |              `403 Forbidden` |
| Non-empty refresh request body              |            `400 Bad Request` |
| Unknown or unowned session                  |              `404 Not Found` |
| Duplicate email                             |               `409 Conflict` |
| Item idempotency conflict                   |               `409 Conflict` |
| Rate limit exceeded                         |      `429 Too Many Requests` |
| Login temporarily locked                    |      `429 Too Many Requests` |
| Stale item version                          |    `412 Precondition Failed` |
| Missing item `If-Match`                     |  `428 Precondition Required` |
| Oversized request                           |      `413 Content Too Large` |
| Unsupported content type                    | `415 Unsupported Media Type` |
| Invalid registration field                  |   `422 Unprocessable Entity` |
| Authentication dependency unavailable       |    `503 Service Unavailable` |
| Required application dependency unavailable |    `503 Service Unavailable` |

Rate-limit and lockout responses include `Retry-After`.

All API errors use this structure:

```json
{
  "error": {
    "code": "error_code",
    "message": "Safe public message.",
    "request_id": "generated-request-id"
  }
}
```

## Database and migrations

Start PostgreSQL and Redis and prepare the development and test databases:

```bash
make db-setup
```

Apply pending migrations:

```bash
make migrate-up
```

Roll back one migration:

```bash
make migrate-down
```

Show the current version:

```bash
make migrate-version
```

Open the development database:

```bash
make db-shell
```

Open the Redis CLI:

```bash
make redis-shell
```

PostgreSQL stores durable application state, including item idempotency records. Redis stores temporary operational security state only and is configured without persistence in local Compose.

## Testing

From the repository root:

```bash
make test-api
```

Or from `apps/api`:

```bash
go test -race -count=1 ./...
```

`TEST_DATABASE_URL` must point to the dedicated `vaultforge_test` database. `TEST_REDIS_URL` must point to an isolated Redis database number.

The integration suite:

1. Rolls the PostgreSQL test schema back to version zero.
2. Applies all migrations.
3. Opens a real PostgreSQL pool and Redis client.
4. Tests schema rules and PostgreSQL repositories.
5. Tests Redis connectivity, safe connection failures, scripts, TTLs, and opaque key behavior.
6. Tests registration, login cookie issuance, login-issued CSRF use, refresh-cookie and CSRF rotation, replay detection, authorization, session listing, and revocation through the real HTTP stack.
7. Tests registration, login, refresh, and mutation request limits.
8. Tests failed-login counting, lockout activation, expiration, and clearing.
9. Tests owner-scoped vault and item workflows.
10. Tests item pagination, idempotency, optimistic concurrency, soft deletion, restoration, and permanent deletion.
11. Verifies sanitized transactional audit intent records remain atomic with domain mutations.
12. Verifies sanitized audit metadata never contains item payloads, keys, names, hashes, or other secret values.
13. Verifies Redis keys and values contain no raw identities or secret markers.
14. Verifies request deadlines, cancellation, dependency errors, and input bounds.
15. Verifies build diagnostics and metrics remain sanitized and race-safe.
16. Verifies OpenTelemetry configuration, normalized route spans, trace correlation, and telemetry redaction.
17. Verifies cross-user ownership isolation and immediate invalidation of revoked access tokens.
18. Rolls the test database back to version zero.

Controlled runtime scripts additionally verify PostgreSQL outage and recovery, Redis outage and recovery, diagnostics metadata, normalized metrics routes, and secret exclusion.

The integration tests use the real:

- Chi router
- HTTP middleware
- Authentication, session, vault, item, diagnostics, health, and metrics handlers
- Authentication, session, and vault services
- Argon2id adapter
- Ed25519 token manager
- PostgreSQL stores and transactions
- Redis client and Lua scripts

## Quality checks

From the repository root:

```bash
make format
make format-check
make lint
make mod-verify
make build-api
make verify
```

`make build-api` injects sanitized Git version and commit metadata and writes the temporary binary outside the repository.

`make verify` runs Go formatting checks, module verification, `go vet`, Staticcheck, the complete race-enabled Go test suite, the API metadata build, frontend checks and tests, the frontend production build, and real-stack Playwright E2E.

GitHub Actions runs Go checks with PostgreSQL and Redis, web checks, browser E2E with PostgreSQL and Redis, and Gitleaks secret scanning for pushed commits and pull requests.

## API contract and fuzz testing

The maintained OpenAPI contract is:

```text
openapi.yaml
```

`internal/api/openapi_contract_test.go` validates the document, compares its
routes and methods with the real Chi router, and validates representative
authentication, vault, item, concurrency, and error requests and responses.

The normal Go test suite also runs the seed corpus for focused fuzz tests
covering:

- Item cursor decoding
- Strong `ETag` and `If-Match` parsing
- Bearer-token parsing

These checks run through `make test-api`, `make verify`, and the Go GitHub
Actions job.

See [`../../docs/testing.md`](../../docs/testing.md) for the complete testing
strategy and optional deeper fuzzing commands.

## Security rules

Never log, meter, store in Redis, or return outside the documented authentication response and cookie transport:

- Request bodies
- Plaintext passwords
- Encoded password hashes
- Authorization headers
- Cookies
- Access or refresh tokens
- Refresh-token digests
- CSRF token values
- Database or Redis URLs
- Signing seeds or private keys
- Rate-limit identity HMAC keys
- Raw rate-limit identities
- Raw URLs, query strings, SQL statements, SQL parameters, Redis commands, Redis keys, or Redis values in telemetry
- Raw vault, item, session, user, email, or IP identifiers in span names or attributes
- Vault passphrases
- Encryption keys
- Item payloads or decrypted vault contents
- Raw dependency errors

Redis keys and values must remain bounded, short-lived, and opaque. PostgreSQL remains authoritative for durable data and idempotency.

Metrics use normalized route patterns instead of raw paths. Diagnostics expose only sanitized build identity. The metrics endpoint accepts only direct loopback peers and ignores forwarded-IP headers.

Use synthetic test data only. VaultForge is not independently audited and must not be used for real credentials or production secrets.
