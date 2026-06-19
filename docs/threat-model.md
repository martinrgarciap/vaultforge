# VaultForge Threat Model

## 1. Security objectives

VaultForge is intended to:

- Prevent backend services from reading decrypted vault contents after the
  browser-side encryption phase is complete.
- Store account passwords using a slow password-hashing algorithm.
- Store refresh tokens only in hashed form.
- Enforce server-side authorization for every protected resource.
- Detect modification of encrypted vault data.
- Prevent secrets from leaking into logs, traces, metrics, queues, URLs,
  analytics, screenshots, or browser persistence.
- Fail safely when required security dependencies are unavailable.

VaultForge is not intended to protect a user whose browser or device is fully
compromised while the vault is unlocked.

## 2. Protected assets

- Account passwords
- Account password hashes
- Access tokens
- Refresh tokens
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
- An attacker with a stolen refresh token
- An attacker exploiting an authorization flaw
- An attacker exploiting cross-site scripting
- A compromised frontend dependency
- A developer accidentally logging sensitive data
- An attacker able to modify stored ciphertext
- A compromised infrastructure operator
- An attacker with access to a CI environment
- An attacker attempting account enumeration or password brute force

## 4. Trust boundaries

### Browser to Go API

The browser sends authentication requests and encrypted vault envelopes to the
Go API.

After client-side encryption is implemented, plaintext vault contents must
never cross this boundary.

### Go API to Rust hashing service

The Go API sends account passwords to the Rust service only during account
registration, login, or password changes.

The hashing service must not receive vault passphrases, encryption keys, vault
items, session tokens, or unrelated user data.

### Go API to PostgreSQL

The Go API stores account records, password hashes, hashed refresh tokens,
vault metadata, and encrypted vault envelopes.

PostgreSQL must never store plaintext account passwords, raw refresh tokens,
vault passphrases, unwrapped keys, or decrypted vault items.

### Go API to Redis

Redis stores rate-limit counters, lockout state, and other short-lived
coordination information.

Redis must never store passwords, raw tokens, encryption keys, or vault
contents.

### Application to logs and telemetry

Only explicitly approved operational metadata may enter logs or telemetry.

Request bodies, authorization headers, cookies, passwords, hashes, tokens,
vault payloads, and encryption keys must never be recorded.

## 5. Component visibility

| Component            | Allowed to see                                                                                                                         | Must never see or retain                                                                                                      |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Browser              | Account password while entered, vault master passphrase, KEK, unwrapped DEK, and decrypted items while unlocked                        | Keys or decrypted content in localStorage, sessionStorage, URLs, analytics, persistent logs, or error reports                 |
| Go API               | Account password transiently during authentication, user ID, email, authorization claims, vault metadata, ciphertext, and wrapped keys | Vault master passphrase, KEK, unwrapped DEK, decrypted items, raw refresh tokens in persistent storage                        |
| Rust hashing service | Account password transiently and PHC password hash while processing                                                                    | Vault passphrase, vault key, vault items, session tokens, or persistent password storage                                      |
| PostgreSQL           | Email, password hash, password algorithm, hashed refresh token, vault metadata, ciphertext, nonce, salt, and wrapped key               | Plaintext account password, raw refresh token, vault passphrase, unwrapped key, or plaintext vault item                       |
| Redis                | Rate-limit identifiers, failed-login counters, and short-lived lockout state                                                           | Passwords, password hashes, raw tokens, vault secrets, encryption keys, or decrypted data                                     |
| Logs and telemetry   | Request ID, trace ID, method, route template, status, duration, sanitized user ID, and dependency status                               | Request bodies, Authorization headers, cookies, passwords, hashes, tokens, vault payloads, encryption keys, or decrypted data |
| CI and GitHub        | Source code, public test fixtures, and placeholder configuration                                                                       | Credentials, signing keys, access tokens, real environment files, or real vault data                                          |

## 6. Threats and mitigations

| Threat                           | Example                                                    | Planned mitigation                                                                                     |
| -------------------------------- | ---------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Account enumeration              | Login response differs for an unknown email                | Return the same generic invalid-credentials response for unknown users and incorrect passwords         |
| Password brute force             | Repeated automated login attempts                          | Argon2id password hashing, rate limiting, and short-lived lockouts                                     |
| Database theft                   | Attacker obtains a database dump                           | Argon2id password hashes, hashed refresh tokens, and ciphertext-only vault items                       |
| Insecure direct object reference | User guesses another user's vault or item ID               | Server-side owner checks for every vault and item operation                                            |
| Refresh-token theft              | An old refresh token is reused after rotation              | Refresh-token rotation, token families, replay detection, and family revocation                        |
| Secret leakage                   | Password or vault item enters a log                        | Structured allow-listed logging and automated no-secret logging tests                                  |
| Ciphertext tampering             | Stored encrypted data is modified                          | AES-GCM authenticated encryption and authentication-tag verification                                   |
| Nonce reuse                      | The same AES-GCM nonce is reused with one key              | Secure random nonce generation and automated tests                                                     |
| Cross-site scripting             | Injected JavaScript reads an unlocked vault                | Restrictive CSP, dependency review, safe rendering, and short in-memory key lifetime                   |
| Hashing-service outage           | The Rust service is unavailable during login               | Authentication fails closed and returns a safe dependency error                                        |
| Supply-chain compromise          | A malicious frontend dependency reads decrypted data       | Minimize dependencies, use lockfiles, run dependency scanning, and review sensitive dependency changes |
| Token leakage                    | Raw refresh token is stored in PostgreSQL                  | Store only a cryptographic hash of the refresh token                                                   |
| Excessive data exposure          | API returns password hash or token fields                  | Use explicit response types containing only safe fields                                                |
| Error leakage                    | Database connection information appears in an API response | Map internal dependency errors to generic public error responses                                       |
| Oversized input                  | Attacker submits a very large request body                 | Enforce request-body and field-length limits                                                           |
| CI secret exposure               | A real key is committed or printed in CI                   | Gitleaks scanning, placeholder configuration, and restricted CI permissions                            |

## 7. Accepted risks

- VaultForge has not received an independent security audit.
- Only synthetic secrets are permitted during development.
- The first sprint may not include browser-side encryption.
- Local development may use HTTP between local services.
- Account recovery is not implemented.
- A compromised browser while the vault is unlocked can access decrypted data.
- Some non-secret metadata, including timestamps and item types, may remain
  visible to the backend.
- The browser must temporarily hold decrypted values and encryption keys in
  memory while the vault is unlocked.
- Denial-of-service protection will be limited during early development.
- Cryptographic parameter choices will require review before any production
  claim could be made.

## 8. Security invariants

These rules must remain true throughout development:

1. Account authentication and vault encryption remain separate.
2. The backend never receives the vault master passphrase.
3. The backend never receives the unwrapped vault data-encryption key.
4. Decrypted vault items never enter server-side storage.
5. Authentication never falls back to a weaker method when the hasher is
   unavailable.
6. Raw refresh tokens are not stored in PostgreSQL.
7. Request bodies and authorization headers are not logged.
8. Real secrets are not used before client-side encryption is complete.
9. Every encrypted payload includes a supported version.
10. Authorization is enforced on the server for every protected object.

## 9. Open decisions

- TODO: Decide whether item type remains visible to the server after encryption.
- TODO: Decide whether vault names remain visible to the server.
- TODO: Decide whether created and updated timestamps are considered acceptable
  metadata leakage.
- TODO: Select final Argon2id parameters after benchmarking.
- TODO: Decide whether refresh tokens use secure cookies in the initial web
  deployment.
- TODO: Decide the inactivity-lock duration for the browser vault.
