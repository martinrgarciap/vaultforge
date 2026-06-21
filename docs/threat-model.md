# VaultForge Threat Model

## 1. Security objectives

VaultForge is intended to:

- Prevent backend services from reading decrypted vault contents after the browser-side encryption phase is complete.
- Store account passwords using a slow password-hashing algorithm.
- Store refresh tokens only in hashed form at rest.
- Enforce server-side authorization for every protected resource.
- Revoke active sessions and invalidate their existing access tokens.
- Detect reuse of rotated refresh tokens.
- Detect modification of encrypted vault data.
- Prevent secrets from leaking into logs, traces, metrics, queues, URLs, analytics, screenshots, or browser persistence.
- Fail safely when required security dependencies are unavailable.

VaultForge is not intended to protect a user whose browser or device is fully compromised while the vault is unlocked.

## 2. Protected assets

- Account passwords
- Account password hashes
- Access tokens
- Refresh tokens
- Refresh-token digests
- Access-token signing seeds and private keys
- Vault master passphrase
- Key-encryption key, also called the KEK
- Vault data-encryption key, also called the DEK
- Decrypted vault items
- Encrypted vault items
- User identity data
- Vault metadata
- Session information
- Cryptographic version and parameter metadata

## 3. Potential attackers

- An unauthenticated external attacker
- A malicious authenticated user
- An attacker with a stolen database backup
- An attacker with a stolen access token
- An attacker with a stolen refresh token
- An attacker replaying a previously rotated refresh token
- An attacker exploiting an authorization flaw
- An attacker exploiting cross-site scripting
- A compromised frontend dependency
- A developer accidentally logging sensitive data
- An attacker able to modify stored ciphertext
- A compromised infrastructure operator
- An attacker with access to a CI environment
- An attacker attempting account enumeration or password brute force

## 4. Trust boundaries

### Browser or API client to Go API

The client sends authentication requests and, after the frontend phase, encrypted vault envelopes to the Go API.

During the current backend-only phase, access and refresh tokens are returned in JSON responses and may be submitted by API clients.

After client-side encryption is implemented, plaintext vault contents must never cross this boundary.

### Go API to Rust hashing service

The Go API will send account passwords to the Rust service only during account registration, login, or password changes.

The hashing service must not receive vault passphrases, encryption keys, vault items, session tokens, or unrelated user data.

### Go API to PostgreSQL

The Go API stores account records, password hashes, refresh-token digests, session metadata, vault metadata, and encrypted vault envelopes.

PostgreSQL must never store plaintext account passwords, raw refresh tokens, access tokens, vault passphrases, unwrapped keys, or decrypted vault items.

### Go API to Redis

Redis will store rate-limit counters, lockout state, and other short-lived coordination information.

Redis must never store passwords, raw tokens, encryption keys, or vault contents.

### Application to logs and telemetry

Only explicitly approved operational metadata may enter logs or telemetry.

Request bodies, authorization headers, cookies, passwords, hashes, tokens, token digests, signing seeds, vault payloads, and encryption keys must never be recorded.

## 5. Component visibility

| Component             | Allowed to see                                                                                                                                                                                    | Must never see or retain                                                                                                                                                  |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Browser or API client | Account password while entered, access token, refresh token, and later vault keys and decrypted items while unlocked                                                                              | Keys or decrypted content in URLs, analytics, persistent logs, or error reports                                                                                           |
| Go API                | Account password and raw refresh token transiently during authentication, authenticated IDs, vault names, item types, and synthetic dummy item payloads during the current backend phase          | Vault master passphrase, KEK, unwrapped DEK, real vault secrets, future decrypted item contents, raw refresh tokens in persistent storage, or tokens and payloads in logs |
| Rust hashing service  | Account password transiently and PHC password hash while processing                                                                                                                               | Vault passphrase, vault key, vault items, session tokens, or persistent password storage                                                                                  |
| PostgreSQL            | Email, password hash, password algorithm, refresh-token digest, session metadata, vault metadata, synthetic dummy payloads during the current phase, item versions, and sanitized outbox metadata | Plaintext account password, raw refresh token, access token, vault passphrase, unwrapped key, real vault secrets, or future decrypted vault items                         |
| Redis                 | Rate-limit identifiers, failed-login counters, and short-lived lockout state                                                                                                                      | Passwords, password hashes, raw tokens, vault secrets, encryption keys, or decrypted data                                                                                 |
| Logs and telemetry    | Request ID, trace ID, method, route template, status, duration, sanitized user ID, and dependency status                                                                                          | Request bodies, Authorization headers, cookies, passwords, hashes, tokens, token digests, signing seeds, vault payloads, keys, decrypted data                             |
| CI and GitHub         | Source code, public test fixtures, and placeholder configuration                                                                                                                                  | Credentials, signing keys, access tokens, real environment files, or real vault data                                                                                      |

## 6. Threats and mitigations

