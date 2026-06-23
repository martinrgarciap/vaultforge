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

- A short-lived Ed25519-signed access token in JSON
- Access-token and refresh-token expiration times in JSON
- A single-use opaque refresh token in a host-only `HttpOnly` cookie
- A readable CSRF cookie for double-submit validation

Access tokens:

- Include the user ID and token-family ID
- Are validated for algorithm, key ID, issuer, audience, issued-at, not-before, and expiration
- Are not stored in PostgreSQL
- Are checked against active PostgreSQL session state on protected requests

Refresh tokens:

- Are generated using cryptographically secure randomness
- Are never returned in JSON
- Are delivered through host-only `HttpOnly`, `SameSite=Strict` cookies scoped to `/v1/auth/refresh`
- Use the `Secure` cookie flag in production
- Are stored in PostgreSQL only as SHA-256 digests
- Are rotated after every successful use
- Preserve the session family and absolute refresh expiration
- Trigger family-wide revocation when replay is detected

Refresh requests:

- Must not contain a request body
- Require the refresh cookie
- Require exactly one readable CSRF cookie and one `X-CSRF-Token` header
- Require the CSRF cookie and header to match
- Rotate both the refresh and CSRF cookies after success
- Clear stale browser cookies after invalid refresh credentials
- Use `Cache-Control: no-store` on login and refresh responses

Revoking a session immediately prevents its existing access and refresh tokens from being used.

Users may:

- List their active session families
- Revoke one owned session
- Logout the current session
- Logout all of their sessions

Unknown, already-revoked, and other users’ session identifiers return the same public not-found response.

Logout of the current session, revocation of the current session by ID, and logout of all sessions clear both browser session cookies. Revoking a different session does not clear the current browser cookies.

## Signing and HMAC key handling

The local API requires an Ed25519 seed through:

```text
ACCESS_TOKEN_ED25519_SEED_BASE64
```

Redis identity protection requires a separate HMAC key through:

```text
RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64
```

Generate each value independently with cryptographically secure randomness. Do not reuse the signing seed as the rate-limit HMAC key.

Signing seeds, private keys, and rate-limit HMAC keys must never be:

- Committed to Git
- Added to test fixtures
- Logged
- Included in screenshots
- Shared in issues or pull requests
- Embedded in diagnostics or metrics
- Reused as production secrets

Production key storage, signing-key rotation, HMAC-key rotation, and multi-key verification are not implemented yet.

## Redis-backed security controls

Redis is a required runtime dependency for distributed abuse protection. It stores operational security state only.

Current Redis responsibilities:

- Fixed-window request counters for registration, login, refresh, and authenticated mutations
- Failed-login counters
- Temporary login-lockout state

Identity material is transformed with HMAC-SHA-256 before being included in Redis keys. Length-prefixed identity parts prevent ambiguous concatenation.

Redis must never store:

- Plaintext passwords
- Password hashes
- Authorization headers
- Access or refresh tokens
- Refresh-token digests
- CSRF token values
- Signing seeds or HMAC keys
- Database or Redis URLs
- Vault passphrases
- Encryption keys
- Item payloads or vault values
- Raw request bodies
- Raw dependency errors
- Raw email addresses, IP addresses, or authenticated user IDs as key material

All rate-limit, failed-login, and lockout records have bounded expiration. Redis persistence is disabled in local Compose because these records are temporary operational state, not durable application data.

The default policies are:

- Registration: 5 requests per 10 minutes per direct peer IP
- Login: 20 requests per minute per direct peer IP
- Refresh: 30 requests per minute per direct peer IP
- Authenticated mutations: 60 requests per minute per authenticated user ID
- Login protection: 5 invalid-credential failures within 15 minutes triggers a 15-minute lockout for normalized email plus direct peer IP

The application uses the direct TCP peer address. It ignores `X-Forwarded-For`, `X-Real-IP`, and similar forwarding headers until a trusted-proxy configuration is implemented.

Successful login clears failed-login and lockout state on a best-effort basis after the session has been created. Durable item-creation idempotency remains in PostgreSQL and is not duplicated in Redis.

## Dependency failure behavior

Failure behavior is defined and tested:

- Application startup fails if PostgreSQL or Redis cannot be reached.
- Liveness does not query dependencies.
- Readiness checks both PostgreSQL and Redis with a short deadline.
- PostgreSQL failure returns sanitized `503 Service Unavailable` responses for data-dependent requests.
- Redis failure blocks registration, login, refresh, and authenticated mutations with sanitized `503` responses.
- Read-only vault and item routes do not call Redis during request handling.
- Registration and login fail closed when the password hasher is unavailable.
- Context cancellation and deadlines propagate through handlers, services, repositories, and dependency calls.
- PostgreSQL and Redis connection details and raw driver errors are never returned to clients.

Controlled outage tests verify failure and recovery behavior for both PostgreSQL and Redis.

## Current synthetic vault API

The current backend implements owner-scoped vault and item workflows using synthetic dummy payloads.

The current item API:

