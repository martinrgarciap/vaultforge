package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const (
	refreshTokenPrefix       = "vf_rt_"
	refreshTokenEntropyBytes = 32
)

var refreshTokenEncodedLength = base64.RawURLEncoding.EncodedLen(
	refreshTokenEntropyBytes,
)

type RefreshToken struct {
	value string
}

type RefreshTokenDigest [sha256.Size]byte

type RefreshTokenGenerator struct {
	random io.Reader
}

func NewRefreshTokenGenerator() *RefreshTokenGenerator {
	return &RefreshTokenGenerator{
		random: rand.Reader,
	}
}

func (generator *RefreshTokenGenerator) Generate(
	ctx context.Context,
) (RefreshToken, error) {
	if err := ctx.Err(); err != nil {
		return RefreshToken{}, err
	}

	if generator == nil ||
		generator.random == nil {
		return RefreshToken{}, fmt.Errorf(
			"generate refresh token: %w",
			ErrRefreshTokenGenerationUnavailable,
		)
	}

	randomBytes := make(
		[]byte,
		refreshTokenEntropyBytes,
	)

	if _, err := io.ReadFull(
		generator.random,
		randomBytes,
	); err != nil {
		return RefreshToken{}, fmt.Errorf(
			"generate refresh token: %w",
			ErrRefreshTokenGenerationUnavailable,
		)
	}

	if err := ctx.Err(); err != nil {
		return RefreshToken{}, err
	}

	encodedValue :=
		base64.RawURLEncoding.EncodeToString(
			randomBytes,
		)

	return RefreshToken{
		value: refreshTokenPrefix + encodedValue,
	}, nil
}

func ParseRefreshToken(
	value string,
) (RefreshToken, error) {
	if len(value) !=
		len(refreshTokenPrefix)+
			refreshTokenEncodedLength ||
		!strings.HasPrefix(
			value,
			refreshTokenPrefix,
		) {
		return RefreshToken{},
			ErrRefreshTokenMalformed
	}

	encodedValue := strings.TrimPrefix(
		value,
		refreshTokenPrefix,
	)

	decodedValue, err :=
		base64.RawURLEncoding.DecodeString(
			encodedValue,
		)
	if err != nil ||
		len(decodedValue) !=
			refreshTokenEntropyBytes ||
		base64.RawURLEncoding.EncodeToString(
			decodedValue,
		) != encodedValue {
		return RefreshToken{},
			ErrRefreshTokenMalformed
	}

	return RefreshToken{
		value: value,
	}, nil
}

func (token RefreshToken) Value() string {
	return token.value
}

func (token RefreshToken) Digest() (
	RefreshTokenDigest,
	error,
) {
	parsedToken, err := ParseRefreshToken(
		token.value,
	)
	if err != nil {
		return RefreshTokenDigest{}, err
	}

	return sha256.Sum256(
		[]byte(parsedToken.value),
	), nil
}

func (token RefreshToken) Format(
	state fmt.State,
	_ rune,
) {
	_, _ = io.WriteString(
		state,
		"[REDACTED]",
	)
}

func (digest RefreshTokenDigest) Bytes() []byte {
	copiedDigest := make(
		[]byte,
		len(digest),
	)

	copy(
		copiedDigest,
		digest[:],
	)

	return copiedDigest
}

func (digest RefreshTokenDigest) Format(
	state fmt.State,
	_ rune,
) {
	_, _ = io.WriteString(
		state,
		"[REDACTED]",
	)
}
