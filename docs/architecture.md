# VaultForge Architecture

## Overview

VaultForge is a backend-first developer secrets vault.

Its architecture intentionally separates:

1. Account authentication
2. Session authentication and authorization
3. Vault unlocking and encryption
4. Persistence
5. Reliability and asynchronous processing
6. Observability

The project is being built incrementally. Components marked as planned are part of the target architecture but are not currently running.

## Current architecture

```mermaid
flowchart LR
    C[React Browser or API Client]
    A[Go API]
    AH[Authentication Handler]
    SC[Session Cookie Manager]
    SH[Session Handler]
    VH[Vault Handler]
    IH[Item Handler]
    M[Bearer Authentication Middleware]
    RL[Redis Rate Limit and Login Protection]
    AS[Authentication Service]
    SS[Session Service]
    VS[Vault Service]
    H[Local Go Argon2id Adapter]
    T[Ed25519 Token Manager]
    P[(PostgreSQL)]
    R[(Redis)]
    O[(Transactional Outbox)]
    D[Diagnostics Handler]
    MX[Metrics Registry]

    C -->|HTTP, JSON, and cookies| A
    A --> MX
    A --> D
    A --> AH
    A --> M
    A --> RL
    M --> SH
    M --> VH
    M --> IH
    AH --> SC
    SH --> SC
    AH --> AS
    AH --> SS
    SH --> SS
    VH --> VS
    IH --> VS
    AS --> H
    SS --> T
    AS --> P
    SS --> P
    VS --> P
    VS --> O
    RL --> R
```

PostgreSQL is the durable source of truth for users, sessions, vaults, items, transactional outbox rows, and item-creation idempotency records.

Redis stores only bounded, expiring operational security state for distributed request limits, failed-login counters, and temporary lockouts. Redis does not store vault data, passwords, tokens, keys, or durable idempotency records.

The diagnostics handler exposes sanitized service, version, and commit metadata. The metrics registry records only bounded HTTP method, normalized Chi route pattern, status class, count, duration, in-flight requests, uptime, and sanitized build metadata.

## Current request flow

### Public authentication requests

Registration:

```text
HTTP JSON request
    ↓
Chi router
    ↓
Bounded request ID, safe logging, metrics, recovery, security headers, timeout
    ↓
Redis fixed-window request limit by direct peer IP
    ↓
Strict JSON decoder and body limit
    ↓
Authentication handler
    ↓
Authentication service
    ├── PasswordHasher
    └── UserStore
          ↓
      PostgreSQL
```

Login:

```text
HTTP JSON request
    ↓
Redis fixed-window request limit by direct peer IP
    ↓
Login lockout check by normalized email plus direct peer IP
    ↓
Authentication and session services
    ├── PasswordHasher
    ├── UserStore
    ├── SessionStore
    ├── AccessTokenProvider
    └── RefreshTokenProvider
          ↓
      PostgreSQL session creation
          ↓
      Best-effort Redis failure-state clearing
```

Login returns the access token and expiration metadata in JSON. It delivers the refresh token through an `HttpOnly` cookie and a separate readable CSRF cookie.

Invalid credentials increment Redis failed-login state. Reaching the configured threshold activates a temporary lockout. Redis failure blocks authentication with a safe dependency response.

Refresh:

```text
Bodyless HTTP request
    ├── HttpOnly refresh cookie
    ├── Readable CSRF cookie
    └── X-CSRF-Token header
          ↓
      Redis fixed-window request limit by direct peer IP
          ↓
      Session cookie manager
          ├── Require exactly one refresh cookie
          ├── Require exactly one CSRF cookie and header
          └── Compare CSRF values
                ↓
            Session service
                ├── Rotate the refresh-token digest
                ├── Preserve absolute family expiration
                └── Issue a new access token
                      ↓
                  Rotate both browser cookies
```

### Protected session, vault, and item requests

