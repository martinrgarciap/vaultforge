# VaultForge Threat Model

## 1. Security objectives

VaultForge is intended to:

- Prevent backend services from reading decrypted vault item contents during normal encrypted item workflows.
- Store account passwords using a slow password-hashing algorithm.
- Store refresh tokens only in hashed form at rest.
- Enforce server-side authorization for every protected resource.
- Revoke active sessions and invalidate their existing access tokens.
- Detect reuse of rotated refresh tokens.
- Prevent cross-site request forgery against cookie-authenticated refresh requests.
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
- CSRF tokens
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

The client sends authentication requests and encrypted vault item envelopes to the Go API. Vault item plaintext, vault passphrases, key-encryption keys, and unwrapped vault data keys stay in the browser during normal encrypted item workflows.

Access tokens are returned in login and refresh JSON responses for in-memory use. They must not be placed in browser persistence, URLs, logs, metrics, or error reports.

Refresh tokens are delivered through host-only `HttpOnly`, `SameSite=Strict` cookies and are never returned in JSON. Refresh requests also require a readable CSRF cookie and matching `X-CSRF-Token` header.

The API bounds aggregate headers, bearer tokens, JSON bodies, queries, cursors, pagination, optimistic-concurrency headers, request IDs, and route identifiers before using them.

The API derives the current client identity from the direct TCP peer. It ignores forwarding headers until a trusted-proxy configuration is implemented.

The browser-side Rust WASM crypto boundary encrypts and decrypts item values for
normal item workflows. Vault passphrases, KEKs, unwrapped vault keys, and
plaintext vault contents must never cross this boundary.

### Go API to Rust hash-service

The Go API sends account passwords to the Rust hash-service only during account registration, login, or password changes.

The hash-service must not receive vault passphrases, encryption keys, vault items, session tokens, Redis identities, or unrelated user data.

Authentication must fail closed if the hash-service is unavailable. It must never fall back to weaker password handling.

### Go API to Rust password-service

The Go API sends account-password generator and strength-check requests to the Rust password-service.

The password-service must not receive vault passphrases, encryption keys, vault items, session tokens, Redis identities, or unrelated user data.

Password-tool requests must fail with safe public errors if the password-service is unavailable.

### Go API to PostgreSQL

The Go API receives raw refresh-token cookie values only transiently during login and refresh processing. It stores account records, password hashes, refresh-token digests, session metadata, vault metadata, encrypted item payload envelopes, durable item-idempotency records, and sanitized transactional outbox records.

PostgreSQL must never store plaintext account passwords, raw refresh tokens, access tokens, vault passphrases, unwrapped keys, or real vault secrets.

PostgreSQL failures are mapped to safe public errors. Connection strings and raw driver errors must not cross the API boundary.

### Go API to Redis

Redis stores only bounded, expiring operational security state:

- Fixed-window request counters
- Failed-login counters
- Temporary login-lockout state

Identity material is transformed with HMAC-SHA-256 over length-prefixed parts before becoming Redis key material.

Redis must never store passwords, password hashes, authorization headers, cookies, raw tokens, token digests, CSRF values, signing seeds, HMAC keys, database URLs, vault payloads, vault passphrases, encryption keys, decrypted data, raw request bodies, raw dependency errors, or raw email, IP, and user identities as key material.

Redis is required for registration, login, refresh, and authenticated mutations. Redis failures return safe temporary dependency errors rather than silently disabling abuse protection.

### Application to logs, diagnostics, and metrics

Only explicitly approved operational metadata may enter logs, diagnostics, or metrics.

Request bodies, raw URLs containing resource IDs, query values, authorization headers, cookies, passwords, hashes, tokens, token digests, signing seeds, HMAC keys, vault payloads, and encryption keys must never be recorded.

Diagnostics expose only sanitized service, version, and commit values.

Metrics use normalized route patterns, bounded HTTP methods, status classes, counts, durations, in-flight requests, uptime, and sanitized build metadata. The metrics endpoint accepts only direct loopback peers and ignores forwarded-IP headers.

### Runtime process to required dependencies

Application startup requires PostgreSQL, Redis, hash-service, and password-service to be reachable within bounded startup deadlines.

Liveness remains independent of required dependencies. Readiness checks each dependency with a short deadline.

PostgreSQL, Redis, hash-service, or password-service failure must not crash the process, leak connection details, or leave a security-sensitive operation partially enforced. Existing clients may recover when the dependency returns.