| Threat                           | Example                                              | Current or planned mitigation                                                                                    |
| -------------------------------- | ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Account enumeration              | Login response differs for an unknown email          | Current: same generic invalid-credentials response for unknown users, incorrect passwords, and disabled accounts |
| Password brute force             | Repeated automated login attempts                    | Current: Argon2id password hashing. Planned: rate limiting and short-lived lockouts                              |
| Database theft                   | Attacker obtains a database dump                     | Current: Argon2id password hashes and refresh-token digests. Planned: ciphertext-only vault items                |
| Session fixation or confusion    | Two logins unexpectedly share one session identity   | Current: each login creates a new random token family                                                            |
| Stolen access token              | Attacker uses a valid token after logout             | Current: protected requests verify active PostgreSQL session state                                               |
| Refresh-token theft              | Stolen refresh token is submitted                    | Current: opaque random tokens, digest-only storage, rotation, bounded lifetime, and family revocation            |
| Refresh-token replay             | An old refresh token is reused after rotation        | Current: replay detection and token-family revocation                                                            |
| Cross-user session revocation    | User guesses another user's session ID               | Current: revoke queries require both authenticated user ID and token-family ID                                   |
| Insecure direct object reference | User guesses another user's vault or item ID         | Current: authenticated owner ID is supplied by middleware and every vault and item query is owner-scoped         |
| Lost update                      | Two clients update the same item concurrently        | Current: strong ETags and required If-Match versions reject stale updates                                        |
| Duplicate create replay          | A client retries an item creation request            | Current: idempotency keys replay the same request and reject conflicting reuse                                   |
| Audit-event secret leakage       | A payload or key is copied into an outbox event      | Current: versioned allow-listed metadata and regression tests reject secret-bearing audit fields                 |
| Secret leakage                   | Password or token enters a log                       | Current: allow-listed structured logging and automated no-secret logging tests                                   |
| Ciphertext tampering             | Stored encrypted data is modified                    | Planned: AES-GCM authenticated encryption and authentication-tag verification                                    |
| Nonce reuse                      | The same AES-GCM nonce is reused with one key        | Planned: secure random nonce generation and automated tests                                                      |
| Cross-site scripting             | Injected JavaScript reads an unlocked vault          | Planned: restrictive CSP, dependency review, safe rendering, and short in-memory key lifetime                    |
| Hashing-service outage           | The Rust service is unavailable during login         | Current service boundary fails closed. Planned Rust adapter must preserve safe dependency errors                 |
| Supply-chain compromise          | A malicious frontend dependency reads decrypted data | Planned: minimize dependencies, use lockfiles, scan dependencies, and review sensitive dependency changes        |
| Token leakage                    | Raw refresh token is stored in PostgreSQL            | Current: store only SHA-256 refresh-token digests                                                                |
| Excessive data exposure          | API returns password hash or token digest fields     | Current: explicit response types containing only safe public fields                                              |
| Error leakage                    | Database details appear in an API response           | Current: map internal dependency errors to generic public errors                                                 |
| Oversized input                  | Attacker submits a very large request body           | Current: enforce request-body and field-length limits                                                            |
| CI secret exposure               | A real key is committed or printed in CI             | Current: Gitleaks scanning, placeholder configuration, restricted permissions, and synthetic fixtures            |

## 7. Accepted risks

- VaultForge has not received an independent security audit.
- Only synthetic secrets are permitted during development.
- Client-side vault encryption is not implemented yet.
- Current synthetic item payloads are visible to the Go API and PostgreSQL.
- During the backend-only phase, refresh tokens are returned in JSON rather than secure cookies.
- CSRF protection for cookie-authenticated refresh requests is not implemented yet.
- Production signing-key storage, rotation, and multi-key verification are not implemented yet.
- Local development may use HTTP between local services.
- Account recovery is not implemented.
- Multi-factor authentication is not implemented.
- A compromised browser while the vault is unlocked can access decrypted data.
- Some non-secret metadata, including timestamps and item types, may remain visible to the backend.
- The browser must temporarily hold decrypted values and encryption keys in memory while the vault is unlocked.
- Denial-of-service protection is limited before Redis rate limiting is implemented.
- Cryptographic parameter choices will require review before any production claim could be made.

## 8. Security invariants

These rules must remain true throughout development:

1. Account authentication and vault encryption remain separate.
2. The backend never receives the vault master passphrase.
3. The backend never receives the unwrapped vault data-encryption key.
4. Real or decrypted vault secrets never enter server-side storage; current item payloads contain synthetic test data only.
5. Authentication never falls back to a weaker method when the hasher is unavailable.
6. Raw refresh tokens are not stored in PostgreSQL.
7. Access tokens are not stored in PostgreSQL.
8. Request bodies, authorization headers, tokens, and token digests are not logged.
9. Real secrets are not used before client-side encryption is complete.
10. Every encrypted payload includes a supported version.
11. Authorization is enforced on the server for every protected object.
12. Revoked session families cannot authorize protected requests.
13. Unknown and unowned session IDs are indistinguishable through public errors.

## 9. Open decisions

- TODO: Decide whether item type remains visible to the server after encryption.
- TODO: Decide whether vault names remain visible to the server.
- TODO: Decide whether created and updated timestamps are considered acceptable metadata leakage.
- TODO: Select final Argon2id parameters after benchmarking.
- TODO: Select production signing-key storage and rotation strategy.
- TODO: Define secure cookie and CSRF behavior for frontend refresh flows.
- TODO: Decide the inactivity-lock duration for the browser vault.