```text
HTTP request
    ↓
Chi router
    ↓
Bounded request ID, safe logging, metrics, recovery, security headers, timeout
    ↓
Bearer authentication middleware
    ├── Parse exactly one bounded Authorization header
    ├── Verify Ed25519 access-token signature and claims
    ├── Confirm active session state in PostgreSQL
    └── Store the authenticated principal in request context
    ↓
Read request or mutation
    ├── Read request: no Redis call during request handling
    └── Mutation: Redis fixed-window limit by authenticated user ID
          ↓
      Session, vault, or item handler
          ↓
      Domain service
          ├── Validate bounded input and state transitions
          ├── Enforce ownership using the authenticated user ID
          ├── Enforce idempotency or expected version where required
          └── Map internal failures to safe public errors
                ↓
            PostgreSQL transaction
                ├── Domain mutation
                └── Sanitized transactional outbox event
```

### Operational requests

```text
GET /health/live
    └── Process-only liveness, no dependency calls

GET /health/ready
    └── Two-second composite PostgreSQL and Redis ping

GET /health/diagnostics
    └── Sanitized service, version, and commit only

GET /internal/metrics
    ├── Direct peer must be IPv4 or IPv6 loopback
    ├── Forwarded-IP headers are ignored
    └── Prometheus text with low-cardinality labels only
```

## Current responsibilities

### Go API

- Exposes JSON HTTP routes.
- Handles strict request validation, bounded inputs, and public error mapping.
- Coordinates registration, login, refresh, authorization, and session management.
- Creates, lists, retrieves, renames, and deletes owner-scoped vaults.
- Creates, lists, retrieves, updates, soft-deletes, restores, and permanently deletes owner-scoped items.
- Supports opaque keyset pagination for item collections.
- Enforces PostgreSQL-backed idempotency keys for item creation.
- Enforces strong `ETag` and `If-Match` optimistic concurrency.
- Writes sanitized audit events through a transactional outbox.
- Normalizes account emails.
- Applies account-password policy.
- Calls the replaceable password hasher.
- Issues and verifies Ed25519 access tokens.
- Generates and rotates opaque refresh tokens.
- Delivers refresh tokens through host-only `HttpOnly`, `SameSite=Strict` cookies.
- Enforces double-submit CSRF protection for bodyless refresh requests.
- Rotates and clears browser session cookies.
- Enforces active-session checks for protected requests.
- Applies Redis-backed distributed request limits.
- Applies failed-login tracking and temporary lockouts.
- Uses HMAC-protected Redis identities and ignores forwarded client-IP headers.
- Exposes dependency-free liveness and PostgreSQL-plus-Redis readiness.
- Exposes sanitized build diagnostics.
- Exposes loopback-only low-cardinality HTTP metrics.
- Applies configurable HTTP and dependency deadlines.
- Propagates context cancellation through services and repositories.
- Maps PostgreSQL, Redis, and password-hasher failures to safe public responses.
- Logs safe request metadata only.
- Accepts only synthetic item payloads until browser-side encryption is implemented.

### Local Argon2id adapter

- Implements the `PasswordHasher` interface.
- Normalizes account passwords using Unicode NFC.
- Generates a fresh random salt for each hash.
- Produces a PHC-style Argon2id encoded value.
- Verifies passwords using encoded parameters.
- Uses constant-time hash comparison.
- Rejects malformed hashes and unsafe parameter values.
- Retains no password after the operation completes.

The local adapter is temporary. It exists so the authentication contract can be completed before the Rust service is built.

### Access-token manager

- Signs access tokens with Ed25519.
- Includes the user ID as the JWT subject.
- Includes the session-family ID in the `sid` claim.
- Validates algorithm, key ID, issuer, audience, issued-at, not-before, and expiration.
- Rejects malformed, expired, incorrectly signed, or incorrectly configured tokens.
- Never stores access tokens in PostgreSQL.
- Redacts token and manager values when formatted.

### Session cookie manager

