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

Unknown email addresses, incorrect passwords, invalid login input, and disabled accounts return the same generic public credentials error.

## Sessions and tokens

Successful login creates a server-side session family and returns:

- A short-lived Ed25519-signed access token
- A single-use opaque refresh token
- Access-token and refresh-token expiration times

Access tokens:

- Include the user ID and token-family ID
- Are validated for algorithm, key ID, issuer, audience, issued-at, not-before, and expiration
- Are not stored in PostgreSQL
- Are checked against active PostgreSQL session state on protected requests

Refresh tokens:

- Are generated using cryptographically secure randomness
- Are stored in PostgreSQL only as SHA-256 digests
- Are rotated after every successful use
- Preserve the session family and absolute refresh expiration
- Trigger family-wide revocation when replay is detected

Revoking a session immediately prevents its existing access and refresh tokens from being used.

Users may:

- List their active session families
- Revoke one owned session
- Logout the current session
- Logout all of their sessions

Unknown, already-revoked, and other users’ session identifiers return the same public not-found response.

During the current backend-only phase, refresh tokens are returned in JSON responses. Secure cookies and CSRF protection are deferred until frontend integration.

## Signing-key handling

The local API requires an Ed25519 seed through:

```text
ACCESS_TOKEN_ED25519_SEED_BASE64
```

Signing seeds and private keys must never be:

- Committed to Git
- Added to test fixtures
- Logged
- Included in screenshots
- Shared in issues or pull requests
- Reused as production secrets

Production key storage, rotation, and multi-key verification are not implemented yet.

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
- Refresh-token digests
- Database URLs
- Token-signing seeds or private keys
- Vault values
- Vault passphrases
- Encryption keys
- Decrypted content

Authentication logs should contain only safe operational metadata such as method, path, status, duration, and request ID.

## Implemented safeguards

The current backend includes:

- Argon2id password hashing
- Generic invalid-credential responses
- Ed25519 access-token verification
- Strict token algorithm and key-ID validation
- Stateful session validation on protected routes
- Opaque refresh-token rotation
- Replay detection and token-family revocation
- Ownership checks for session revocation
- Strict JSON decoding and body-size limits
- Safe structured request logging
- Panic recovery with generic public errors
- PostgreSQL integration tests
- Race-enabled test execution
- Static analysis and secret scanning in CI

## Known limitations

- VaultForge has not received an independent security audit.
- Client-side vault encryption is not yet implemented.
- Rate limiting and login lockouts are not yet implemented.
- Secure refresh-token cookies and CSRF protection are not yet implemented.
- Production signing-key storage and rotation are not yet implemented.
- Local development may use unencrypted HTTP.
- Account recovery is not implemented.
- Multi-factor authentication is not implemented.
- A compromised browser could access decrypted data while a future vault is unlocked.
- Some non-secret metadata may remain visible to backend services.
- The temporary Go Argon2id adapter will later be replaced by a Rust gRPC service.

See [`docs/threat-model.md`](docs/threat-model.md) for the complete threat model, accepted risks, and trust boundaries.

## Reporting a vulnerability

Do not place passwords, tokens, private keys, signing seeds, real credentials, database URLs, or decrypted vault contents in a public issue.

Report suspected security problems privately to the repository owner.

A private security-reporting contact method must be added before the repository is promoted broadly.
