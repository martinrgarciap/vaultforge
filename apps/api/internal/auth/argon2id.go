package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	AlgorithmArgon2id = "argon2id"

	defaultArgon2Memory      uint32 = 19 * 1024
	defaultArgon2Iterations  uint32 = 2
	defaultArgon2Parallelism uint8  = 1
	defaultArgon2SaltLength         = 16
	defaultArgon2KeyLength   uint32 = 32

	minArgon2Memory      uint32 = 19 * 1024
	maxArgon2Memory      uint32 = 256 * 1024
	minArgon2Iterations  uint32 = 1
	maxArgon2Iterations  uint32 = 10
	minArgon2Parallelism uint8  = 1
	maxArgon2Parallelism uint8  = 16

	minArgon2SaltLength = 16
	maxArgon2SaltLength = 64
	minArgon2KeyLength  = 16
	maxArgon2KeyLength  = 64

	maxEncodedPasswordHashLength = 512
)

type Argon2idHasher struct {
	random io.Reader
}

type parsedArgon2idHash struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	hash        []byte
}

func NewArgon2idHasher() *Argon2idHasher {
	return &Argon2idHasher{
		random: rand.Reader,
	}
}

// newArgon2idHasher allows tests in this package to provide a controlled
// randomness source without exposing that option to production wiring.
func newArgon2idHasher(
	random io.Reader,
) *Argon2idHasher {
	return &Argon2idHasher{
		random: random,
	}
}

func (hasher *Argon2idHasher) Hash(
	ctx context.Context,
	password string,
) (PasswordHash, error) {
	if err := ctx.Err(); err != nil {
		return PasswordHash{}, err
	}

	if hasher == nil || hasher.random == nil {
		return PasswordHash{},
			fmt.Errorf(
				"hash password: %w",
				ErrPasswordHasherUnavailable,
			)
	}

	normalizedPassword, err :=
		NormalizeAndValidatePassword(password)
	if err != nil {
		return PasswordHash{}, err
	}

	passwordBytes := []byte(normalizedPassword)
	defer clear(passwordBytes)

	salt := make(
		[]byte,
		defaultArgon2SaltLength,
	)

	if _, err := io.ReadFull(
		hasher.random,
		salt,
	); err != nil {
		return PasswordHash{},
			fmt.Errorf(
				"generate password salt: %w",
				ErrPasswordHasherUnavailable,
			)
	}

	derivedHash := argon2.IDKey(
		passwordBytes,
		salt,
		defaultArgon2Iterations,
		defaultArgon2Memory,
		defaultArgon2Parallelism,
		defaultArgon2KeyLength,
	)
	defer clear(derivedHash)

	if err := ctx.Err(); err != nil {
		return PasswordHash{}, err
	}

	encodedHash := encodeArgon2idHash(
		salt,
		derivedHash,
	)

	return PasswordHash{
		Encoded:   encodedHash,
		Algorithm: AlgorithmArgon2id,
	}, nil
}

func (hasher *Argon2idHasher) Verify(
	ctx context.Context,
	password string,
	passwordHash PasswordHash,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if hasher == nil {
		return fmt.Errorf(
			"verify password: %w",
			ErrPasswordHasherUnavailable,
		)
	}

	if passwordHash.Algorithm != AlgorithmArgon2id {
		return ErrPasswordAlgorithmUnsupported
	}

	parsedHash, err := parseArgon2idHash(
		passwordHash.Encoded,
	)
	if err != nil {
		return err
	}
	defer clear(parsedHash.salt)
	defer clear(parsedHash.hash)

	normalizedPassword, err :=
		NormalizeAndValidatePassword(password)
	if err != nil {
		return err
	}

	passwordBytes := []byte(normalizedPassword)
	defer clear(passwordBytes)

	actualHash := argon2.IDKey(
		passwordBytes,
		parsedHash.salt,
		parsedHash.iterations,
		parsedHash.memory,
		parsedHash.parallelism,
		uint32(len(parsedHash.hash)),
	)
	defer clear(actualHash)

	hashesMatch := subtle.ConstantTimeCompare(
		actualHash,
		parsedHash.hash,
	)

	if err := ctx.Err(); err != nil {
		return err
	}

	if hashesMatch != 1 {
		return ErrPasswordMismatch
	}

	return nil
}

func encodeArgon2idHash(
	salt []byte,
	hash []byte,
) string {
	encodedSalt := base64.RawStdEncoding.EncodeToString(
		salt,
	)
	encodedHash := base64.RawStdEncoding.EncodeToString(
		hash,
	)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		defaultArgon2Memory,
		defaultArgon2Iterations,
		defaultArgon2Parallelism,
		encodedSalt,
		encodedHash,
	)
}

func parseArgon2idHash(
	encodedHash string,
) (parsedArgon2idHash, error) {
	if encodedHash == "" ||
		len(encodedHash) > maxEncodedPasswordHashLength {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	parts := strings.Split(encodedHash, "$")

	if len(parts) != 6 ||
		parts[0] != "" ||
		parts[1] != AlgorithmArgon2id {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	version, err := parseUintParameter(
		parts[2],
		"v=",
		32,
	)
	if err != nil ||
		version != uint64(argon2.Version) {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	parameterParts := strings.Split(
		parts[3],
		",",
	)
	if len(parameterParts) != 3 {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	memory, err := parseUintParameter(
		parameterParts[0],
		"m=",
		32,
	)
	if err != nil {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	iterations, err := parseUintParameter(
		parameterParts[1],
		"t=",
		32,
	)
	if err != nil {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	parallelism, err := parseUintParameter(
		parameterParts[2],
		"p=",
		8,
	)
	if err != nil {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	if memory < uint64(minArgon2Memory) ||
		memory > uint64(maxArgon2Memory) ||
		iterations < uint64(minArgon2Iterations) ||
		iterations > uint64(maxArgon2Iterations) ||
		parallelism < uint64(minArgon2Parallelism) ||
		parallelism > uint64(maxArgon2Parallelism) {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	salt, err := base64.RawStdEncoding.DecodeString(
		parts[4],
	)
	if err != nil ||
		len(salt) < minArgon2SaltLength ||
		len(salt) > maxArgon2SaltLength {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	hash, err := base64.RawStdEncoding.DecodeString(
		parts[5],
	)
	if err != nil ||
		len(hash) < minArgon2KeyLength ||
		len(hash) > maxArgon2KeyLength {
		return parsedArgon2idHash{},
			ErrPasswordHashMalformed
	}

	return parsedArgon2idHash{
		memory:      uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(parallelism),
		salt:        salt,
		hash:        hash,
	}, nil
}

func parseUintParameter(
	value string,
	prefix string,
	bitSize int,
) (uint64, error) {
	if !strings.HasPrefix(value, prefix) ||
		len(value) == len(prefix) {
		return 0, ErrPasswordHashMalformed
	}

	parsedValue, err := strconv.ParseUint(
		value[len(prefix):],
		10,
		bitSize,
	)
	if err != nil {
		return 0, ErrPasswordHashMalformed
	}

	return parsedValue, nil
}
