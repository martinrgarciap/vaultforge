# VaultForge API

The VaultForge API is a Go HTTP service for account authentication, secure session management, authorization, PostgreSQL persistence, and future encrypted vault workflows.

The current implementation includes:

- Account registration and login
- Argon2id password hashing behind a replaceable interface
- Ed25519-signed access tokens
- Opaque refresh tokens stored only as SHA-256 digests
- Refresh-token rotation with replay detection
- Stateful access-token validation against active PostgreSQL sessions
- Session listing, targeted revocation, current-session logout, and logout-all
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
Public authentication routes
    └── Authentication handler
          ↓
      Authentication and session services
          ├── PasswordHasher
          ├── AccessTokenProvider
          ├── RefreshTokenProvider
          ├── UserStore
          └── SessionStore
                ↓
            PostgreSQL

Protected session routes
    ↓
Bearer authentication middleware
    ├── Verify Ed25519 access token
    ├── Load the active session family from PostgreSQL
    └── Add the authenticated principal to request context
    ↓
Session handler
    ↓
Session service
    ↓
PostgreSQL
```

The HTTP handlers do not know which algorithm or language performs password hashing.

The current `PasswordHasher` implementation is a local Go Argon2id adapter. A future Rust gRPC adapter can replace it without changing the handlers or public API.

## Package layout

```text
apps/api/
├── cmd/api/                  # Application entry point and dependency wiring
├── internal/
│   ├── api/
│   │   ├── authhandler/      # Registration, login, and refresh HTTP handlers
│   │   ├── health/           # Liveness and readiness handlers
│   │   ├── middleware/       # Logging, recovery, security, and bearer authentication
│   │   ├── request/          # Strict JSON request decoding
│   │   ├── response/         # Shared JSON response contracts
│   │   └── sessionhandler/   # Session listing and revocation HTTP handlers
│   ├── auth/                 # Password policy, Argon2id, and account authentication
│   ├── db/                   # PostgreSQL connection setup
│   ├── session/              # Tokens, login, refresh, authentication, and session orchestration
│   └── store/                # PostgreSQL persistence
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
  "refreshToken": "opaque-refresh-token",
  "refreshTokenExpiresAt": "timestamp"
}
```

Each login creates a new session family. The supplied user agent is stored as session metadata.

### Refresh

```text
POST http://localhost:8080/v1/auth/refresh
```

Thunder Client configuration:

```text
Method:  POST
Header:  Content-Type: application/json
Body:    JSON
```

```json
{
  "refreshToken": "opaque-refresh-token"
}
```

Successful response:

```text
200 OK
```

```json
{
  "tokenType": "Bearer",
  "accessToken": "new-signed-access-token",
  "accessTokenExpiresAt": "timestamp",
  "refreshToken": "new-opaque-refresh-token",
  "refreshTokenExpiresAt": "timestamp"
}
```

Refresh tokens are single-use. A successful refresh:

1. Revokes the submitted refresh-token row.
2. Creates a replacement row in the same token family.
3. Preserves the family’s absolute expiration time.
4. Issues a new access token for the same user and session family.

Reusing an already-rotated refresh token is treated as replay. The entire token family is revoked.

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

This revokes the authenticated token family. Its existing access and refresh tokens can no longer be used.

### Revoke one owned session

```text
DELETE http://localhost:8080/v1/sessions/<session-id>
Authorization: Bearer <access-token>
```

Successful response:

```text
204 No Content
```

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

This revokes every active session belonging to the authenticated user, including the session used to make the request.

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
- Are never stored in PostgreSQL

Refresh tokens:

- Are cryptographically random opaque values
- Are returned in JSON during the current backend-only phase
- Are stored in PostgreSQL only as SHA-256 digests
- Are rotated after each successful use
- Use family-wide revocation after replay detection

Frontend cookies and CSRF protection are deferred until the frontend integration stage.

## Request protection

Authentication JSON requests must:

- Use `Content-Type: application/json`
- Contain exactly one JSON object
- Use only documented fields
- Remain within the configured request-body limit

Examples:

| Condition                             |                       Status |
| ------------------------------------- | ---------------------------: |
| Malformed JSON                        |            `400 Bad Request` |
| Missing or invalid access token       |           `401 Unauthorized` |
| Incorrect credentials                 |           `401 Unauthorized` |
| Invalid refresh token                 |           `401 Unauthorized` |
| Unknown or unowned session            |              `404 Not Found` |
| Duplicate email                       |               `409 Conflict` |
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
5. Tests registration, login, refresh rotation, replay detection, authorization, session listing, targeted revocation, current logout, and logout-all through the real HTTP stack.
6. Verifies ownership isolation and immediate invalidation of revoked access tokens.
7. Rolls the test database back to version zero.

The authentication and session integration tests use the real:

- Chi router
- HTTP middleware
- Authentication and session handlers
- Authentication and session services
- Argon2id adapter
- Ed25519 token manager
- PostgreSQL stores

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

Never log or return:

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
