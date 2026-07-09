# VaultForge Web

VaultForge Web is the React and TypeScript client for the VaultForge Go API.

It implements the current user-facing authentication, session, vault, and item workflows, plus a public home page and a public password generator. Vault item values are encrypted and decrypted in the browser with the Rust WASM crypto module before normal create, edit, list, and detail workflows reach the Go API. Only synthetic data belongs in the vault.

## Technology

- React
- TypeScript strict mode
- Vite
- React Router in declarative mode
- Native `fetch`
- React Context for authentication state
- Vitest
- React Testing Library
- Playwright
- axe accessibility scanning
- Plain CSS without a component framework

The client intentionally does not use Redux, Zustand, Axios, React Query, a CSS framework, SSR, or a generated API client.

## Routes

```text
/
/password-generator
/register
/login
/vaults
/vaults/:vaultId
/vaults/:vaultId/items/:itemId
/sessions
```

`/` and `/password-generator` are public and require no account.

Route behavior includes:

- Signed-out users are redirected from protected routes to Login.
- The requested internal path is preserved and restored after successful login.
- Authenticated users are redirected away from Login and Register.
- Authentication loss immediately unmounts protected route content.
- Unknown routes provide an authentication-aware return link.

## Authentication lifecycle

The Go API returns a short-lived access token in login and refresh JSON responses.

The client stores that access token only in React memory. It is never written to browser persistence.

The API manages the refresh token through a host-only `HttpOnly`, `SameSite=Strict` cookie. A separate readable CSRF cookie is sent back through `X-CSRF-Token` during refresh.

On page reload:

1. The in-memory access token is gone.
2. The authentication provider checks for the CSRF cookie.
3. The client sends one bodyless refresh request.
4. A successful refresh stores the new access token in memory and restores authenticated routing.
5. A failed refresh clears local authentication state and leaves the user signed out.

Protected requests:

1. Attach the in-memory access token through `Authorization: Bearer`.
2. Refresh once after a `401`.
3. Retry the original request once with the replacement token.
4. Clear authentication if the retry still receives `401`.

Concurrent refresh attempts share one in-flight refresh promise.

## Current workflows

### Public pages

- Home page introducing the project without requiring an account
- Password generator page with configurable length and character classes, generated entropy, and strength feedback, calling the public `/v1/passwords/generate` and `/v1/passwords/strength` API endpoints

### Account

- Register, with live password-strength feedback from the same public `/v1/passwords/strength` endpoint used by the password generator page
- Login
- Automatic session restoration
- Current-session logout

### Vaults

- Create
- List
- Open
- Rename
- Delete

### Items

- Create
- List active or deleted items
- Keyset pagination
- Open item details
- Reveal and copy sensitive values
- Edit
- Handle stale-version conflicts
- Soft delete
- Restore
- Permanently delete

Supported item types:

- Login
- API key
- Environment variable
- Database connection
- Secure note

### Sessions

- List active session families
- Identify the current browser session
- Revoke another session
- Log out the current device
- Log out all sessions

## Request and error handling

The frontend validates runtime API contracts before using response data.

User-facing states include:

- Initial loading
- Empty collections
- Retryable request failures
- Pagination failures that preserve already-loaded rows
- Vault not found
- Item not found
- Authentication required
- Authentication restoration
- Stale-version conflict feedback
- Unknown routes

The client uses relative API URLs. During local development, Vite proxies:

```text
/v1     -> http://127.0.0.1:8080
/health -> http://127.0.0.1:8080
```

`VAULTFORGE_API_TARGET` can override the proxy target for isolated test environments.

## Commands

Install dependencies:

```bash
npm ci
```

Start the development client:

```bash
npm run dev
```

Format files:

```bash
npm run format
npm run format:check
```

Run static checks:

```bash
npm run lint
npm run typecheck
```

Run Vitest:

```bash
npm run test
npm run test:watch
```

Build the production bundle:

```bash
npm run build
```

Run Playwright directly:

```bash
npm run test:e2e
npm run test:e2e:headed
npm run test:e2e:report
```

From the repository root, prefer:

```bash
make test-e2e
```

The Makefile target resets the isolated `vaultforge_e2e` PostgreSQL database, applies all migrations, starts the Go API on port `8081`, starts Vite on port `4173`, runs Chromium, and then shuts down the test servers.

## Test ownership

Vitest owns:

```text
src/**/*.test.ts
src/**/*.test.tsx
```

Playwright owns:

```text
e2e/**/*.spec.ts
```

The current Vitest suite covers API helpers, authentication state, runtime contracts, route guards, vault pages, item workflows, session workflows, managed clipboard clearing, visible copy feedback, timed sensitive-value reveals, inactivity privacy resets, modal focus behavior, and error states.

The real-stack Playwright workflow covers:

- Registration and login
- Empty `localStorage`, `sessionStorage`, and IndexedDB
- Sensitive-value masking before explicit reveal
- Clipboard copy feedback without browser-persistence or URL leakage
- Absence of synthetic passwords in browser console messages and page errors
- Presence of an `HttpOnly` session cookie
- Vault creation
- Browser-side vault encryption setup and unlock
- Encrypted item creation
- Authentication restoration after reload
- Two-session stale-version conflict handling
- Delete, restore, and permanent deletion
- Session listing and current-session logout
- Protected-route rejection after logout
- axe accessibility scans
- Phone, tablet, and desktop overflow checks

This Playwright workflow is the official VaultForge complete-stack smoke test.
It replaces the need for a second Postman or command-line smoke workflow and is
run by both `make verify` and GitHub Actions.

See [`../../docs/testing.md`](../../docs/testing.md) for the complete testing
and QA strategy.

Synthetic data is generated uniquely for each Playwright run.

## Security boundary

Access tokens must remain in React memory only.

Never store access tokens in:

- `localStorage`
- `sessionStorage`
- IndexedDB
- URLs
- Logs
- Error reports

Never expose or log:

- Plaintext passwords
- Authorization headers
- Refresh cookies
- CSRF token values
- Access or refresh tokens
- Signing keys
- Database URLs
- Vault passphrases
- Encryption keys
- Real credential values

Refresh tokens are managed by the API through an `HttpOnly` cookie. The client reads only the separate CSRF cookie required for refresh.

Use synthetic vault data only. Normal item create, edit, list, and detail workflows encrypt and decrypt item values in the browser with the Rust WASM crypto module. The Go API and PostgreSQL store ciphertext envelopes, nonces, wrapped vault keys, salts, and non-secret metadata, but they must not receive vault passphrases, key-encryption keys, unwrapped vault data keys, or decrypted item payloads.

Sensitive values are masked by default and automatically re-mask 15 seconds after reveal. Successful copies display temporary visual confirmation and accessible status feedback. VaultForge attempts to clear copied values after 30 seconds only when browser support allows reading the clipboard and the clipboard still contains the copied value. Revealed values are also hidden after five minutes of inactivity or when the browser tab becomes hidden. These controls do not cryptographically lock or encrypt the vault.

## Related documentation

- [`../../README.md`](../../README.md)
- [`../api/README.md`](../api/README.md)
- [`../../SECURITY.md`](../../SECURITY.md)
- [`../../docs/architecture.md`](../../docs/architecture.md)
- [`../../docs/testing.md`](../../docs/testing.md)
- [`../../docs/threat-model.md`](../../docs/threat-model.md)