## 5. Component visibility

| Component             | Allowed to see                                                                                                                                                                                                                         | Must never see or retain                                                                                                                                                                         |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Browser or API client | Account password while entered, in-memory access token, readable CSRF token, server-managed refresh cookie, vault passphrase during unlock/setup, KEK in memory, unwrapped vault key in memory, decrypted item payloads while unlocked | Real production credentials in this portfolio project, access tokens in browser persistence, JavaScript access to the `HttpOnly` refresh token, or keys and decrypted content in persistent logs |
| Go API                | Account password and raw refresh token transiently during authentication, authenticated IDs, vault names, vault IDs, item IDs, item types, versions, timestamps, salts, wrapped vault keys, and encrypted item payload envelopes       | Vault passphrase, KEK, unwrapped vault data key, decrypted item payloads, raw refresh tokens in persistent storage, or tokens and payloads in logs or metrics                                    |
| Rust hash-service     | Account password transiently and PHC password hash while processing                                                                                                                                                                    | Vault passphrase, vault key, vault items, session tokens, Redis state, or persistent password storage                                                                                            |
| Rust password-service | Generated account passwords and account passwords submitted for strength checks transiently while processing                                                                                                                           | Vault passphrase, vault key, vault items, session tokens, Redis state, or persistent password storage                                                                                            |
| PostgreSQL            | Email, password hash, password algorithm, refresh-token digest, session metadata, vault metadata, salts, wrapped vault keys, ciphertext, nonces, item versions, idempotency records, and sanitized outbox metadata                     | Plaintext account password, raw refresh token, access token, vault passphrase, KEK, unwrapped vault data key, or decrypted item payloads                                                         |
| Redis                 | Rate-limit and lockout state, HMAC-protected identities, bounded operational metadata                                                                                                                                                  | Passwords, tokens, vault passphrases, encryption keys, decrypted item payloads, ciphertext payloads, raw identities, database URLs, request bodies, or raw dependency errors                     |
| Logs and telemetry    | Safe request IDs, normalized routes, status classes, bounded error codes, safe operation names, sanitized build metadata                                                                                                               | Tokens, cookies, passwords, vault passphrases, keys, decrypted item payloads, ciphertext blobs, nonces, raw SQL, Redis keys, raw URLs, or raw resource identifiers                               |
| CI and GitHub         | Source code, public test fixtures, placeholder configuration, synthetic database and Redis credentials                                                                                                                                 | Real credentials, signing keys, HMAC keys, access tokens, real environment files, or real vault data                                                                                             |

## 6. Threats and mitigations

