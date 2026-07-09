# VaultForge Password Service

A Rust gRPC service for **password generation** and **password strength rating**,
part of VaultForge.

The Go API exposes this service publicly through `POST /v1/passwords/generate` and
`POST /v1/passwords/strength`. This service itself is internal: it listens only on
`services/password-service`'s own gRPC port behind the Go API and is never exposed
directly to the browser or the public internet. Generated passwords and submitted
strength-check inputs are not stored or logged by this service or by the Go API.

> **Scope & security note:** This service generates account-level passwords and
> rates password strength for VaultForge's public generator page and registration
> form. It is a portfolio/demo project using synthetic data — **not** an audited or
> production-safe security product. It does **not** handle vault-item secrets
> (those are browser-side only) or account password hashing (that is the separate
> `hash-service`).

## RPCs

Proto: `packages/proto/vaultforge/password/v1/password.proto`
Package: `vaultforge.password.v1`

### `Generate(GenerateRequest) -> GenerateResponse`

Generates a random password from the requested character classes.

- Request: `length`, `include_uppercase`, `include_lowercase`, `include_digits`,
  `include_symbols`, `exclude_chars`
- Response: `password`, `entropy_bits`
- Uses a cryptographically secure RNG (`OsRng`) with bias-free selection.
- Guarantees at least one character from each enabled class, then shuffles.

### `CheckStrength(CheckStrengthRequest) -> CheckStrengthResponse`

Rates a password's strength using the `zxcvbn` algorithm.

- Request: `password`
- Response: `score` (0–4), `label`, `entropy_bits`, `crack_time_estimate`,
  `suggestions`

## Validation

- `length` must be within `[min_length, max_length]` (default 4–256).
- At least one character class must be enabled.
- Each enabled class must retain usable characters after `exclude_chars`.
- Validation failures return gRPC `invalid_argument`; internal failures return
  `internal`. Error messages never include the password or generated output.

## Configuration

Environment variables (with defaults):

| Variable                      | Default           | Description             |
| ----------------------------- | ----------------- | ----------------------- |
| `PASSWORD_SERVICE_BIND_ADDR`  | `127.0.0.1:50053` | gRPC bind address       |
| `PASSWORD_SERVICE_MIN_LENGTH` | `4`               | Minimum password length |
| `PASSWORD_SERVICE_MAX_LENGTH` | `256`             | Maximum password length |

## Running locally

```bash
cd services/password-service
cargo run
# password-service listening on 127.0.0.1:50053
```

## Health & reflection

- Health: `grpc.health.v1.Health/Check` returns `SERVING`.
- gRPC reflection is enabled (introspect with `grpcurl -plaintext <addr> list`).

## Verification

```bash
make verify-password-service   # fmt-check, clippy -D warnings, test, build
```
