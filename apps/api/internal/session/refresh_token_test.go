package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRefreshTokenGeneratorGenerate(
	t *testing.T,
) {
	randomBytes := bytes.Repeat(
		[]byte{0xab},
		refreshTokenEntropyBytes,
	)

	generator := &RefreshTokenGenerator{
		random: bytes.NewReader(
			randomBytes,
		),
	}

	token, err := generator.Generate(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"generate refresh token: %v",
			err,
		)
	}

	expectedValue := refreshTokenPrefix +
		base64.RawURLEncoding.EncodeToString(
			randomBytes,
		)

	if token.Value() != expectedValue {
		t.Fatal(
			"generated refresh token did not match the deterministic test value",
		)
	}

	parsedToken, err := ParseRefreshToken(
		token.Value(),
	)
	if err != nil {
		t.Fatalf(
			"parse generated refresh token: %v",
			err,
		)
	}

	if parsedToken.Value() != token.Value() {
		t.Fatal(
			"parsed refresh token did not preserve its value",
		)
	}

	digest, err := token.Digest()
	if err != nil {
		t.Fatalf(
			"digest refresh token: %v",
			err,
		)
	}

	expectedDigest := sha256.Sum256(
		[]byte(token.Value()),
	)

	if digest !=
		RefreshTokenDigest(expectedDigest) {
		t.Fatal(
			"refresh token digest did not match SHA-256",
		)
	}
}

func TestRefreshTokenGeneratorCreatesUniqueTokens(
	t *testing.T,
) {
	generator := NewRefreshTokenGenerator()

	seen := make(
		map[string]struct{},
		100,
	)

	for range 100 {
		token, err := generator.Generate(
			context.Background(),
		)
		if err != nil {
			t.Fatalf(
				"generate refresh token: %v",
				err,
			)
		}

		value := token.Value()

		if _, exists := seen[value]; exists {
			t.Fatal(
				"refresh token generator produced a duplicate token",
			)
		}

		seen[value] = struct{}{}
	}
}

func TestRefreshTokenGeneratorRejectsCanceledContext(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	generator := NewRefreshTokenGenerator()

	_, err := generator.Generate(ctx)
	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}
}

func TestRefreshTokenGeneratorMapsRandomFailure(
	t *testing.T,
) {
	const internalMarker = "synthetic-random-source-failure-marker"

	generator := &RefreshTokenGenerator{
		random: errorReader{
			err: errors.New(
				internalMarker,
			),
		},
	}

	_, err := generator.Generate(
		context.Background(),
	)
	if !errors.Is(
		err,
		ErrRefreshTokenGenerationUnavailable,
	) {
		t.Fatalf(
			"expected ErrRefreshTokenGenerationUnavailable, got %v",
			err,
		)
	}

	if strings.Contains(
		err.Error(),
		internalMarker,
	) {
		t.Fatal(
			"refresh token error exposed random-source details",
		)
	}
}

func TestRefreshTokenGeneratorRejectsMissingRandomSource(
	t *testing.T,
) {
	var nilGenerator *RefreshTokenGenerator

	_, err := nilGenerator.Generate(
		context.Background(),
	)
	if !errors.Is(
		err,
		ErrRefreshTokenGenerationUnavailable,
	) {
		t.Fatalf(
			"expected ErrRefreshTokenGenerationUnavailable, got %v",
			err,
		)
	}

	generator := &RefreshTokenGenerator{}

	_, err = generator.Generate(
		context.Background(),
	)
	if !errors.Is(
		err,
		ErrRefreshTokenGenerationUnavailable,
	) {
		t.Fatalf(
			"expected ErrRefreshTokenGenerationUnavailable, got %v",
			err,
		)
	}
}