- Issues the host-only refresh cookie with `HttpOnly`, `SameSite=Strict`, and `Path=/v1/auth/refresh`.
- Issues a separate readable CSRF cookie with `SameSite=Strict` and `Path=/`.
- Enables the `Secure` flag in production and permits local HTTP in development and test.
- Requires exactly one refresh cookie, CSRF cookie, and `X-CSRF-Token` header.
- Parses and compares CSRF values without exposing them through formatting.
- Rotates both cookies after successful refresh.
- Clears both cookies after current-session logout, current-session revocation, logout-all, or invalid refresh credentials.

### Session service

- Creates one session family per successful login.
- Generates opaque refresh tokens.
- Stores only refresh-token SHA-256 digests.
- Rotates refresh tokens atomically.
- Preserves token-family identity and absolute refresh expiration.
- Revokes the entire token family when rotated-token replay is detected.
- Validates access tokens against active PostgreSQL session state.
- Lists active session families for the authenticated user.
- Revokes the current session, one owned session, or all user sessions.
- Fails closed when post-commit access-token issuance fails.

### PostgreSQL

Currently stores:

- Users
- Encoded account-password hashes
- Password algorithm identifiers
- Session rows
- Refresh-token SHA-256 digests
- Token-family identifiers
- Session expiration and revocation timestamps
- Session user-agent metadata
- Owner-scoped vault metadata
- Synthetic generic item payloads during the current phase
- Item versions, timestamps, and deletion state
- Sanitized transactional outbox records
- Durable item-creation idempotency records

PostgreSQL never stores plaintext account passwords, raw refresh tokens, access tokens, vault passphrases, unwrapped encryption keys, or real vault secrets.

The current synthetic item payload is intentionally temporary. A future browser-side encryption phase will replace it with an opaque encrypted envelope.

### Redis

Currently stores only:

- Fixed-window request counters
- Failed-login counters
- Temporary login-lockout state

Every Redis identity is transformed with HMAC-SHA-256 over length-prefixed parts before key construction. All records expire according to their policy.

Redis never stores:

- Passwords or password hashes
- Authorization headers
- Access or refresh tokens
- Refresh-token digests
- CSRF tokens
- Signing seeds or HMAC keys
- Database or Redis URLs
- Vault passphrases
- Encryption keys
- Item payloads or vault values
- Raw request bodies
- Raw dependency errors
- Raw email, IP, or user identities as key material

Redis is required at startup and for registration, login, refresh, and authenticated mutations. Read-only vault and item requests do not call Redis during request handling.

## Current account registration flow

```mermaid
sequenceDiagram
    participant C as Client
    participant A as Go API
    participant S as Auth Service
    participant H as PasswordHasher
    participant P as PostgreSQL

    C->>A: POST /v1/auth/register
    A->>S: Email and account password
    S->>S: Normalize email and validate password
    S->>H: Hash password
    H-->>S: Encoded hash and algorithm
    S->>P: Create user
    P-->>S: User ID, status, timestamps
    S-->>A: Safe account
    A-->>C: 201 Created
```

The store receives only the encoded hash and algorithm identifier. It never receives the plaintext password.

## Current account login flow

```mermaid
sequenceDiagram
    participant C as Client
    participant A as Go API
    participant AS as Auth Service
    participant H as PasswordHasher
    participant SS as Session Service
    participant T as Token Manager
    participant SC as Session Cookie Manager
    participant P as PostgreSQL

    C->>A: POST /v1/auth/login with JSON credentials
    A->>AS: Email and account password
    AS->>P: Find normalized email
    P-->>AS: User and encoded hash
    AS->>H: Verify password
    H-->>AS: Match or mismatch
    AS-->>SS: Safe account
    SS->>SS: Generate refresh token
    SS->>P: Store refresh-token digest and session family
    P-->>SS: Session ID, family ID, timestamps
    SS->>T: Issue Ed25519 access token
    T-->>SS: Access token
    SS-->>A: Account, access token, raw refresh token, expiration
    A->>SC: Issue refresh and CSRF cookies
    SC-->>C: HttpOnly refresh cookie and readable CSRF cookie
    A-->>C: 200 JSON with access token and expiration metadata
```

