# VaultForge Scope

## Product pitch

VaultForge is a backend-first developer secrets vault for storing API keys,
environment variables, database credentials, login records, and secure notes,
with browser-side encryption preventing backend services from reading vault
contents.

## Intended audience

VaultForge v1.0 is designed for one individual developer managing personal
development credentials.

It is not initially designed for teams, enterprise organizations, or shared
vaults.

## Core user stories

1. As a developer, I can register an account.
2. As a developer, I can log in and manage my active sessions.
3. As a developer, I can create a personal vault.
4. As a developer, I can create, edit, and delete encrypted vault items.
5. As a developer, I can lock and unlock my vault with a separate vault master
   passphrase.

## Supported item types

- API key
- Environment variable
- Database connection
- Login record
- Secure note

## Explicitly excluded from v1.0

- Team vaults
- Vault sharing
- Billing
- Browser extension
- Mobile application
- Passkeys
- Account recovery
- Import and export
- Kubernetes production deployment
- Claims suggesting the application has been independently audited

## Security boundary

Account authentication and vault encryption are separate systems.

The account password authenticates the user to VaultForge.

The vault master passphrase derives a browser-side key used to unlock encrypted
vault data.

The Go API may transiently receive an account password during registration and
login, but it must never receive:

- The vault master passphrase
- The key-encryption key
- The unwrapped vault data-encryption key
- Decrypted vault item contents

Only synthetic secrets may be used until client-side encryption is complete.

## Non-goals

VaultForge is a portfolio and learning project.

It is not an audited password manager and must not be represented as safe for
storing real credentials.