| Threat                                | Example                                                                            | Mitigation                                                                                                                                                                            |
| ------------------------------------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Account enumeration                   | Login response differs for an unknown email                                        | Current: same generic invalid-credentials response for unknown users, incorrect passwords, invalid input, and disabled accounts                                                       |
| Password brute force                  | Repeated automated login attempts                                                  | Current: Argon2id, Redis fixed-window login limits, failed-login counters, and temporary lockouts keyed by normalized email plus direct peer IP                                       |
| Registration abuse                    | Automated account creation                                                         | Current: Redis fixed-window registration limit by direct peer IP                                                                                                                      |
| Refresh abuse                         | Repeated refresh attempts                                                          | Current: Redis fixed-window refresh limit by direct peer IP, bodyless request contract, CSRF validation, and cookie rotation                                                          |
| Mutation abuse                        | Repeated authenticated writes                                                      | Current: Redis fixed-window mutation limit by authenticated user ID                                                                                                                   |
| Forwarded-IP spoofing                 | Attacker supplies `X-Forwarded-For: 127.0.0.1`                                     | Current: direct TCP peer only; forwarded-IP headers are ignored                                                                                                                       |
| Redis identity disclosure             | Email or IP appears in Redis keys                                                  | Current: HMAC-SHA-256 over length-prefixed identity parts and regression tests that scan Redis keys and values                                                                        |
| Database theft                        | Attacker obtains a database dump                                                   | Current: Argon2id password hashes, refresh-token digests, and ciphertext-only item values for normal encrypted item workflows                                                         |
| Redis snapshot theft                  | Attacker obtains temporary Redis data                                              | Current: only bounded counters and lockout markers with opaque keys and TTLs; local Redis persistence is disabled                                                                     |
| Session fixation or confusion         | Two logins unexpectedly share one session identity                                 | Current: each login creates a new random token family                                                                                                                                 |
| Stolen access token                   | Attacker uses a valid token after logout                                           | Current: protected requests verify active PostgreSQL session state                                                                                                                    |
| Refresh-token theft                   | Stolen refresh token is submitted                                                  | Current: `HttpOnly`, `SameSite=Strict` cookie transport, opaque random tokens, digest-only storage, rotation, bounded lifetime, and family revocation                                 |
| Refresh-token replay                  | An old refresh token is reused after rotation                                      | Current: replay detection and token-family revocation                                                                                                                                 |
| Cross-site request forgery            | A third-party site triggers a cookie-authenticated refresh                         | Current: `SameSite=Strict`, bodyless refresh requests, and exact CSRF cookie/header matching                                                                                          |
| Cross-user session revocation         | User guesses another user's session ID                                             | Current: revoke queries require both authenticated user ID and token-family ID                                                                                                        |
| Insecure direct object reference      | User guesses another user's vault or item ID                                       | Current: authenticated owner ID is supplied by middleware and every vault and item query is owner-scoped                                                                              |
| Lost update                           | Two clients update the same item concurrently                                      | Current: strong ETags and required `If-Match` versions reject stale updates                                                                                                           |
| Duplicate create replay               | A client retries an item creation request                                          | Current: PostgreSQL-backed idempotency keys replay the same request and reject conflicting reuse                                                                                      |
| Audit-event secret leakage            | A payload or key is copied into an outbox event                                    | Current: versioned allow-listed metadata and regression tests reject secret-bearing audit fields                                                                                      |
| Log leakage                           | Password or token enters a log                                                     | Current: allow-listed structured logging, bounded request IDs, and no-secret regression tests                                                                                         |
| Metrics cardinality or secret leakage | Raw vault IDs or query values become labels                                        | Current: normalized Chi route patterns, bounded method and status labels, loopback-only exposure, and secret-exclusion tests                                                          |
| Diagnostics leakage                   | Build endpoint exposes environment variables or dependency details                 | Current: sanitized service, version, and commit only, with `Cache-Control: no-store`                                                                                                  |
| Internal endpoint spoofing            | External caller claims a loopback forwarding address                               | Current: metrics authorization uses the direct peer and ignores forwarding headers                                                                                                    |
| PostgreSQL outage                     | Database stops while the API is running                                            | Current: readiness fails, data-dependent requests return sanitized `503`, liveness remains available, and the existing pool recovers after restart                                    |
| Redis outage                          | Redis stops while the API is running                                               | Current: readiness fails; registration, login, refresh, and mutations fail closed; read-only routes avoid Redis; the client recovers after restart                                    |
| Hash-service outage                   | Rust hash-service is unavailable during registration or login                      | Current: service boundary fails closed with safe dependency errors and does not fall back to weaker hashing                                                                           |
| Password-service outage               | Rust password-service is unavailable during password generation or strength checks | Current: password-tool requests return safe dependency errors without exposing service details                                                                                        |
| Error leakage                         | Database or Redis connection details appear in an API response                     | Current: dependency errors map to generic public responses, and startup and outage tests reject secret connection details                                                             |
| Oversized input                       | Attacker submits huge headers, bodies, tokens, queries, or identifiers             | Current: aggregate header, body, token, query, cursor, pagination, `If-Match`, request-ID, and identifier limits                                                                      |
| Slow request or stuck dependency      | Work continues after the client or deadline is gone                                | Current: configurable request and dependency deadlines with context cancellation through handlers, services, repositories, PostgreSQL, and Redis                                      |
| Ciphertext tampering                  | Stored encrypted data is modified                                                  | Current: Rust WASM AES-GCM authenticated encryption rejects modified ciphertext or tags during browser decrypt; malformed encrypted envelopes are rejected before decrypt             |
| Nonce reuse                           | The same AES-GCM nonce is reused with one key                                      | Current: browser encryption generates a fresh random nonce for each item encryption, and API envelope validation enforces nonce length                                                |
| Cross-site scripting                  | Injected JavaScript reads an access token or unlocked vault                        | Current: refresh tokens remain `HttpOnly`; access tokens and unwrapped vault keys stay in memory only; vault keys are cleared on explicit lock, auth loss, hidden tab, and inactivity |
| Supply-chain compromise               | A malicious frontend dependency reads decrypted data                               | Current: minimize dependencies, use lockfiles, scan dependencies, and review sensitive dependency changes                                                                             |
| Token leakage                         | Raw refresh token is stored in PostgreSQL                                          | Current: store only SHA-256 refresh-token digests                                                                                                                                     |
| Excessive data exposure               | API returns password hash or token digest fields                                   | Current: explicit response types containing only safe public fields                                                                                                                   |
| CI secret exposure                    | A real key is committed or printed in CI                                           | Current: Gitleaks scanning, placeholder configuration, restricted permissions, and synthetic fixtures                                                                                 |