Unknown emails, incorrect passwords, invalid login input, and disabled accounts produce the same public credentials error.

Unknown accounts also consume password-hashing work to reduce obvious timing differences.

If access-token issuance fails after session creation, the newly created token family is revoked.

Login responses use `Cache-Control: no-store`. The refresh token is never included in the JSON response.

## Current refresh flow

```mermaid
sequenceDiagram
    participant C as Client
    participant A as Go API
    participant SC as Session Cookie Manager
    participant SS as Session Service
    participant T as Token Manager
    participant P as PostgreSQL

    C->>A: POST /v1/auth/refresh with no body
    Note over C,A: Refresh cookie, CSRF cookie, and X-CSRF-Token header
    A->>SC: Read refresh cookie and validate CSRF pair
    SC-->>A: Raw refresh token
    A->>SS: Opaque refresh token
    SS->>SS: Parse and SHA-256 digest token
    SS->>SS: Generate replacement refresh token
    SS->>P: Atomically rotate token row
    P-->>SS: User ID, family ID, preserved expiration
    SS->>T: Issue new access token
    T-->>SS: Access token
    SS-->>A: New access token and replacement refresh token
    A->>SC: Rotate refresh and CSRF cookies
    SC-->>C: Replacement cookies
    A-->>C: 200 JSON with new access token and expiration metadata
```

A successful refresh revokes the submitted refresh-token row and inserts the replacement in the same family.

The request body must be empty. The CSRF cookie and `X-CSRF-Token` header must be present exactly once and must match.

Reusing a previously rotated refresh token triggers family-wide revocation.

Invalid, expired, revoked, replayed, malformed, and disabled-user refresh states return the same public `invalid_refresh_token` response and clear stale browser cookies.

Refresh responses use `Cache-Control: no-store`. Replacement refresh tokens are never included in JSON.

## Current protected-request flow

```mermaid
sequenceDiagram
    participant C as Client
    participant M as Auth Middleware
    participant T as Token Manager
    participant P as PostgreSQL
    participant H as Session Handler
    participant S as Session Service

    C->>M: Authorization: Bearer access-token
    M->>T: Verify token
    T-->>M: User ID and session-family ID
    M->>P: Get active session state
    P-->>M: Active session or not found
    M->>H: Request with principal in context
    H->>S: Authorized session operation
    S->>P: List or revoke owned session families
    P-->>S: Result
    S-->>H: Safe domain result
    H-->>C: JSON or 204 No Content
```

Protected requests require both a valid access token and an active PostgreSQL session family.

Revocation therefore invalidates already-issued access tokens immediately instead of waiting for JWT expiration.

## Current session-management behavior

The API exposes:

```text
GET    /v1/sessions
DELETE /v1/sessions
DELETE /v1/sessions/current
DELETE /v1/sessions/{sessionID}
```

The session identifier exposed by the API is the token-family ID.

Refresh-token rotation creates a new session row but does not create a second logical session family.

Ownership is enforced by querying with both:

- Authenticated user ID
- Requested token-family ID

Unknown, already-revoked, and other users’ session identifiers return the same public `session_not_found` result.

Logout of the current session, revocation of the current session by ID, and logout of all sessions clear the browser refresh and CSRF cookies. Revoking a different owned session leaves the current browser cookies unchanged.

## Current vault and item workflow

```mermaid
sequenceDiagram
    participant C as Client
    participant M as Auth Middleware
    participant H as Vault or Item Handler
    participant S as Vault Service
    participant P as PostgreSQL

    C->>M: Bearer access token and request
    M->>P: Confirm active session
    P-->>M: Authenticated principal
    M->>H: Request with owner ID in context
    H->>S: Owner-scoped domain input
    S->>P: Begin transaction
    S->>P: Apply vault or item mutation
    S->>P: Insert sanitized outbox event
    S->>P: Commit transaction
    P-->>S: Safe domain result
    S-->>H: Vault, item, page, or no-content result
    H-->>C: JSON, ETag, Location, or 204
```