func TestParseRefreshTokenRejectsMalformedValues(
	t *testing.T,
) {
	validEncodedValue :=
		base64.RawURLEncoding.EncodeToString(
			bytes.Repeat(
				[]byte{0xcd},
				refreshTokenEntropyBytes,
			),
		)

	testCases := map[string]string{
		"empty":          "",
		"missing prefix": validEncodedValue,
		"wrong prefix":   "other_" + validEncodedValue,
		"too short": refreshTokenPrefix +
			validEncodedValue[:len(validEncodedValue)-1],
		"too long": refreshTokenPrefix +
			validEncodedValue +
			"a",
		"invalid character": refreshTokenPrefix +
			validEncodedValue[:len(validEncodedValue)-1] +
			"*",
		"padded encoding": refreshTokenPrefix +
			validEncodedValue +
			"=",
		"surrounding space": " " +
			refreshTokenPrefix +
			validEncodedValue,
	}

	for name, value := range testCases {
		t.Run(
			name,
			func(t *testing.T) {
				_, err := ParseRefreshToken(
					value,
				)
				if !errors.Is(
					err,
					ErrRefreshTokenMalformed,
				) {
					t.Fatalf(
						"expected ErrRefreshTokenMalformed, got %v",
						err,
					)
				}
			},
		)
	}
}

func TestRefreshTokenDigestRejectsZeroValue(
	t *testing.T,
) {
	var token RefreshToken

	_, err := token.Digest()
	if !errors.Is(
		err,
		ErrRefreshTokenMalformed,
	) {
		t.Fatalf(
			"expected ErrRefreshTokenMalformed, got %v",
			err,
		)
	}
}

func TestRefreshTokenFormattingIsRedacted(
	t *testing.T,
) {
	rawValue := refreshTokenPrefix +
		base64.RawURLEncoding.EncodeToString(
			bytes.Repeat(
				[]byte{0xef},
				refreshTokenEntropyBytes,
			),
		)

	token, err := ParseRefreshToken(
		rawValue,
	)
	if err != nil {
		t.Fatalf(
			"parse refresh token: %v",
			err,
		)
	}

	digest, err := token.Digest()
	if err != nil {
		t.Fatalf(
			"digest refresh token: %v",
			err,
		)
	}

	formattedValues := []string{
		fmt.Sprintf("%s", token),
		fmt.Sprintf("%v", token),
		fmt.Sprintf("%+v", token),
		fmt.Sprintf("%#v", token),
		fmt.Sprintf("%x", token),
		fmt.Sprintf("%v", digest),
		fmt.Sprintf("%#v", digest),
		fmt.Sprintf("%x", digest),
	}

	for _, formattedValue := range formattedValues {
		if strings.Contains(
			formattedValue,
			rawValue,
		) {
			t.Fatal(
				"formatted refresh-token value exposed the raw token",
			)
		}

		if formattedValue != "[REDACTED]" {
			t.Fatalf(
				"formatted value = %q, want [REDACTED]",
				formattedValue,
			)
		}
	}
}

func TestRefreshTokenDigestBytesReturnsCopy(
	t *testing.T,
) {
	rawValue := refreshTokenPrefix +
		base64.RawURLEncoding.EncodeToString(
			bytes.Repeat(
				[]byte{0x11},
				refreshTokenEntropyBytes,
			),
		)

	token, err := ParseRefreshToken(
		rawValue,
	)
	if err != nil {
		t.Fatalf(
			"parse refresh token: %v",
			err,
		)
	}

	digest, err := token.Digest()
	if err != nil {
		t.Fatalf(
			"digest refresh token: %v",
			err,
		)
	}

	firstCopy := digest.Bytes()
	firstCopy[0] ^= 0xff

	secondCopy := digest.Bytes()

	if bytes.Equal(
		firstCopy,
		secondCopy,
	) {
		t.Fatal(
			"mutating returned digest bytes changed the stored digest",
		)
	}

	if len(secondCopy) != sha256.Size {
		t.Fatalf(
			"digest length = %d, want %d",
			len(secondCopy),
			sha256.Size,
		)
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read(
	_ []byte,
) (int, error) {
	return 0, reader.err
}
