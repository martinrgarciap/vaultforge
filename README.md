# VaultForge

VaultForge is a backend-first developer secrets vault demonstrating Go backend
engineering, Rust service integration, browser-side cryptography, and secure
distributed-system design.

## Status

VaultForge is currently in the planning and engineering-foundation phase.

No real credentials may be stored in this application.

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

The browser will encrypt and decrypt vault contents through a Rust WebAssembly
module.

The Go API may transiently receive the account password during authentication,
but it must never receive:

- The vault master passphrase
- The key-encryption key
- The unwrapped vault data-encryption key
- Decrypted vault contents

## Repository structure

```text
vaultforge/
├── apps/
│   ├── api/
│   └── web/
├── services/
│   └── hash-service/
├── packages/
│   └── proto/
├── deployments/
├── docs/
├── scripts/
├── CLAUDE.md
├── Makefile
└── README.md
```

## Current milestone

- [x] Product scope
- [x] Initial threat model
- [x] Target architecture
- [ ] Repository and CI baseline
- [ ] Go API foundation
- [ ] PostgreSQL schema
- [ ] Authentication
- [ ] Client-side encryption

## Local requirements

Install:

- Go
- Node.js
- npm
- Make
- Staticcheck
- Git

Install Staticcheck:

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
```

## Initial setup

```bash
make setup
```

## Quality checks

```bash
make format-check
make lint
make test
```

## Development

The browser development server can be started with:

```bash
make dev
```

The production-style API server begins in Step 2.

## Documentation

- docs/scope.md
- docs/threat-model.md
- docs/architecture.md
- SECURITY.md

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

VaultForge is a portfolio and learning project.
