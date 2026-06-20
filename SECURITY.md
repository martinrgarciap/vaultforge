# Security Policy

## Project status

VaultForge is an educational and portfolio project.

It is not an audited password manager or production secrets-management system.

Only synthetic data may be used until client-side vault encryption is complete and the implementation has received an independent security review.

## Supported data

Do not store real:

- Passwords
- API keys
- Private keys
- Database credentials
- Access or refresh tokens
- Recovery information
- Personal or business secrets

## Account authentication

Account authentication and vault encryption are separate systems.

The Go API receives the account password during registration and login and passes it through a replaceable `PasswordHasher` interface.

The current development adapter uses Argon2id with a fresh random salt for every stored password hash.

PostgreSQL stores only:

- The encoded password hash
- The password algorithm identifier

The application must never store or log a plaintext password.

The current login flow validates credentials but does not yet issue sessions or tokens.

## Vault encryption

Future vault encryption will occur in the browser through a Rust WebAssembly module.

The Go API must never receive:

- The vault master passphrase
- A key-encryption key
- An unwrapped vault data-encryption key
- Decrypted vault contents

The backend will persist encrypted payloads and non-secret cryptographic metadata only.

## Sensitive logging policy

Logs, traces, metrics, queues, errors, and screenshots must not contain:

- Request bodies
- Passwords
- Password hashes
- Authorization headers
- Cookies
- Access or refresh tokens
- Database URLs
- Vault values
- Vault passphrases
- Encryption keys
- Decrypted content

Authentication logs should contain only safe operational metadata such as method, path, status, duration, and request ID.

## Known limitations

- VaultForge has not received an independent security audit.
- Client-side vault encryption is not yet implemented.
- Sessions, access tokens, refresh tokens, and revocation are not yet implemented.
- Rate limiting and login lockouts are not yet implemented.
- Local development may use unencrypted HTTP.
- Account recovery is not implemented.
- A compromised browser could access decrypted data while a future vault is unlocked.
- Some non-secret metadata may remain visible to backend services.
- The temporary Go Argon2id adapter will later be replaced by a Rust gRPC service.

See [`docs/threat-model.md`](docs/threat-model.md) for the complete threat model, accepted risks, and trust boundaries.

## Reporting a vulnerability

Do not place passwords, tokens, private keys, real credentials, database URLs, or decrypted vault contents in a public issue.

Report suspected security problems privately to the repository owner.

A private security-reporting contact method must be added before the repository is promoted broadly.