Item creation requires an idempotency key. Update, soft-delete, restore, and permanent-delete operations require the current strong item version through `If-Match`.

Item collection pagination uses an opaque URL-safe cursor backed by `updated_at DESC, id DESC` ordering.

Outbox payloads contain only allow-listed versioned metadata. They never include item payloads, vault names, keys, hashes, or other secret-bearing values.

## Target architecture

```mermaid
flowchart LR
    U[Individual Developer]
    B[React and TypeScript Browser]
    W[Rust WASM Crypto Module]
    A[Go REST API]
    H[Rust gRPC Hashing Service]
    P[(PostgreSQL)]
    R[(Redis)]
    Q[RabbitMQ]
    K[Audit Worker]
    O[OpenTelemetry Collector]

    U --> B
    B --> W
    B -->|REST and JSON| A
    W -->|Ciphertext only| A
    A -->|Hash and Verify over gRPC| H
    A --> P
    A --> R
    A --> Q
    Q --> K
    A --> O
    H --> O
    K --> O
```

## Target component boundaries

### Browser client

Current responsibilities:

- Render the registration, login, vault, item, and session-management workflows.
- Use relative `/v1` and `/health` URLs through the Vite development proxy.
- Keep access tokens in React memory only.
- Rely on the server-issued `HttpOnly` refresh cookie.
- Read the separate CSRF cookie and send it through `X-CSRF-Token`.
- Restore authentication after reload through the refresh and CSRF cookies.
- Coordinate one shared refresh request when concurrent protected requests need a new access token.
- Retry each failed protected request at most once after refresh.
- Clear authentication after refresh failure or a final unauthorized retry.
- Guard authenticated and guest-only routes.
- Preserve safe internal return paths across login.
- Parse and validate API response contracts at runtime.
- Support vault creation, listing, retrieval, rename, and deletion.
- Support item creation, listing, pagination, retrieval, update, soft deletion, restoration, and permanent deletion.
- Preserve optimistic-concurrency guarantees through strong `ETag` and `If-Match` versions.
- Show explicit stale-version conflict feedback instead of silently overwriting.
- List active sessions and support targeted, current-device, and all-session revocation.
- Provide loading, empty, retryable error, not-found, unauthorized, and authentication-restoration states.
- Avoid persistent browser storage for access tokens or secret-bearing values.
- Use synthetic item payloads until browser-side encryption is implemented.

Current testing responsibilities:

- Use Vitest and React Testing Library for isolated behavior.
- Use Playwright for a real-stack workflow across React, Go, and PostgreSQL.
- Run axe accessibility scans.
- Check phone, tablet, and desktop layouts for horizontal overflow.

Future browser-encryption responsibilities:

- Collect the separate vault master passphrase during vault unlock.
- Derive and unwrap vault encryption keys through Rust WebAssembly.
- Encrypt vault items before upload.
- Decrypt downloaded vault items.
- Keep unwrapped keys in memory only.
- Clear application references when the vault locks.

### Go API

Current and planned responsibilities:

- Expose REST and JSON routes.
- Manage users, sessions, authorization, vault metadata, and encrypted payloads.
- Call the Rust hashing service through `PasswordHasher`.
- Store opaque encrypted envelopes.
- Coordinate Redis reliability controls.
- Write sanitized events through a transactional outbox.
- Propagate telemetry without recording sensitive values.
- Remain unable to decrypt vault contents.

### Rust hashing service

Planned responsibilities:

- Implement the existing `PasswordHasher` contract over gRPC.
- Hash and verify account passwords with Argon2id.
- Expose health and metrics.
- Enforce deadlines and bounded input.
- Retain no passwords.
- Use no application database.
- Have no access to vault data.

### Rust WebAssembly module

Planned responsibilities:

