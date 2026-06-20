# VaultForge API

The VaultForge API is a Go HTTP service responsible for account authentication, session management, authorization, vault metadata, encrypted payload persistence, and later backend reliability workflows.

The current implementation includes account registration and login through a replaceable password-hashing boundary.

## Architecture

```text
HTTP request
    ↓
Chi router and middleware
    ↓
Authentication handler
    ↓
Authentication service
    ├── PasswordHasher
    └── UserStore
          ↓
      PostgreSQL
```

The HTTP handler does not know which algorithm or language performs password hashing.

The current `PasswordHasher` implementation is a local Go Argon2id adapter. A future Rust gRPC adapter can replace it without changing the handler or public API.

## Package layout

```text
apps/api/
├── cmd/api/                  # Application entry point and dependency wiring
├── internal/
│   ├── api/
│   │   ├── authhandler/      # Registration and login HTTP handlers
│   │   ├── health/           # Liveness and readiness handlers
│   │   ├── middleware/       # Logging, recovery, and security headers
│   │   ├── request/          # Strict JSON request decoding
│   │   └── response/         # Shared JSON response contracts
│   ├── auth/                 # Password policy, Argon2id, and auth service
│   ├── db/                   # PostgreSQL connection setup
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

Login currently validates the account and returns the safe user representation. It does not issue a session or token yet.

Session creation, access tokens, refresh-token rotation, and revocation belong to the next roadmap phase.

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

## Request protection

Authentication requests must:

- Use `Content-Type: application/json`
- Contain exactly one JSON object
- Use only documented fields
- Remain within the configured request-body limit

Examples:

| Condition                             |                       Status |
| ------------------------------------- | ---------------------------: |
| Malformed JSON                        |            `400 Bad Request` |
| Incorrect credentials                 |           `401 Unauthorized` |
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
4. Tests repositories, schema behavior, registration, and login.
5. Rolls the test database back to version zero.

The authentication integration tests use the real:

- Chi router
- HTTP handler
- Authentication service
- Argon2id adapter
- PostgreSQL store

## Quality checks

From the repository root:

```bash
make format
make format-check
make lint
make mod-verify
make verify
```

## Security rules

Never log or return:

- Request bodies
- Plaintext passwords
- Encoded password hashes
- Authorization headers
- Cookies
- Access or refresh tokens
- Database URLs
- Vault passphrases
- Encryption keys
- Decrypted vault contents

Use synthetic test data only.
