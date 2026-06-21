# VaultForge

VaultForge is a backend-first developer secrets vault built to demonstrate secure API design, authentication, PostgreSQL engineering, cross-language service integration, distributed systems, and browser-side cryptography.

The project is designed for an individual developer storing API keys, environment variables, database connection details, login records, and secure notes.

> VaultForge is a portfolio and learning project, not an audited password manager. Use synthetic data only.

## Architecture

VaultForge separates account authentication from vault encryption.

### Current backend

```text
Client
  ↓
Go REST API
  ├── Account registration and login
  ├── Ed25519 access tokens
  ├── Opaque refresh-token rotation
  ├── Stateful bearer authorization
  ├── Session listing and revocation
  └── PostgreSQL persistence
```

### Planned platform

```mermaid
graph LR
    B["React and TypeScript Client"] --> A["Go REST API"]
    B --> W["Rust WASM Crypto"]
    W --> A
    A --> H["Rust gRPC Hashing Service"]
    A --> P["PostgreSQL"]
    A --> R["Redis"]
    A --> Q["RabbitMQ"]
    A --> O["OpenTelemetry"]
```

Redis, RabbitMQ, OpenTelemetry, Rust services, WebAssembly cryptography, the React client, and vault CRUD remain planned roadmap work.

### Account authentication

The Go API receives the account password during registration and login. Password operations pass through a replaceable `PasswordHasher` interface.

The current development implementation uses a local Argon2id adapter. A later Rust gRPC service can replace it without changing the HTTP contracts or authentication service.

Successful login creates a server-side session family and returns:

- A short-lived Ed25519-signed access token
- A single-use opaque refresh token
- Access-token and refresh-token expiration times

Refresh tokens are stored in PostgreSQL only as SHA-256 digests. Refresh rotation preserves the session family and detects replay.

Protected routes verify both the access-token signature and the active PostgreSQL session state. Revoking a session therefore invalidates already-issued access tokens immediately.

### Vault encryption

Vault encryption is a separate future browser-side workflow.

A Rust WebAssembly module will derive and manage vault encryption keys in the browser. The Go API must never receive the vault master passphrase, an unwrapped vault key, or decrypted vault contents.

## Current state

The current backend supports:

- Account registration
- Account login
- Argon2id password hashing with random salts
- Ed25519 access-token issuance and verification
- Opaque refresh tokens stored only as SHA-256 digests
- Atomic refresh-token rotation
- Refresh-token replay detection and family revocation
- Stateful bearer authentication backed by PostgreSQL
- Active-session listing
- Targeted owned-session revocation
- Current-session logout
- Logout-all
- Generic credential and token failure responses
- PostgreSQL persistence and migrations
- Database-backed readiness checks
- Strict JSON request handling and body limits
- Safe structured logging
- Real PostgreSQL and HTTP integration tests

Frontend, Redis, RabbitMQ, Rust services, WebAssembly cryptography, and vault CRUD are intentionally deferred until their roadmap phases.

## Technology

- **Backend:** Go, Chi, pgx, Zap
- **Database:** PostgreSQL
- **Authentication:** Argon2id through a replaceable hasher interface
- **Tokens:** Ed25519 JWT access tokens and opaque refresh tokens
- **Authorization:** Stateful bearer middleware with PostgreSQL session validation
- **Testing:** Go testing, race detector, real PostgreSQL integration tests
- **Quality:** gofmt, Vet, Staticcheck, Gitleaks
- **Planned:** Redis, RabbitMQ, OpenTelemetry, Rust gRPC, Rust WebAssembly, React, Docker, Kubernetes

## Repository structure

```text
vaultforge/
├── apps/
│   └── api/                 # Go HTTP API
├── deployments/
│   └── compose.yaml         # Local PostgreSQL
├── docs/
│   ├── architecture.md
│   ├── scope.md
│   └── threat-model.md
├── Makefile
├── README.md
├── SECURITY.md
└── .env.example
```

## Quick start

### Requirements

- Go
- Docker with Docker Compose
- Make
- Staticcheck
- golang-migrate
- direnv

### Configure the environment

```bash
cp .env.example .env
direnv allow
```

Generate a local Ed25519 seed:

```bash
openssl rand -base64 32
```

Place the generated value in:

```text
ACCESS_TOKEN_ED25519_SEED_BASE64
```

Only local synthetic values belong in `.env`. Never commit production credentials or signing keys.

### Start PostgreSQL and apply migrations

```bash
make db-setup
```

This prepares:

```text
vaultforge       development database
vaultforge_test  integration-test database
```

### Start the API

```bash
make dev
```

The API runs at:

```text
http://localhost:8080
```

## API routes

```text
GET    /health
GET    /health/live
GET    /health/ready

POST   /v1/auth/register
POST   /v1/auth/login
POST   /v1/auth/refresh

GET    /v1/sessions
DELETE /v1/sessions
DELETE /v1/sessions/current
DELETE /v1/sessions/{sessionID}
```

The session routes require:

```text
Authorization: Bearer <access-token>
```

See [`apps/api/README.md`](apps/api/README.md) for setup details, response contracts, security behavior, and Thunder Client examples.

## Testing and quality

Run all tests with the race detector:

```bash
make test
```

Run the complete local verification suite:

```bash
make verify
```

The integration suite rebuilds the dedicated test database from migration version zero and tests real PostgreSQL behavior.

The current integration coverage includes:

- Registration and login
- Session creation
- Access-token verification
- Refresh-token rotation
- Replay detection
- Stateful authorization
- Session listing
- Targeted revocation
- Current logout
- Logout-all
- Cross-user ownership isolation
- Immediate invalidation of revoked access tokens

GitHub Actions runs formatting checks, module verification, Vet, Staticcheck, race-enabled tests, PostgreSQL integration tests, and Gitleaks.

## Security boundary

VaultForge must never log or expose:

- Plaintext passwords
- Encoded password hashes
- Authorization headers
- Cookies
- Access or refresh tokens
- Refresh-token digests
- Database URLs
- Token-signing seeds or private keys
- Vault passphrases
- Encryption keys
- Decrypted vault data

Account password hashing, session authentication, and future vault encryption are separate security concerns.

During the current backend-only phase, refresh tokens are returned in JSON. Secure cookies and CSRF protection are deferred until frontend integration.

See:

- [`SECURITY.md`](SECURITY.md)
- [`docs/architecture.md`](docs/architecture.md)
- [`docs/threat-model.md`](docs/threat-model.md)
- [`docs/scope.md`](docs/scope.md)

## Disclaimer

VaultForge has not received an independent security audit and must not be used for real credentials or production secrets.