- Derive a key-encryption key from the vault master passphrase.
- Generate a random vault data-encryption key.
- Wrap and unwrap the vault key.
- Encrypt and decrypt vault items with authenticated encryption.
- Reject modified ciphertext, nonces, tags, and unsupported versions.

### Redis

Implemented responsibilities:

- Rate-limit registration, login, refresh, and authenticated mutations.
- Track short-lived failed-login counters.
- Track temporary login-lockout state.
- Protect identity material with HMAC-derived opaque keys.
- Participate in readiness as a required security dependency.

Redis is not used for durable item idempotency because PostgreSQL already provides that source of truth.

Future Redis use must remain limited to bounded operational metadata. Redis must never store passwords, raw tokens, encryption keys, vault payloads, decrypted data, or raw dependency errors.

### RabbitMQ and audit worker

Planned responsibilities:

- Publish sanitized events from a transactional outbox.
- Process events idempotently.
- Retry transient failures.
- Route poison messages to a dead-letter queue.
- Never place secret content in messages.

## Account authentication versus vault encryption

The account password proves identity to the server.

The vault master passphrase unlocks encrypted data in the browser.

They remain separate so that:

- The server can authenticate a user without learning the vault key.
- Changing the account password does not require re-encrypting vault items.
- Changing the vault passphrase can re-wrap the vault key independently.
- The password-hashing service has no reason to access vault records.
- Backend services cannot decrypt stored vault content.

Both workflows may use Argon2id, but for different purposes, with separate parameters, salts, code, and documentation.

## Repository boundaries

```text
apps/api                Go HTTP API
apps/web                React and TypeScript browser client
services/hash-service   Planned Rust gRPC hashing service
packages/proto          Planned shared Protocol Buffer contracts
deployments             Compose and later Kubernetes configuration
docs                    Architecture, threat model, decisions, and runbooks
```

## Current roadmap position

Completed:

- Product and security boundaries
- Repository and CI foundation
- Go API skeleton
- PostgreSQL schema and migrations
- Account registration and login
- Replaceable password-hashing boundary
- Session creation on login
- Ed25519 access-token issuance and verification
- Opaque refresh-token rotation
- Replay detection and token-family revocation
- Stateful authorization middleware
- Active-session listing and revocation
- Owner-scoped vault lifecycle
- Owner-scoped item lifecycle
- Item pagination
- PostgreSQL-backed idempotent item creation
- Optimistic concurrency with `ETag` and `If-Match`
- Sanitized transactional outbox integration
- Browser-safe refresh cookies and CSRF protection
- Bodyless refresh requests and cookie rotation
- React and TypeScript browser client
- In-memory access-token lifecycle and cookie-based authentication restoration
- Registration and login UI
- Vault and item lifecycle UI
- Session-management UI
- Loading, empty, error, not-found, and unauthorized states
- Vitest and React Testing Library coverage
- Real-stack Playwright coverage with PostgreSQL and Redis
- Automated accessibility and responsive-layout checks
- Redis client lifecycle and composite readiness
- Distributed request limits
- Failed-login tracking and temporary lockouts
- Configurable HTTP and dependency timeouts
- Context cancellation and safe dependency failure mapping
- Request, header, query, pagination, token, and identifier bounds
- PostgreSQL and Redis outage verification
- Sanitized build diagnostics
- Loopback-only low-cardinality HTTP metrics
- Maintained OpenAPI contract for the current HTTP API
- Automated synchronization checks between OpenAPI and Chi routes
- Representative OpenAPI request and response validation
- Focused fuzz tests for cursors, item versions, and bearer tokens
- Documented testing ownership and security-safe fixture policy
- Official real-stack Playwright system smoke test
- Go, web, browser E2E, and secret-scan GitHub Actions jobs

Next:

- Add RabbitMQ audit and notification workflows.

Later phases add RabbitMQ publication, OpenTelemetry and operational runbooks, Rust services, browser-side encryption, production deployment, and release documentation.
