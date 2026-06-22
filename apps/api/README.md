# VaultForge API

The VaultForge API is a Go HTTP service for account authentication, secure session management, authorization, PostgreSQL persistence, and future encrypted vault workflows.

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
- Complete synthetic vault-item lifecycle workflows
- Keyset pagination
- Idempotency keys for item creation
- Strong `ETag` and `If-Match` optimistic concurrency
- Sanitized transactional outbox writes
- Strict JSON decoding, safe public errors, request logging, panic recovery, and security headers

## Architecture

```text
HTTP request
    ↓
Chi router
    ↓
Global middleware
    ├── Request ID
    ├── Safe request logging
    ├── Panic recovery
    ├── Security headers
    └── Request timeout
    ↓
Public routes
    └── Authentication handler
          ↓
      Authentication and session services
          ↓
      PostgreSQL

Protected routes
    ↓
Bearer authentication middleware
    ├── Verify the Ed25519 access token
    ├── Confirm the PostgreSQL session is active
    └── Add the authenticated principal to request context
    ↓
Session, vault, or item handler
    ↓
Session or vault service
    ↓
PostgreSQL transaction
    ├── Domain mutation
    └── Sanitized outbox event
```

Handlers receive ownership from the authenticated principal. Clients cannot select an owner ID in request bodies or query parameters.

The HTTP handlers do not know which algorithm or language performs password hashing. The current `PasswordHasher` implementation is a local Go Argon2id adapter. A future Rust gRPC adapter can replace it without changing the handlers or public API.

Vault-item payloads are currently synthetic generic JSON. The contract is intentionally generic so a later encrypted envelope can replace the synthetic payload without redesigning the item routes.

## Package layout

```text
apps/api/
├── cmd/api/                  # Application entry point and dependency wiring
├── internal/
│   ├── api/
│   │   ├── authhandler/      # Registration, login, and refresh handlers
│   │   ├── health/           # Liveness and readiness handlers
│   │   ├── itemhandler/      # Item lifecycle, pagination, ETag, and idempotency HTTP contracts
│   │   ├── middleware/       # Logging, recovery, security, and bearer authentication
│   │   ├── request/          # Strict JSON and bodyless-request validation
│   │   ├── response/         # Shared JSON response contracts
│   │   ├── sessioncookie/    # Refresh-cookie and CSRF transport
│   │   ├── sessionhandler/   # Session listing and revocation handlers
│   │   └── vaulthandler/     # Vault lifecycle handlers
│   ├── auth/                 # Password policy, Argon2id, and account authentication
│   ├── db/                   # PostgreSQL connection setup
│   ├── session/              # Tokens, login, refresh, authentication, and sessions
│   ├── store/                # PostgreSQL stores and transactional operations
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
direnv allow
```

## Run locally

From the repository root:

```bash
make db-setup
make dev
```

Default address:

```text
http://localhost:8080
```

## Health routes

```text
GET /health
GET /health/live
GET /health/ready
```

`/health` and `/health/live` verify that the process can respond.

`/health/ready` also checks PostgreSQL with a short timeout and returns `503 Service Unavailable` when the database is unavailable.

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

> The current item payload is synthetic dummy JSON. Do not store real credentials. Browser-side encryption has not been implemented yet.

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
  "payload": {
    "label": "Synthetic development key",
    "token": "synthetic-value-only"
  }
}
```

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
      "payload": {
        "value": "synthetic"
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
  "payload": {
    "value": "updated synthetic value"
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
    "payload": {
      "label": "Synthetic key",
      "token": "synthetic-value-only"
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
- Remain within the configured request-body limit

Refresh requests must contain no body and must use the cookie and CSRF-header contract described above.

Examples:

| Condition                             |                       Status |
| ------------------------------------- | ---------------------------: |
| Malformed JSON                        |            `400 Bad Request` |
| Missing or invalid access token       |           `401 Unauthorized` |
| Incorrect credentials                 |           `401 Unauthorized` |
| Invalid refresh token                 |           `401 Unauthorized` |
| Missing or invalid refresh CSRF token |              `403 Forbidden` |
| Non-empty refresh request body        |            `400 Bad Request` |
| Unknown or unowned session            |              `404 Not Found` |
| Duplicate email                       |               `409 Conflict` |
| Item idempotency conflict             |               `409 Conflict` |
| Stale item version                    |    `412 Precondition Failed` |
| Missing item `If-Match`               |  `428 Precondition Required` |
| Oversized request                     |      `413 Content Too Large` |
| Unsupported content type              | `415 Unsupported Media Type` |
| Invalid registration field            |   `422 Unprocessable Entity` |
| Authentication dependency unavailable |    `503 Service Unavailable` |

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

Start PostgreSQL and prepare both databases:

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

## Testing

From the repository root:

```bash
make test
```

Or from `apps/api`:

```bash
go test -race -count=1 ./...
```

`TEST_DATABASE_URL` must point to the dedicated `vaultforge_test` database.

The integration suite:

1. Rolls the test schema back to version zero.
2. Applies all migrations.
3. Opens a real PostgreSQL pool.
4. Tests schema rules and PostgreSQL repositories.
5. Tests registration, login cookie issuance, login-issued CSRF use, refresh-cookie and CSRF rotation, replay detection, authorization, session listing, and revocation through the real HTTP stack.
6. Tests owner-scoped vault and item workflows.
7. Tests item pagination, idempotency, optimistic concurrency, soft deletion, restoration, and permanent deletion.
8. Verifies transactional outbox writes remain atomic with domain mutations.
9. Verifies sanitized audit metadata never contains item payloads, keys, names, hashes, or other secret values.
10. Verifies cross-user ownership isolation and immediate invalidation of revoked access tokens.
11. Rolls the test database back to version zero.

The integration tests use the real:

- Chi router
- HTTP middleware
- Authentication, session, vault, and item handlers
- Authentication, session, and vault services
- Argon2id adapter
- Ed25519 token manager
- PostgreSQL stores and transactions

## Quality checks

From the repository root:

```bash
make format
make format-check
make lint
make mod-verify
make verify
```

`make verify` runs formatting checks, module verification, `go vet`, Staticcheck, and the complete race-enabled Go test suite.

GitHub Actions runs the repository verification workflow for pushed commits and pull requests.

## Security rules

Never log or return outside the documented authentication response and cookie transport:

- Request bodies
- Plaintext passwords
- Encoded password hashes
- Authorization headers
- Cookies
- Access or refresh tokens
- Refresh-token digests
- Database URLs
- Vault passphrases
- Encryption keys
- Decrypted vault contents

Use synthetic test data only.
