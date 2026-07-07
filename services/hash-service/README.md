# VaultForge Hash Service

The VaultForge hash service is a Rust gRPC service for account password hashing and verification.

It is part of VaultForge Step 14 and handles account authentication password hashing only. It does not perform vault-item encryption, browser-side key derivation, or WebAssembly cryptography.

VaultForge is a portfolio and learning project. Use synthetic data only.

## Scope

This service supports:

- Argon2id password hashing
- Fresh random salt generation per hash
- PHC-encoded password hash output
- PHC hash verification
- Safe input validation
- Safe gRPC error mapping
- gRPC health checks
- gRPC reflection for local debugging

This service does not:

- Store passwords
- Store password hashes
- Receive vault master passphrases
- Derive vault encryption keys
- Encrypt or decrypt vault items
- Replace the Go API password hasher yet

Go API integration belongs to Step 15.

## gRPC contract

Proto file:

```text
packages/proto/vaultforge/hash/v1/hash.proto
```

Service:

```text
vaultforge.hash.v1.HashService
```

RPCs:

```text
HashPassword(HashPasswordRequest) returns (HashPasswordResponse)
VerifyPassword(VerifyPasswordRequest) returns (VerifyPasswordResponse)
```

## HashPassword

Input:

```json
{
  "password": "synthetic-password-only"
}
```

Output:

```json
{
  "passwordHash": "$argon2id$..."
}
```

The returned value is a PHC-encoded Argon2id hash.

## VerifyPassword

Input:

```json
{
  "password": "synthetic-password-only",
  "passwordHash": "$argon2id$..."
}
```

Correct password output:

```json
{
  "verified": true
}
```

Wrong password output:

```json
{}
```

In protobuf JSON output, `{}` means `verified` is false because false is the default boolean value.

## Error behavior

| Case                     | gRPC status       | Message                         |
| ------------------------ | ----------------- | ------------------------------- |
| Empty password           | `InvalidArgument` | `password must not be empty`    |
| Oversized password       | `InvalidArgument` | Safe size-limit message         |
| Empty hash               | `InvalidArgument` | `stored hash must not be empty` |
| Oversized hash           | `InvalidArgument` | Safe size-limit message         |
| Malformed PHC hash       | `InvalidArgument` | `stored hash is malformed`      |
| Wrong password           | OK response       | `verified=false`                |
| Internal hashing failure | `Internal`        | Secret-free internal error      |

Error messages must never include plaintext passwords, password hashes, tokens, keys, or vault data.

## Local commands

From the repository root:

```bash
make verify-rust
```

From this service folder:

```bash
cargo fmt --check
cargo clippy -- -D warnings
cargo test
cargo build
cargo run
```

The service listens on:

```text
127.0.0.1:50051
```

## grpcurl examples

List exposed services:

```bash
grpcurl -plaintext 127.0.0.1:50051 list
```

Expected services include:

```text
grpc.health.v1.Health
grpc.reflection.v1.ServerReflection
vaultforge.hash.v1.HashService
```

Check health:

```bash
grpcurl -plaintext \
  -d '{"service":"vaultforge.hash.v1.HashService"}' \
  127.0.0.1:50051 \
  grpc.health.v1.Health/Check
```

Hash a synthetic password:

```bash
grpcurl -plaintext \
  -d '{"password":"correct horse battery staple"}' \
  127.0.0.1:50051 \
  vaultforge.hash.v1.HashService/HashPassword
```

Verify a synthetic password:

```bash
grpcurl -plaintext \
  -d '{"password":"correct horse battery staple","passwordHash":"PASTE_SYNTHETIC_PHC_HASH_HERE"}' \
  127.0.0.1:50051 \
  vaultforge.hash.v1.HashService/VerifyPassword
```

Never use real passwords or real secrets in manual testing.
