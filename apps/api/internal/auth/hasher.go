package auth

import "context"

// PasswordHash contains the encoded account-password hash and the
// algorithm identifier that must be persisted with it.
type PasswordHash struct {
	Encoded   string `json:"-"`
	Algorithm string `json:"-"`
}

// PasswordHasher hashes and verifies account passwords.
//
// Implementations may be local Go adapters, test fakes, or remote
// services such as the future Rust gRPC password-hashing service.
type PasswordHasher interface {
	Hash(
		ctx context.Context,
		password string,
	) (PasswordHash, error)

	Verify(
		ctx context.Context,
		password string,
		passwordHash PasswordHash,
	) error
}