- Accepts only documented item types.
- Derives the owner from the authenticated bearer principal.
- Prevents cross-user access through owner-scoped service and store queries.
- Requires an idempotency key when creating an item.
- Requires a strong quoted `If-Match` version for updates and lifecycle mutations.
- Rejects stale versions instead of silently overwriting newer data.
- Supports soft deletion, restoration, and permanent deletion.
- Writes allow-listed sanitized audit metadata through the same database transaction as each mutation.
- Includes regression tests that reject payload, key, hash, name, and secret leakage into outbox metadata.

These payloads are not encrypted yet and are visible to the Go API and PostgreSQL. Only synthetic values are permitted.

## Vault encryption

Future vault encryption will occur in the browser through a Rust WebAssembly module.

The Go API must never receive:

- The vault master passphrase
- A key-encryption key
- An unwrapped vault data-encryption key
- Decrypted vault contents

The backend will persist encrypted payloads and non-secret cryptographic metadata only.

## Sensitive logging and metrics policy

Logs, traces, metrics, queues, errors, diagnostics, screenshots, and Redis records must not contain:

- Request bodies
- Passwords
- Password hashes
- Authorization headers
- Cookies
- Access or refresh tokens
- Refresh-token digests
- CSRF token values
- Database or Redis URLs
- Token-signing seeds or private keys
- Rate-limit identity HMAC keys
- Raw rate-limit identities
- Vault values
- Vault passphrases
- Encryption keys
- Decrypted content
- Raw dependency errors

Authentication logs contain only safe operational metadata such as method, path, status, duration, and bounded request ID.

The metrics registry records only normalized HTTP method, Chi route pattern, status class, count, duration, in-flight requests, uptime, and sanitized build metadata. It never records raw URLs, query values, resource IDs, user IDs, email addresses, tokens, cookies, headers, or request bodies.

`/internal/metrics` is available only to the direct loopback peer. Forwarded-IP headers are ignored. Rejected external requests receive the same public not-found response used for unavailable internal routes.

`/health/diagnostics` returns only the service name, sanitized build version, and sanitized commit identifier. Diagnostics and metrics responses use `Cache-Control: no-store`.

## Implemented safeguards

The current backend includes:

- Argon2id password hashing
- Generic invalid-credential responses
- Ed25519 access-token verification
- Strict token algorithm and key-ID validation
- Stateful session validation on protected routes
- Opaque refresh-token rotation
- Replay detection and token-family revocation
- Host-only `HttpOnly`, `SameSite=Strict` refresh cookies
- Double-submit CSRF protection for bodyless refresh requests
- Refresh and CSRF cookie rotation and logout clearing
- Ownership checks for session, vault, and item operations
- Strict JSON decoding and request-body limits
- Aggregate request-header limits
- Bounded authorization tokens, request IDs, route identifiers, queries, cursors, pagination, and `If-Match` values
- Idempotency-key protection for item creation
- Strong `ETag` and `If-Match` optimistic concurrency
- Soft-delete, restore, and permanent-delete state enforcement
- Sanitized transactional outbox writes
- Automated checks preventing sensitive values from entering outbox payloads
- Redis-backed distributed limits for authentication and mutations
- HMAC-protected Redis identities
- Failed-login tracking and temporary lockouts
- Required PostgreSQL and Redis startup checks
- Composite readiness with a short deadline
- Configurable HTTP and Redis timeouts
- Context cancellation through handlers, services, and repositories
- Safe PostgreSQL and Redis failure mapping
- Fail-closed password-hasher behavior
- Sanitized build diagnostics
- Loopback-only low-cardinality metrics
- Safe structured request logging
- Panic recovery with generic public errors
- PostgreSQL and Redis integration tests
- Controlled dependency-outage and recovery tests
- Race-enabled test execution
- Static analysis and secret scanning in CI

## Known limitations

- VaultForge has not received an independent security audit.
- Client-side vault encryption is not yet implemented.
- Current synthetic item payloads are visible to the Go API and PostgreSQL.
- Production signing-key and HMAC-key storage and rotation are not yet implemented.
- Local development may use unencrypted HTTP and therefore omits the cookie `Secure` flag; production enables it.
- Trusted reverse-proxy handling is not implemented; forwarded client-IP headers are intentionally ignored.
- The internal metrics boundary currently relies on the direct peer being loopback. Production network policy and a dedicated internal listener are not implemented yet.
- A successful cross-site scripting attack could access the in-memory access token and readable CSRF token, although not the `HttpOnly` refresh token.
- Account recovery is not implemented.
- Multi-factor authentication is not implemented.
- A compromised browser could access decrypted data while a future vault is unlocked.
- Some non-secret metadata may remain visible to backend services.
- The temporary Go Argon2id adapter will later be replaced by a Rust gRPC service.
- RabbitMQ publication, OpenTelemetry tracing, production deployment, and browser-side encryption remain future roadmap work.

See [`docs/threat-model.md`](docs/threat-model.md) for the complete threat model, accepted risks, and trust boundaries.

## Reporting a vulnerability

Do not place passwords, tokens, private keys, signing seeds, real credentials, database URLs, or decrypted vault contents in a public issue.

Report suspected security problems privately to the repository owner.

A private security-reporting contact method must be added before the repository is promoted broadly.
