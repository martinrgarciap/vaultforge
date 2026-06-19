# VaultForge

VaultForge is a backend-first developer secrets vault demonstrating secure Go
backend engineering, PostgreSQL persistence, browser-side cryptography, and
distributed-system design.

VaultForge is a portfolio and learning project. Only synthetic sample data
should be used.

## Current status

The backend and PostgreSQL foundation are complete.

Implemented so far:

- Go HTTP API using Chi
- Environment-based configuration
- Structured Zap logging
- Request IDs
- Safe request logging
- Panic recovery
- Security headers
- Request and server timeouts
- Graceful shutdown
- Standard JSON responses
- JSON `404 Not Found` and `405 Method Not Allowed` responses
- PostgreSQL through `pgxpool`
- Versioned SQL migrations
- Database liveness and readiness checks
- Initial user persistence store
- PostgreSQL repository integration tests
- Schema constraint and lifecycle integration tests
- GitHub Actions PostgreSQL integration testing
- Vet, Staticcheck, race detection, formatting, and Gitleaks checks

Authentication is the next major backend phase.

Frontend and Rust service work are intentionally deferred until the backend
foundation is ready for them.

## Product direction

VaultForge is designed for one individual developer storing:

- API keys
- Environment variables
- Database connection details
- Login records
- Secure notes

## Security model

Account authentication and vault encryption are separate concerns.

The server authenticates the account.

The browser will eventually encrypt and decrypt vault contents through a Rust
WebAssembly module.

The Go API may transiently receive the account password during authentication,
but it must never receive:

- The vault master passphrase
- The key-encryption key
- The unwrapped vault data-encryption key
- Decrypted vault contents

Vault item payloads are stored only as opaque ciphertext and associated
cryptographic metadata.

## Repository structure

```text
vaultforge/
├── apps/
│   └── api/
│       ├── cmd/api/
│       ├── internal/
│       │   ├── api/
│       │   ├── db/
│       │   └── store/
│       ├── migrations/
│       ├── go.mod
│       └── go.sum
├── deployments/
│   └── compose.yaml
├── docs/
├── Makefile
├── README.md
├── SECURITY.md
└── .env.example
```

## Milestones

- [x] Product scope and security boundaries
- [x] Repository and CI baseline
- [x] Go API foundation
- [x] PostgreSQL schema and migrations
- [x] PostgreSQL integration testing
- [ ] Account authentication
- [ ] Session and refresh-token lifecycle
- [ ] Vault metadata APIs
- [ ] Client-side encryption
- [ ] Encrypted vault item APIs
- [ ] Rust password-hashing service
- [ ] Messaging, auditing, and observability

## Local requirements

Install:

- Go
- Docker with Docker Compose
- Make
- Git
- Staticcheck
- golang-migrate
- direnv

Install Staticcheck:

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
```

On macOS, install the migration CLI and direnv with Homebrew:

```bash
brew install golang-migrate
brew install direnv
```

Configure the direnv shell hook using the instructions printed by Homebrew.

## Environment setup

Copy the example environment file:

```bash
cp .env.example .env
```

Allow direnv to load the local environment:

```bash
direnv allow
```

Confirm the required variables are available without printing their values:

```bash
test -n "$DATABASE_URL" && echo "DATABASE_URL is set"
test -n "$TEST_DATABASE_URL" && echo "TEST_DATABASE_URL is set"
```

The `.env` file is ignored by Git.

Do not commit real credentials or production database URLs.

## Initial setup

Download the Go dependencies:

```bash
make setup
```

Start PostgreSQL, create the dedicated test database, and apply the development
migrations:

```bash
make db-setup
```

This creates two local databases:

```text
vaultforge       development database
vaultforge_test  integration-test database
```

## Running the API

Start PostgreSQL if it is not already running:

```bash
make compose-up
```

Start the API:

```bash
make dev
```

The default address is:

```text
http://localhost:8080
```

## Health endpoints

The API exposes:

```text
GET /health
GET /health/live
GET /health/ready
```

`/health` and `/health/live` report whether the HTTP process is responsive.

`/health/ready` also verifies that PostgreSQL is available. It returns
`503 Service Unavailable` when the database cannot be reached.

Example:

```bash
curl -i http://localhost:8080/health/live
curl -i http://localhost:8080/health/ready
```

## PostgreSQL commands

Start PostgreSQL:

```bash
make compose-up
```

View container status:

```bash
make compose-ps
```

Follow PostgreSQL logs:

```bash
make compose-logs
```

Open a PostgreSQL shell connected to the development database:

```bash
make db-shell
```

Stop PostgreSQL while preserving its volume:

```bash
make compose-stop
```

Stop the Compose project:

```bash
make compose-down
```

## Migration commands

Apply all pending migrations:

```bash
make migrate-up
```

Roll back one migration:

```bash
make migrate-down
```

Show the current migration version:

```bash
make migrate-version
```

Create a new sequential migration:

```bash
make migrate-create name=create_example
```

The command creates matching `.up.sql` and `.down.sql` files under
`apps/api/migrations`.

## Testing

Run the complete test suite with the race detector:

```bash
make test
```

The integration tests use `TEST_DATABASE_URL`.

Before the store tests run, they:

1. Roll the dedicated test database back to migration version zero.
2. Apply every migration from the beginning.
3. Open a real PostgreSQL connection pool.
4. Run repository and schema integration tests.
5. Roll the test database back to version zero after completion.

The normal development database is not modified by integration tests.

## Quality checks

Check formatting:

```bash
make format-check
```

Apply formatting:

```bash
make format
```

Run Vet and Staticcheck:

```bash
make lint
```

Verify downloaded Go modules:

```bash
make mod-verify
```

Run every local quality check:

```bash
make verify
```

## Continuous integration

GitHub Actions runs:

- Go formatting checks
- Module verification
- `go vet`
- Staticcheck
- The full test suite with the race detector
- PostgreSQL repository and schema integration tests
- Gitleaks secret scanning

The CI Go job starts a dedicated PostgreSQL service and supplies a synthetic
test database URL.

## Documentation

Project documentation includes:

- `docs/scope.md`
- `docs/threat-model.md`
- `docs/architecture.md`
- `SECURITY.md`

## Data policy

Use synthetic sample data only.

Do not enter real:

- Passwords
- API keys
- Private keys
- Database credentials
- Access tokens
- Refresh tokens
- Personal secrets

## Disclaimer

VaultForge is a portfolio and learning project. It should not currently be used
as a production password manager or secrets-management system.