## 7. Accepted risks

- VaultForge has not received an independent security audit.
- Only synthetic secrets are permitted during development.
- Normal encrypted item workflows use browser-side encryption and backend ciphertext storage, but VaultForge has not received an independent security review.
- Non-secret metadata such as vault IDs, item IDs, item types, versions, timestamps, salts, and wrapped vault keys remains visible to backend services.
- Production signing-key and HMAC-key storage, rotation, and multi-key verification are not implemented yet.
- Local development may use HTTP and therefore omits the cookie `Secure` flag; production enables it.
- Trusted reverse-proxy support is not implemented. Forwarded client-IP headers are intentionally ignored.
- Metrics currently share the API listener and rely on direct loopback peer enforcement. A dedicated internal listener and production network policy are not implemented yet.
- A successful cross-site scripting attack could access the in-memory access token and readable CSRF token, although not the `HttpOnly` refresh token.
- Account recovery is not implemented.
- Multi-factor authentication is not implemented.
- A compromised browser while the vault is unlocked can access decrypted data.
- Some non-secret metadata, including timestamps and item types, may remain visible to the backend.
- The browser must temporarily hold decrypted values and encryption keys in memory while the vault is unlocked.
- Fixed-window limits can allow bursts near window boundaries.
- Distributed rate limiting depends on Redis availability; security-sensitive requests intentionally become unavailable during a Redis outage.
- Cryptographic parameter choices will require review before any production claim could be made.

## 8. Security invariants

These rules must remain true throughout development:

1. Account authentication and vault encryption remain separate.
2. The backend never receives the vault master passphrase.
3. The backend never receives the KEK or unwrapped vault key.
4. Real or decrypted vault secrets never enter server-side storage during normal encrypted item workflows.
5. Authentication never falls back to a weaker method when the hasher is unavailable.
6. Raw refresh tokens are not stored in PostgreSQL or returned in JSON.
7. Refresh requests require exactly one CSRF cookie and matching `X-CSRF-Token` header.
8. Access tokens are not stored in PostgreSQL or browser persistence.
9. Request bodies, authorization headers, cookies, tokens, token digests, signing seeds, HMAC keys, and dependency URLs are not logged or metered.
10. Real secrets are not used before an independent security review.
11. Every encrypted payload includes a supported version.
12. Authorization is enforced on the server for every protected object.
13. Revoked session families cannot authorize protected requests.
14. Unknown and unowned session IDs are indistinguishable through public errors.
15. Redis contains only bounded, expiring operational metadata.
16. Redis keys and values never contain raw email addresses, IP addresses, user IDs, passwords, tokens, vault payloads, or encryption keys.
17. Security-sensitive Redis enforcement fails closed when Redis is unavailable.
18. PostgreSQL remains authoritative for durable application state and item-creation idempotency.
19. Liveness remains dependency-free, while readiness reflects every required security dependency.
20. Diagnostics expose only sanitized build identity.
21. Metrics use normalized route patterns and bounded labels, never raw resource identifiers or user data.
22. Internal metrics access is based on the direct peer and never on untrusted forwarded-IP headers.
23. Context cancellation and deadlines reach dependency operations.
24. Public dependency errors never include connection details or raw driver messages.

## 9. Known production hardening decisions

- Decide whether item type remains visible to the server after encryption.
- Decide whether vault names remain visible to the server.
- Decide whether created and updated timestamps are acceptable metadata leakage.
- Select final Argon2id parameters after benchmarking.
- Select production signing-key and rate-limit HMAC-key storage and rotation strategy.
- Define trusted reverse-proxy configuration before honoring forwarded client-IP headers.
- Decide whether production metrics use a dedicated internal listener, mutual TLS, or platform network policy in addition to loopback restrictions.
- Decide the inactivity-lock duration for the browser vault.
