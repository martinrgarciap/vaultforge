package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

const testPassword = "correct horse battery staple"

func TestArgon2idHasherHashAndVerify(
	t *testing.T,
) {
	hasher := NewArgon2idHasher()

	passwordHash, err := hasher.Hash(
		context.Background(),
		testPassword,
	)
	if err != nil {
		t.Fatalf(
			"Hash() returned unexpected error: %v",
			err,
		)
	}

	if passwordHash.Algorithm != AlgorithmArgon2id {
		t.Fatalf(
			"Hash() algorithm = %q, want %q",
			passwordHash.Algorithm,
			AlgorithmArgon2id,
		)
	}

	if !strings.HasPrefix(
		passwordHash.Encoded,
		"$argon2id$",
	) {
		t.Fatalf(
			"Hash() returned unexpected encoded format",
		)
	}

	err = hasher.Verify(
		context.Background(),
		testPassword,
		passwordHash,
	)
	if err != nil {
		t.Fatalf(
			"Verify() returned unexpected error: %v",
			err,
		)
	}
}

func TestArgon2idHasherUsesUniqueSalts(
	t *testing.T,
) {
	hasher := NewArgon2idHasher()

	firstHash, err := hasher.Hash(
		context.Background(),
		testPassword,
	)
	if err != nil {
		t.Fatalf(
			"first Hash() returned unexpected error: %v",
			err,
		)
	}

	secondHash, err := hasher.Hash(
		context.Background(),
		testPassword,
	)
	if err != nil {
		t.Fatalf(
			"second Hash() returned unexpected error: %v",
			err,
		)
	}

	if firstHash.Encoded == secondHash.Encoded {
		t.Fatal(
			"Hash() returned identical encoded hashes for the same password",
		)
	}
}

func TestArgon2idHasherRejectsIncorrectPassword(
	t *testing.T,
) {
	hasher := NewArgon2idHasher()

	passwordHash, err := hasher.Hash(
		context.Background(),
		testPassword,
	)
	if err != nil {
		t.Fatalf(
			"Hash() returned unexpected error: %v",
			err,
		)
	}

	err = hasher.Verify(
		context.Background(),
		"incorrect horse battery staple",
		passwordHash,
	)

	if !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			ErrPasswordMismatch,
		)
	}
}

func TestArgon2idHasherNormalizesEquivalentUnicode(
	t *testing.T,
) {
	hasher := NewArgon2idHasher()

	composedPassword := strings.Repeat(
		"é",
		MinPasswordLength,
	)
	decomposedPassword := strings.Repeat(
		"e\u0301",
		MinPasswordLength,
	)

	passwordHash, err := hasher.Hash(
		context.Background(),
		composedPassword,
	)
	if err != nil {
		t.Fatalf(
			"Hash() returned unexpected error: %v",
			err,
		)
	}

	err = hasher.Verify(
		context.Background(),
		decomposedPassword,
		passwordHash,
	)
	if err != nil {
		t.Fatalf(
			"Verify() returned unexpected error: %v",
			err,
		)
	}
}

func TestArgon2idHasherRejectsMalformedHashes(
	t *testing.T,
) {
	encodedSalt := base64.RawStdEncoding.EncodeToString(
		bytes.Repeat(
			[]byte{1},
			minArgon2SaltLength,
		),
	)
	encodedHash := base64.RawStdEncoding.EncodeToString(
		bytes.Repeat(
			[]byte{2},
			minArgon2KeyLength,
		),
	)

	tests := []struct {
		name        string
		encodedHash string
	}{
		{
			name:        "empty hash",
			encodedHash: "",
		},
		{
			name:        "invalid format",
			encodedHash: "not-a-password-hash",
		},
		{
			name: "unsupported encoded algorithm",
			encodedHash: fmt.Sprintf(
				"$argon2i$v=%d$m=%d,t=2,p=1$%s$%s",
				argon2.Version,
				defaultArgon2Memory,
				encodedSalt,
				encodedHash,
			),
		},
		{
			name: "unsupported version",
			encodedHash: fmt.Sprintf(
				"$argon2id$v=999$m=%d,t=2,p=1$%s$%s",
				defaultArgon2Memory,
				encodedSalt,
				encodedHash,
			),
		},
		{
			name: "memory exceeds safety bound",
			encodedHash: fmt.Sprintf(
				"$argon2id$v=%d$m=%d,t=2,p=1$%s$%s",
				argon2.Version,
				maxArgon2Memory+1,
				encodedSalt,
				encodedHash,
			),
		},
		{
			name: "invalid salt encoding",
			encodedHash: fmt.Sprintf(
				"$argon2id$v=%d$m=%d,t=2,p=1$invalid*$%s",
				argon2.Version,
				defaultArgon2Memory,
				encodedHash,
			),
		},
	}

	hasher := NewArgon2idHasher()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := hasher.Verify(
				context.Background(),
				testPassword,
				PasswordHash{
					Encoded:   test.encodedHash,
					Algorithm: AlgorithmArgon2id,
				},
			)

			if !errors.Is(
				err,
				ErrPasswordHashMalformed,
			) {
				t.Fatalf(
					"Verify() error = %v, want %v",
					err,
					ErrPasswordHashMalformed,
				)
			}
		})
	}
}

func TestArgon2idHasherRejectsUnsupportedAlgorithm(
	t *testing.T,
) {
	hasher := NewArgon2idHasher()

	err := hasher.Verify(
		context.Background(),
		testPassword,
		PasswordHash{
			Encoded:   "unused",
			Algorithm: "bcrypt",
		},
	)

	if !errors.Is(
		err,
		ErrPasswordAlgorithmUnsupported,
	) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			ErrPasswordAlgorithmUnsupported,
		)
	}
}

func TestArgon2idHasherHonorsCanceledContext(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	hasher := NewArgon2idHasher()

	_, err := hasher.Hash(
		ctx,
		testPassword,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			context.Canceled,
		)
	}

	err = hasher.Verify(
		ctx,
		testPassword,
		PasswordHash{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}

func TestArgon2idHasherMapsRandomFailureSafely(
	t *testing.T,
) {
	hasher := newArgon2idHasher(
		failingRandomReader{},
	)

	_, err := hasher.Hash(
		context.Background(),
		testPassword,
	)

	if !errors.Is(
		err,
		ErrPasswordHasherUnavailable,
	) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			ErrPasswordHasherUnavailable,
		)
	}

	if strings.Contains(
		err.Error(),
		"synthetic random failure",
	) {
		t.Fatal(
			"Hash() exposed the underlying randomness error",
		)
	}
}

func TestArgon2idHasherRejectsInvalidPassword(
	t *testing.T,
) {
	hasher := NewArgon2idHasher()

	_, err := hasher.Hash(
		context.Background(),
		"too short",
	)

	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			ErrPasswordTooShort,
		)
	}
}

type failingRandomReader struct{}

func (failingRandomReader) Read(
	_ []byte,
) (int, error) {
	return 0, errors.New(
		"synthetic random failure",
	)
}
