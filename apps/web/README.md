# VaultForge Web

The VaultForge web application is a minimal React and TypeScript client for the VaultForge Go API.

## Current scope

The current frontend foundation includes:

- Vite
- React
- TypeScript strict mode
- Declarative React Router routes
- A minimal application shell
- Placeholder workflow pages
- Vitest
- React Testing Library
- Development proxying to the Go API

Authentication, API requests, vault operations, item operations, and session management will be added in later Step 7 subsections.

## Commands

```bash
npm run dev
npm run format
npm run format:check
npm run lint
npm run typecheck
npm run test
npm run build

During development, Vite proxies /v1 and /health to:

http://127.0.0.1:8080

The frontend must use relative API URLs.

Security boundary

Access tokens must remain in React memory only.

Never store access tokens in:

localStorage
sessionStorage
IndexedDB
URLs
Logs
Error reports

Refresh tokens are managed by the API through an HttpOnly cookie.

Use synthetic vault data only. Browser-side encryption is not implemented.


---
```
